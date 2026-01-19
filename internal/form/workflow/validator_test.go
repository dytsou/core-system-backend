package workflow_test

import (
	"context"
	"testing"

	"NYCU-SDC/core-system-backend/internal/form/workflow"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestActivate_InvalidNodeReferences tests that validateGraphConnectivity catches invalid node references
func TestActivate_InvalidNodeReferences(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		workflowJSON []byte
	}

	testCases := []testCase{
		{
			name:         "node references non-existent node ID in next field",
			workflowJSON: createWorkflowWithInvalidNextRef(t),
		},
		{
			name:         "condition node references non-existent node ID in nextTrue field",
			workflowJSON: createWorkflowWithInvalidNextTrueRef(t),
		},
		{
			name:         "condition node references non-existent node ID in nextFalse field",
			workflowJSON: createWorkflowWithInvalidNextFalseRef(t),
		},
		{
			name:         "condition node references non-existent nodes in both nextTrue and nextFalse",
			workflowJSON: createWorkflowWithInvalidConditionRefs(t),
		},
	}

	validator := workflow.NewValidator()
	ctx := context.Background()
	formID := uuid.New()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validator.Activate(ctx, formID, tc.workflowJSON, nil)

			require.Error(t, err, "expected validation error")
		})
	}
}

// Helper functions to create test workflows with invalid references

func createWorkflowWithInvalidNextRef(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	endID := uuid.New()
	nonExistentID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  nonExistentID.String(), // References non-existent node
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

func createWorkflowWithInvalidNextTrueRef(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	nonExistentID := uuid.New()
	sectionID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  conditionID.String(),
		},
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  nonExistentID.String(), // References non-existent node
			"nextFalse": endID.String(),
			"conditionRule": map[string]interface{}{
				"source":  "choice",
				"nodeId":  sectionID.String(),
				"key":     uuid.New().String(),
				"pattern": "yes",
			},
		},
		{
			"id":    sectionID.String(),
			"type":  "section",
			"label": "Section",
			"next":  conditionID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

func createWorkflowWithInvalidNextFalseRef(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	nonExistentID := uuid.New()
	sectionID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  conditionID.String(),
		},
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  endID.String(),
			"nextFalse": nonExistentID.String(), // References non-existent node
			"conditionRule": map[string]interface{}{
				"source":  "choice",
				"nodeId":  sectionID.String(),
				"key":     uuid.New().String(),
				"pattern": "yes",
			},
		},
		{
			"id":    sectionID.String(),
			"type":  "section",
			"label": "Section",
			"next":  conditionID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

func createWorkflowWithInvalidConditionRefs(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	nonExistentID1 := uuid.New()
	nonExistentID2 := uuid.New()
	sectionID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  conditionID.String(),
		},
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  nonExistentID1.String(), // References non-existent node
			"nextFalse": nonExistentID2.String(), // References non-existent node
			"conditionRule": map[string]interface{}{
				"source":  "non-choice",
				"nodeId":  sectionID.String(),
				"key":     uuid.New().String(),
				"pattern": "^no$",
			},
		},
		{
			"id":    sectionID.String(),
			"type":  "section",
			"label": "Section",
			"next":  conditionID.String(),
		},
	})
}
