package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"sync"
	"time"
)

// baseURL is a placeholder for the target server's base URL.
// IMPORTANT: Adjust this to your actual server address (e.g., "http://localhost:8080" or "https://api.example.com")
var baseURL = "http://localhost:8080"

// authenticatedClient will store the HTTP client with an active session cookie.
var authenticatedClient *http.Client

const (
	// loginEndpoint is the authentication endpoint specified in the instructions.
	loginEndpoint = "/api/auth/login/internal"

	// loginUID is a sample UUID for internal login. The specific UUID doesn't cause the race condition;
	// it's the *internal* generation of a nil UUID later that's the bug.
	loginUID = "2c5e15ab-de56-48ad-9387-4291a3505f17"

	// targetEndpointPattern is derived from `expert_analysis.verification.steps`.
	// The `{slug}` placeholder will be filled.
	targetEndpointPattern = "/api/orgs/%s"

	// targetSlug is identified from `triage.detected_keywords` ("k6-slug-test").
	// This specific slug is used to trigger the problematic path under load.
	targetSlug = "k6-slug-test"

	// numWorkers is set to at least 50 to simulate "heavy concurrent load" as per "Race Condition" category and "high concurrency" keyword.
	numWorkers = 50

	// errorKeyword1, errorKeyword2, errorKeyword3 are derived from `triage.detected_keywords` and `primary_error_log`.
	// These strings are expected in the response body when the error is reproduced.
	errorKeyword1 = "Failed to get tenant by id"
	errorKeyword2 = "NotFoundError"
	// errorKeyword3 directly represents the "nil UUID" mentioned in `root_cause.evidence`.
	errorKeyword3 = "00000000-0000-0000-0000-000000000000"
)

var (
	errorsFound     int        // Counter for successful error reproductions
	errorsFoundLock sync.Mutex // Mutex to protect access to errorsFound in concurrent goroutines
)

// setup performs the login request and initializes the authenticated http.Client
// with a cookie jar to maintain session state.
func setup() error {
	// Create a new cookie jar to manage session cookies automatically.
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Initialize the global authenticatedClient with the cookie jar and a timeout.
	authenticatedClient = &http.Client{
		Jar:     jar,
		Timeout: 10 * time.Second,
	}

	loginURL := fmt.Sprintf("%s%s", baseURL, loginEndpoint)

	// Prepare the JSON body for the login request as specified in instructions.
	loginBody := map[string]string{"uid": loginUID}
	jsonBody, err := json.Marshal(loginBody)
	if err != nil {
		return fmt.Errorf("failed to marshal login body: %w", err)
	}

	// Create a POST request to the login endpoint.
	req, err := http.NewRequest(http.MethodPost, loginURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json") // Set Content-Type for JSON body.

	// Execute the login request.
	resp, err := authenticatedClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform login request: %w", err)
	}
	defer resp.Body.Close() // Ensure the response body is closed.

	// Check if login was successful (HTTP 200 OK).
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	log.Println("Login successful. Client authenticated.")
	return nil
}

// worker is a goroutine function that repeatedly hits the target endpoint.
// It checks the response for error keywords to identify successful reproduction.
func worker(id int, wg *sync.WaitGroup) {
	defer wg.Done() // Signal that this goroutine is done when it exits.

	targetURL := fmt.Sprintf(baseURL+targetEndpointPattern, targetSlug)

	// Create a GET request. The "k6/load script" in verification suggests read operations.
	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
	if err != nil {
		log.Printf("Worker %d: Failed to create request: %v", id, err)
		return
	}

	// Execute the request using the shared authenticated client.
	resp, err := authenticatedClient.Do(req)
	if err != nil {
		log.Printf("Worker %d: Failed to make request to %s: %v", id, targetURL, err)
		return
	}
	defer resp.Body.Close() // Ensure the response body is closed.

	// Read the response body to check for error keywords.
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Worker %d: Failed to read response body: %v", id, err)
		return
	}
	responseBody := string(bodyBytes)

	// Validate response for error keywords or HTTP 500 status.
	// This logic matches conditions described in `triage.detected_keywords` and `primary_error_log`.
	if resp.StatusCode == http.StatusInternalServerError ||
		strings.Contains(responseBody, errorKeyword1) ||
		strings.Contains(responseBody, errorKeyword2) ||
		strings.Contains(responseBody, errorKeyword3) {

		// If an error is detected, increment the shared counter safely using a mutex.
		errorsFoundLock.Lock()
		errorsFound++
		errorsFoundLock.Unlock()

		log.Printf("Worker %d: !!! Error Reproduced Successfully !!! Status: %d, Body contains expected error. Sample Body: %s", id, resp.StatusCode, responseBody)
		return
	}

	// Optionally log successful (non-error) responses if needed for debugging.
	// log.Printf("Worker %d: Request to %s successful. Status: %d. No error keywords found.", id, targetURL, resp.StatusCode)
}

func main() {
	log.Printf("Starting SDET reproduction script for Race Condition on %s", baseURL)

	// Step 1: Perform authentication using the setup function.
	if err := setup(); err != nil {
		log.Fatalf("Authentication failed: %v", err) // Terminate if authentication fails.
	}

	log.Printf("Initiating %d concurrent requests to %s...", numWorkers, fmt.Sprintf(baseURL+targetEndpointPattern, targetSlug))
	log.Println("Waiting for potential race conditions to manifest...")

	var wg sync.WaitGroup // WaitGroup to wait for all goroutines to complete.
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)           // Increment the counter for each goroutine.
		go worker(i+1, &wg) // Launch a new worker goroutine.
	}

	wg.Wait() // Block until all goroutines have called wg.Done().

	log.Println("All concurrent workers finished.")

	// Report the final results.
	if errorsFound > 0 {
		log.Printf("--- Test Result: ERROR REPRODUCED ---")
		log.Printf("Detected %d instances of the expected error pattern (e.g., '%s' or '%s' or '%s').", errorsFound, errorKeyword1, errorKeyword2, errorKeyword3)
		log.Println("The race condition, potentially leading to 'Failed to get tenant by id' with a nil UUID ('00000000-0000-0000-0000-000000000000'), was successfully observed.")
	} else {
		log.Printf("--- Test Result: NO ERROR REPRODUCED ---")
		log.Printf("No expected error patterns were detected during %d concurrent requests.", numWorkers)
		log.Println("This might indicate the bug is fixed, or requires more workers/different server conditions (e.g., higher load, specific timing).")
	}
}
