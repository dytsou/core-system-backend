package oauthprovider

import (
	"NYCU-SDC/core-system-backend/internal/user"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type GoogleConfig struct {
	config *oauth2.Config
}

type GoogleOauth struct {
	ClientID     string `yaml:"client_id"     envconfig:"GOOGLE_OAUTH_CLIENT_ID"`
	ClientSecret string `yaml:"client_secret" envconfig:"GOOGLE_OAUTH_CLIENT_SECRET"`
}

func NewGoogleConfig(clientID, clientSecret, redirectURL string) *GoogleConfig {
	return &GoogleConfig{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes: []string{
				"openid", // Required for OpenID Connect to get ID token
				"https://www.googleapis.com/auth/userinfo.email",
				"https://www.googleapis.com/auth/userinfo.profile",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func (g *GoogleConfig) Name() string {
	return "google"
}

func (g *GoogleConfig) Config() *oauth2.Config {
	return g.config
}

func (g *GoogleConfig) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return g.config.Exchange(ctx, code)
}

func (g *GoogleConfig) GetUsername(email string) string {
	return strings.Split(email, "@")[0]
}

// GetUserInfo fetches user information from Google's JWT token (ID token)
// Using the 'sub' field from JWT token as recommended by Google for consistent user identification
func (g *GoogleConfig) GetUserInfo(ctx context.Context, token *oauth2.Token) (user.User, error) {
	// Extract ID token from the OAuth2 token
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return user.User{}, fmt.Errorf("no id_token found in OAuth2 token response")
	}

	// Parse the JWT token to extract user information
	// Note: In production, you should validate the JWT signature, but since we're getting
	// this directly from Google over HTTPS using our client secret, it's safe to decode
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return user.User{}, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (middle part of JWT)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return user.User{}, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	var claims struct {
		Sub           string `json:"sub"` // Unique user identifier (recommended by Google)
		Name          string `json:"name"`
		Email         string `json:"email"`
		Picture       string `json:"picture"`
		EmailVerified bool   `json:"email_verified"`
	}

	if err := json.Unmarshal(payload, &claims); err != nil {
		return user.User{}, fmt.Errorf("failed to unmarshal JWT claims: %w", err)
	}

	// Validate that we have the essential fields
	if claims.Sub == "" {
		return user.User{}, fmt.Errorf("missing 'sub' field in JWT token")
	}

	return user.User{
		ID:          uuid.New(),
		Name:        claims.Name,
		Username:    g.GetUsername(claims.Email),
		AvatarUrl:   claims.Picture,
		Role:        "user",
		OauthUserID: claims.Sub, // Use 'sub' field for consistent user identification
	}, nil
}
