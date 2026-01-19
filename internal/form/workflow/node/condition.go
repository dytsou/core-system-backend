package node

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// ConditionNode represents a condition node
type ConditionNode struct {
	node map[string]interface{}
}

func NewConditionNode(node map[string]interface{}) (Validatable, error) {
	return &ConditionNode{node: node}, nil
}

func (n *ConditionNode) Validate(nodeMap map[string]map[string]interface{}) error {
	nodeID, _ := n.node["id"].(string)

	// Validate field names (check for typos and invalid fields)
	err := n.validateFieldNames(nodeID)
	if err != nil {
		return err
	}

	// Condition node must have nextTrue and nextFalse
	nextTrue, ok := n.node["nextTrue"].(string)
	if !ok || nextTrue == "" {
		return fmt.Errorf("condition node '%s' must have a 'nextTrue' field", nodeID)
	}

	nextFalse, ok := n.node["nextFalse"].(string)
	if !ok || nextFalse == "" {
		return fmt.Errorf("condition node '%s' must have a 'nextFalse' field", nodeID)
	}

	// Validate that nextTrue and nextFalse nodes exist
	_, exists := nodeMap[nextTrue]
	if !exists {
		return fmt.Errorf("condition node '%s' references non-existent node '%s' in nextTrue", nodeID, nextTrue)
	}
	if _, exists := nodeMap[nextFalse]; !exists {
		return fmt.Errorf("condition node '%s' references non-existent node '%s' in nextFalse", nodeID, nextFalse)
	}

	// Validate conditionRule
	conditionRuleRaw, ok := n.node["conditionRule"]
	if !ok {
		return fmt.Errorf("condition node '%s' must have a 'conditionRule' field", nodeID)
	}

	// Parse conditionRule
	conditionRuleBytes, err := json.Marshal(conditionRuleRaw)
	if err != nil {
		return fmt.Errorf("condition node '%s' has invalid conditionRule format: %w", nodeID, err)
	}

	var conditionRule ConditionRule
	if err := json.Unmarshal(conditionRuleBytes, &conditionRule); err != nil {
		return fmt.Errorf("condition node '%s' has invalid conditionRule format: %w", nodeID, err)
	}

	// Validate conditionRule fields
	err = n.validateConditionRule(nodeID, conditionRule, nodeMap)
	if err != nil {
		return err
	}

	return nil
}

// validateFieldNames validates that the node only contains valid field names
func (n *ConditionNode) validateFieldNames(nodeID string) error {
	validFields := map[string]bool{
		"id":            true,
		"type":          true,
		"label":         true,
		"nextTrue":      true,
		"nextFalse":     true,
		"conditionRule": true,
	}

	var invalidFields []string
	for fieldName := range n.node {
		if !validFields[fieldName] {
			invalidFields = append(invalidFields, fieldName)
		}
	}

	if len(invalidFields) > 0 {
		return fmt.Errorf("condition node '%s' contains invalid field(s): %v. Valid fields are: conditionRule, id, label, nextFalse, nextTrue, type", nodeID, invalidFields)
	}

	return nil
}

func (n *ConditionNode) validateConditionRule(nodeID string, rule ConditionRule, nodeMap map[string]map[string]interface{}) error {
	// Validate source
	if rule.Source != ConditionSourceChoice && rule.Source != ConditionSourceNonChoice {
		return fmt.Errorf("condition node '%s' has invalid conditionRule.source: '%s'", nodeID, rule.Source)
	}

	// Validate nodeId exists
	_, exists := nodeMap[rule.NodeID]
	if !exists {
		return fmt.Errorf("condition node '%s' references non-existent node '%s' in conditionRule.nodeId", nodeID, rule.NodeID)
	}

	// Validate key
	if rule.Key == "" {
		return fmt.Errorf("condition node '%s' conditionRule.key cannot be empty", nodeID)
	}

	// Validate pattern (required for both choice and nonChoice sources)
	if rule.Pattern == "" {
		return fmt.Errorf("condition node '%s' conditionRule.pattern cannot be empty", nodeID)
	}

	// Validate pattern is a valid regex
	_, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("condition node '%s' conditionRule.pattern is not a valid regex: %w", nodeID, err)
	}

	return nil
}
