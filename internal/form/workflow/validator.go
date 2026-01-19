package workflow

import (
	"encoding/json"
	"errors"
	"fmt"

	"NYCU-SDC/core-system-backend/internal/form/workflow/node"

	"github.com/google/uuid"
)

type validatorFunc func([]byte) error

func (f validatorFunc) Activate(workflow []byte) error {
	return f(workflow)
}

// NewValidator creates a new workflow validator using the built-in NodeType set.
func NewValidator() Validator {
	return validatorFunc(Activate)
}

// validateWorkflow validates the workflow JSON structure before activation.
// It checks:
// - Valid JSON format
// - Array structure
// - Node structure and required fields
// - Valid node types
// - Graph connectivity (all nodes are reachable)
// Returns all validation errors if validation fails
func Activate(workflow []byte) error {
	var validationErrors []error

	// Validate workflow length
	if err := validateWorkflowLength(workflow); err != nil {
		validationErrors = append(validationErrors, err)
	}

	// Validate and parse JSON format
	nodes, err := validateWorkflowJSON(workflow)
	if err != nil {
		validationErrors = append(validationErrors, err)
	}

	// If JSON parsing failed, we can't continue with node validation
	if nodes == nil {
		if len(validationErrors) > 0 {
			return fmt.Errorf("workflow validation failed: %w", errors.Join(validationErrors...))
		}
		return fmt.Errorf("workflow validation failed: unable to parse workflow")
	}

	// Validate all nodes and build node map
	nodeMap, startNodeCount, endNodeCount, nodeErrors := validateNodes(nodes)
	if len(nodeErrors) > 0 {
		validationErrors = append(validationErrors, nodeErrors...)
	}

	// Validate required node types (exactly one start and one end node)
	nodeTypeErrors := validateRequiredNodeTypes(startNodeCount, endNodeCount)
	if len(nodeTypeErrors) > 0 {
		validationErrors = append(validationErrors, nodeTypeErrors...)
	}

	// Validate graph connectivity: ensure all nodes are reachable
	// Only validate if we have a valid nodeMap (from successful node validation)
	if nodeMap != nil {
		err = validateGraphConnectivity(nodes, nodeMap)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("graph validation failed: %w", err))
		}
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("workflow validation failed: %w", errors.Join(validationErrors...))
	}

	return nil
}

// validateWorkflowLength validates that the workflow has a minimum length
func validateWorkflowLength(workflow []byte) error {
	if len(workflow) < 2 {
		return fmt.Errorf("workflow must contain at least 2 nodes, one start node and one end node")
	}
	return nil
}

// validateWorkflowJSON validates and parses the workflow JSON format
func validateWorkflowJSON(workflow []byte) ([]map[string]interface{}, error) {
	var nodes []map[string]interface{}
	err := json.Unmarshal(workflow, &nodes)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON format: %w", err)
	}
	return nodes, nil
}

// validateRequiredFields validates that a node has all required fields: id, type, label
func validateRequiredFields(node map[string]interface{}, index int) error {
	// Check for 'id' field
	if _, ok := node["id"]; !ok {
		return fmt.Errorf("node at index %d missing required field 'id'", index)
	}

	// Check for 'type' field
	if _, ok := node["type"]; !ok {
		return fmt.Errorf("node at index %d missing required field 'type'", index)
	}

	// Check for 'label' field
	if _, ok := node["label"]; !ok {
		return fmt.Errorf("node at index %d missing required field 'label'", index)
	}

	return nil
}

// validateNodeID validates that a node has a valid ID field
func validateNodeID(node map[string]interface{}, index int) (string, error) {
	id, ok := node["id"].(string)
	if !ok || id == "" {
		return "", fmt.Errorf("node at index %d missing required field 'id'", index)
	}

	// Validate UUID format
	_, err := uuid.Parse(id)
	if err != nil || id == "" {
		return "", fmt.Errorf("node at index %d has invalid UUID format for id '%s': %w", index, id, err)
	}

	return id, nil
}


// validateNodes validates all nodes in the workflow and returns:
// - nodeMap: map of node ID to node
// - startNodeCount: number of start nodes found
// - endNodeCount: number of end nodes found
// - errors: all validation errors collected
func validateNodes(nodes []map[string]interface{}) (map[string]map[string]interface{}, int, int, []error) {
	nodeMap := make(map[string]map[string]interface{})
	nodeIDs := make(map[string]bool)
	validatedNodes := make([]node.Validatable, 0, len(nodes))
	startNodeCount := 0
	endNodeCount := 0
	var validationErrors []error

	for i, nodeData := range nodes {
		// Validate required fields (id, type, label)
		if err := validateRequiredFields(nodeData, i); err != nil {
			validationErrors = append(validationErrors, err)
			continue // Skip this node but continue validating others
		}

		// Validate node ID
		id, err := validateNodeID(nodeData, i)
		if err != nil {
			validationErrors = append(validationErrors, err)
			continue // Skip this node but continue validating others
		}

		// Check for duplicate node IDs
		if nodeIDs[id] {
			validationErrors = append(validationErrors, fmt.Errorf("duplicate node id '%s' at index %d", id, i))
			continue // Skip this node but continue validating others
		}
		nodeIDs[id] = true
		nodeMap[id] = nodeData

		// Validate node type and build Validatable instance
		validatedNode, nodeTypeStr, err := node.New(nodeData)
		if err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("node at index %d: %w", i, err))
			continue // Skip this node but continue validating others
		}
		validatedNodes = append(validatedNodes, validatedNode)

		// Count start and end nodes
		if nodeTypeStr == string(NodeTypeStart) {
			startNodeCount++
		}
		if nodeTypeStr == string(NodeTypeEnd) {
			endNodeCount++
		}
	}

	// Validate each node using Validatable interface
	// This validates node-specific rules (e.g., condition nodes must have conditionRule)
	// Only validate nodes that were successfully created
	for i, validatedNode := range validatedNodes {
		if err := validatedNode.Validate(nodeMap); err != nil {
			validationErrors = append(validationErrors, fmt.Errorf("node at index %d: %w", i, err))
		}
	}

	return nodeMap, startNodeCount, endNodeCount, validationErrors
}

// validateRequiredNodeTypes validates that the workflow contains exactly one start node and exactly one end node
// Returns all validation errors found
func validateRequiredNodeTypes(startNodeCount, endNodeCount int) []error {
	var validationErrors []error
	if startNodeCount == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("workflow must contain exactly one start node, found %d", startNodeCount))
	} else if startNodeCount > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("workflow must contain exactly one start node, found %d", startNodeCount))
	}
	if endNodeCount == 0 {
		validationErrors = append(validationErrors, fmt.Errorf("workflow must contain exactly one end node, found %d", endNodeCount))
	} else if endNodeCount > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("workflow must contain exactly one end node, found %d", endNodeCount))
	}
	return validationErrors
}

// validateGraphConnectivity checks if all nodes in the workflow are reachable
// It ensures:
// - All node references (next, nextTrue, nextFalse) point to valid nodes
// - All nodes can be reached from entry points (start nodes or first node)
// - No orphaned nodes exist
func validateGraphConnectivity(nodes []map[string]interface{}, nodeMap map[string]map[string]interface{}) error {
	// Build graph and validate references
	graph := make(map[string][]string)
	incomingEdges := make(map[string]int)

	// Initialize incoming edge counts
	for id := range nodeMap {
		incomingEdges[id] = 0
	}

	// Build adjacency list
	for _, node := range nodes {
		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)

		var nextNodes []string

		// Handle different node types and their connection fields
		if nodeType == string(NodeTypeCondition) {
			// Condition nodes have nextTrue and nextFalse
			if nextTrue, ok := node["nextTrue"].(string); ok && nextTrue != "" {
				nextNodes = append(nextNodes, nextTrue)
				incomingEdges[nextTrue]++
			}
			if nextFalse, ok := node["nextFalse"].(string); ok && nextFalse != "" {
				nextNodes = append(nextNodes, nextFalse)
				incomingEdges[nextFalse]++
			}
		} else {
			// Other nodes have next field
			if next, ok := node["next"].(string); ok && next != "" {
				nextNodes = append(nextNodes, next)
				incomingEdges[next]++
			}
		}

		graph[nodeID] = nextNodes
	}

	// Find the start node (there should be exactly one)
	startNodeID := ""
	for _, node := range nodes {
		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)

		// The start node is the only entry point
		if nodeType == string(NodeTypeStart) {
			startNodeID = nodeID
			break
		}
	}

	// BFS traversal from the start node to ensure all nodes are reachable
	visited := make(map[string]bool)
	queue := make([]string, 0)

	// Start from the start node
	queue = append(queue, startNodeID)
	visited[startNodeID] = true

	// Traverse the graph
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		// Visit all connected nodes
		for _, nextNodeID := range graph[current] {
			if !visited[nextNodeID] {
				visited[nextNodeID] = true
				queue = append(queue, nextNodeID)
			}
		}
	}

	// Check if all nodes are reachable
	unreachableNodes := []string{}
	for nodeID := range nodeMap {
		if !visited[nodeID] {
			unreachableNodes = append(unreachableNodes, nodeID)
		}
	}

	if len(unreachableNodes) > 0 {
		return fmt.Errorf("unreachable nodes found: %v. All nodes must be reachable from the start node", unreachableNodes)
	}

	return nil
}
