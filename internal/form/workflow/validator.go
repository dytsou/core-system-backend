package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/form/workflow/node"

	"github.com/google/uuid"
)

// QuestionStore defines the interface for querying form questions
// This allows the validator to check if condition rule question IDs exist and match expected types
type QuestionStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (question.Answerable, error)
	ListByFormID(ctx context.Context, formID uuid.UUID) ([]question.Answerable, error)
}

type workflowValidator struct{}

func (v workflowValidator) Activate(ctx context.Context, formID uuid.UUID, workflow []byte, questionStore QuestionStore) error {
	return Activate(ctx, formID, workflow, questionStore)
}

// Validate performs a relaxed validation suitable for draft updates.
//
// It intentionally allows incomplete graphs (e.g. unreachable nodes, missing next fields,
// incomplete condition nodes) while still enforcing:
// - valid JSON array structure
// - required node fields (id/type/label) and UUID id format
// - supported node types
// - duplicate id detection
// - reference integrity for any explicitly provided next/nextTrue/nextFalse fields
// - exactly one start and one end node
func (v workflowValidator) Validate(ctx context.Context, formID uuid.UUID, workflow []byte, questionStore QuestionStore) error {
	return ValidateDraft(ctx, formID, workflow, questionStore)
}

// NewValidator creates a new workflow validator using the built-in NodeType set.
func NewValidator() Validator {
	return workflowValidator{}
}

// validateWorkflow validates the workflow JSON structure before activation.
// It checks:
// - Valid JSON format
// - Array structure
// - Node structure and required fields
// - Valid node types
// - Graph connectivity (all nodes are reachable)
// - Condition rule question IDs exist and types match
// Returns all validation errors if validation fails
func Activate(ctx context.Context, formID uuid.UUID, workflow []byte, questionStore QuestionStore) error {
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

	// Validate all nodes and build node map (strict mode)
	nodeMap, startNodeCount, endNodeCount, nodeErrors := validateNodes(ctx, formID, nodes, questionStore, true)
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

// ValidateDraft performs relaxed validation for draft workflows (used by Update).
func ValidateDraft(ctx context.Context, formID uuid.UUID, workflow []byte, questionStore QuestionStore) error {
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

	// Validate nodes and build node map (relaxed mode: skip node-specific validation)
	nodeMap, startNodeCount, endNodeCount, nodeErrors := validateNodes(ctx, formID, nodes, questionStore, false)
	if len(nodeErrors) > 0 {
		validationErrors = append(validationErrors, nodeErrors...)
	}

	// Require exactly one start and one end node even in drafts
	nodeTypeErrors := validateRequiredNodeTypes(startNodeCount, endNodeCount)
	if len(nodeTypeErrors) > 0 {
		validationErrors = append(validationErrors, nodeTypeErrors...)
	}

	// In draft mode we allow unreachable nodes, but we still validate that any explicit references
	// point to nodes that exist.
	if nodeMap != nil {
		err := validateGraphReferences(nodes, nodeMap)
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
	_, ok := node["id"]
	if !ok || node["id"] == "" {
		return fmt.Errorf("node at index %d missing required field 'id'", index)
	}

	// Check for 'type' field
	_, ok = node["type"]
	if !ok || node["type"] == "" {
		return fmt.Errorf("node at index %d missing required field 'type'", index)
	}

	// Check for 'label' field
	_, ok = node["label"]
	if !ok || node["label"] == "" {
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
// When isActivate is true, this performs full node-specific validation (used by Activate).
// When false, it performs a relaxed validation suitable for draft Update.
func validateNodes(ctx context.Context, formID uuid.UUID, nodes []map[string]interface{}, questionStore QuestionStore, isActivate bool) (map[string]map[string]interface{}, int, int, []error) {
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
		if isActivate {
			validatedNodes = append(validatedNodes, validatedNode)
		}

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
	if isActivate {
		for i, validatedNode := range validatedNodes {
			// Pass context, formID, and questionStore for condition rule validation
			err := validatedNode.Validate(ctx, formID, nodeMap, questionStore)
			if err != nil {
				validationErrors = append(validationErrors, fmt.Errorf("node at index %d: %w", i, err))
			}
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
	// First validate references
	err := validateGraphReferences(nodes, nodeMap)
	if err != nil {
		return err
	}

	// Build graph for reachability validation
	graph := make(map[string][]string)

	// Build adjacency list and validate references
	for _, node := range nodes {
		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)

		var nextNodes []string

		// Handle different node types and their connection fields
		if nodeType == string(NodeTypeCondition) {
			// Condition nodes have nextTrue and nextFalse
			nextTrue, ok := node["nextTrue"].(string)
			if ok && nextTrue != "" {
				nextNodes = append(nextNodes, nextTrue)
			}
			nextFalse, ok := node["nextFalse"].(string)
			if ok && nextFalse != "" {
				nextNodes = append(nextNodes, nextFalse)
			}
		} else {
			// Other nodes have next field
			next, ok := node["next"].(string)
			if ok && next != "" {
				nextNodes = append(nextNodes, next)
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

// validateGraphReferences validates that any explicit reference fields point to nodes that exist.
func validateGraphReferences(nodes []map[string]interface{}, nodeMap map[string]map[string]interface{}) error {
	var referenceErrors []error

	for _, node := range nodes {
		nodeID, _ := node["id"].(string)
		nodeType, _ := node["type"].(string)

		if nodeType == string(NodeTypeCondition) {
			nextTrue, ok := node["nextTrue"].(string)
			if ok && nextTrue != "" {
				if _, exists := nodeMap[nextTrue]; !exists {
					referenceErrors = append(referenceErrors, fmt.Errorf("condition node '%s' references non-existent node '%s' in nextTrue", nodeID, nextTrue))
				}
			}

			nextFalse, ok := node["nextFalse"].(string)
			if ok && nextFalse != "" {
				if _, exists := nodeMap[nextFalse]; !exists {
					referenceErrors = append(referenceErrors, fmt.Errorf("condition node '%s' references non-existent node '%s' in nextFalse", nodeID, nextFalse))
				}
			}
		} else {
			next, ok := node["next"].(string)
			if ok && next != "" {
				if _, exists := nodeMap[next]; !exists {
					referenceErrors = append(referenceErrors, fmt.Errorf("node '%s' references non-existent node '%s' in next", nodeID, next))
				}
			}
		}
	}

	if len(referenceErrors) > 0 {
		return fmt.Errorf("invalid node references found: %w", errors.Join(referenceErrors...))
	}
	return nil
}
