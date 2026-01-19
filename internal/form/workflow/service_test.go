package workflow_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"NYCU-SDC/core-system-backend/internal/form/workflow"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"go.uber.org/zap"
)

// mockQuerier is a mock implementation of workflow.Querier interface
type mockQuerier struct {
	mock.Mock
}

func (m *mockQuerier) Get(ctx context.Context, formID uuid.UUID) (workflow.GetRow, error) {
	args := m.Called(ctx, formID)
	return args.Get(0).(workflow.GetRow), args.Error(1)
}

func (m *mockQuerier) Update(ctx context.Context, arg workflow.UpdateParams) (workflow.UpdateRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(workflow.UpdateRow), args.Error(1)
}

func (m *mockQuerier) CreateNode(ctx context.Context, arg workflow.CreateNodeParams) (workflow.CreateNodeRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(workflow.CreateNodeRow), args.Error(1)
}

func (m *mockQuerier) DeleteNode(ctx context.Context, arg workflow.DeleteNodeParams) ([]byte, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *mockQuerier) Activate(ctx context.Context, arg workflow.ActivateParams) (workflow.ActivateRow, error) {
	args := m.Called(ctx, arg)
	return args.Get(0).(workflow.ActivateRow), args.Error(1)
}

// mockValidator is a mock implementation of workflow.Validator interface
type mockValidator struct {
	mock.Mock
}

func (m *mockValidator) Activate(workflow []byte) error {
	args := m.Called(workflow)
	return args.Error(0)
}

// createTestService creates a workflow.Service with mocked dependencies
func createTestService(t *testing.T, logger *zap.Logger, tracer trace.Tracer, mockQuerier *mockQuerier, mockValidator *mockValidator) *workflow.Service {
	t.Helper()
	return workflow.NewServiceForTesting(logger, tracer, mockQuerier, mockValidator)
}

func TestService_Activate(t *testing.T) {
	t.Parallel()

	type Params struct {
		workflowJSON []byte
	}

	type testCase struct {
		name   string
		params Params
	}

	testCases := []testCase{
		{
			name: "successful activation with simple workflow",
			params: Params{
				workflowJSON: createSimpleValidWorkflow(t),
			},
		},
		{
			name: "successful activation with complex workflow",
			params: Params{
				workflowJSON: createComplexValidWorkflow(t),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			logger := zap.NewNop()
			tracer := noop.NewTracerProvider().Tracer("test")
			formID := uuid.New()
			userID := uuid.New()

			mockQuerier := new(mockQuerier)
			mockValidator := new(mockValidator)
			service := createTestService(t, logger, tracer, mockQuerier, mockValidator)

			mockValidator.On("Activate", tc.params.workflowJSON).Return(nil).Once()

			mockQuerier.On("Activate", mock.Anything, workflow.ActivateParams{
				FormID:     formID,
				LastEditor: userID,
				Workflow:   tc.params.workflowJSON,
			}).Return(workflow.ActivateRow{
				ID:         uuid.New(),
				FormID:     formID,
				LastEditor: userID,
				IsActive:   true,
				Workflow:   tc.params.workflowJSON,
			}, nil).Once()

			result, err := service.Activate(ctx, formID, userID, tc.params.workflowJSON)

			require.NoError(t, err, "unexpected error: %v", err)
			require.NotNilf(t, result.ID, "result.ID is nil")
			require.Equal(t, formID, result.FormID, "formID mismatch")
			require.Equal(t, userID, result.LastEditor, "userID mismatch")
			require.True(t, result.IsActive, "result.IsActive is false")
			require.Equal(t, tc.params.workflowJSON, result.Workflow, "workflow mismatch")

			mockValidator.AssertExpectations(t)
			mockQuerier.AssertExpectations(t)
		})
	}
}

func TestService_Activate_validation(t *testing.T) {
	t.Parallel()

	type Params struct {
		workflowJSON []byte
	}

	type testCase struct {
		name   string
		params Params
	}

	testCases := []testCase{
		{
			name:   "invalid JSON format",
			params: Params{workflowJSON: []byte(`{invalid json}`)},
		},
		{
			name:   "empty workflow",
			params: Params{workflowJSON: []byte(`[]`)},
		},
		{
			name:   "missing start node",
			params: Params{workflowJSON: createWorkflowJSON(t, []map[string]interface{}{createEndNode(t)})},
		},
		{
			name:   "missing end node",
			params: Params{workflowJSON: createWorkflowJSON(t, []map[string]interface{}{createStartNode(t, uuid.New().String())})},
		},
		{
			name:   "multiple start nodes",
			params: Params{workflowJSON: createWorkflowWithMultipleStarts(t)},
		},
		{
			name:   "multiple end nodes",
			params: Params{workflowJSON: createWorkflowWithMultipleEnds(t)},
		},
		{
			name:   "duplicate node IDs",
			params: Params{workflowJSON: createWorkflowWithDuplicateIDs(t)},
		},
		{
			name:   "invalid node ID format",
			params: Params{workflowJSON: createWorkflowWithInvalidID(t)},
		},
		{
			name:   "missing required fields",
			params: Params{workflowJSON: createWorkflowMissingFields(t)},
		},
		{
			name:   "unreachable nodes",
			params: Params{workflowJSON: createWorkflowWithOrphan(t)},
		},
		{
			name:   "invalid node references",
			params: Params{workflowJSON: createWorkflowWithInvalidRef(t)},
		},
		{
			name:   "invalid node type",
			params: Params{workflowJSON: createWorkflowWithInvalidType(t)},
		},
		{
			name:   "start node missing next field",
			params: Params{workflowJSON: createStartNodeMissingNext(t)},
		},
		{
			name:   "condition node missing conditionRule",
			params: Params{workflowJSON: createConditionNodeMissingRule(t)},
		},
		{
			name:   "condition node missing nextTrue",
			params: Params{workflowJSON: createConditionNodeMissingNextTrue(t)},
		},
		{
			name:   "condition node missing nextFalse",
			params: Params{workflowJSON: createConditionNodeMissingNextFalse(t)},
		},
		{
			name:   "condition node invalid conditionRule source",
			params: Params{workflowJSON: createConditionNodeInvalidSource(t)},
		},
		{
			name:   "condition node invalid regex pattern",
			params: Params{workflowJSON: createConditionNodeInvalidRegex(t)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			logger := zap.NewNop()
			tracer := noop.NewTracerProvider().Tracer("test")
			formID := uuid.New()
			userID := uuid.New()

			mockQuerier := new(mockQuerier)
			mockValidator := new(mockValidator)
			service := createTestService(t, logger, tracer, mockQuerier, mockValidator)

			mockValidator.On("Activate", tc.params.workflowJSON).Return(fmt.Errorf("workflow validation failed")).Once()

			_, err := service.Activate(ctx, formID, userID, tc.params.workflowJSON)

			require.Error(t, err, "expected error but got nil")
			mockValidator.AssertExpectations(t)
			mockQuerier.AssertNotCalled(t, "Activate")
		})
	}
}

// Helper functions to create test workflows

func createSimpleValidWorkflow(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	endID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  endID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

func createComplexValidWorkflow(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	sectionID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	referenceNodeID := uuid.New()

	workflowJSON, err := json.Marshal([]map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  sectionID.String(),
		},
		{
			"id":    sectionID.String(),
			"type":  "section",
			"label": "Section",
			"next":  conditionID.String(),
		},
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  endID.String(),
			"nextFalse": endID.String(),
			"conditionRule": map[string]interface{}{
				"source":  "choice",
				"nodeId":  referenceNodeID.String(),
				"key":     "answer",
				"pattern": "yes",
			},
		},
		{
			"id":    referenceNodeID.String(),
			"type":  "section",
			"label": "Reference Section",
			"next":  conditionID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
	require.NoError(t, err)
	return workflowJSON
}

func createWorkflowJSON(t *testing.T, nodes []map[string]interface{}) []byte {
	t.Helper()
	json, err := json.Marshal(nodes)
	require.NoError(t, err)
	return json
}

func createStartNode(t *testing.T, nextID string) map[string]interface{} {
	t.Helper()
	return map[string]interface{}{
		"id":    uuid.New().String(),
		"type":  "start",
		"label": "Start",
		"next":  nextID,
	}
}

func createEndNode(t *testing.T) map[string]interface{} {
	t.Helper()
	return map[string]interface{}{
		"id":    uuid.New().String(),
		"type":  "end",
		"label": "End",
	}
}

func createWorkflowWithMultipleStarts(t *testing.T) []byte {
	t.Helper()
	endID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, endID.String()),
		createStartNode(t, endID.String()),
		createEndNode(t),
	})
}

func createWorkflowWithMultipleEnds(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, startID.String()),
		createEndNode(t),
		createEndNode(t),
	})
}

func createWorkflowWithDuplicateIDs(t *testing.T) []byte {
	t.Helper()
	duplicateID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    duplicateID.String(),
			"type":  "start",
			"label": "Start",
			"next":  duplicateID.String(),
		},
		{
			"id":    duplicateID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

func createWorkflowWithInvalidID(t *testing.T) []byte {
	t.Helper()
	endID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    "not-a-uuid",
			"type":  "start",
			"label": "Start",
			"next":  endID.String(),
		},
		createEndNode(t),
	})
}

func createWorkflowMissingFields(t *testing.T) []byte {
	t.Helper()
	endID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":   uuid.New().String(),
			"type": "start",
			// missing "label"
			"next": endID.String(),
		},
		createEndNode(t),
	})
}

func createWorkflowWithOrphan(t *testing.T) []byte {
	t.Helper()
	endID := uuid.New()
	orphanID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, endID.String()),
		createEndNode(t),
		{
			"id":    orphanID.String(),
			"type":  "section",
			"label": "Orphan",
		},
	})
}

func createWorkflowWithInvalidRef(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	nonExistentID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  nonExistentID.String(),
		},
		createEndNode(t),
	})
}

func createWorkflowWithInvalidType(t *testing.T) []byte {
	t.Helper()
	endID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    uuid.New().String(),
			"type":  "invalid_type",
			"label": "Invalid",
			"next":  endID.String(),
		},
		createEndNode(t),
	})
}

func createStartNodeMissingNext(t *testing.T) []byte {
	t.Helper()
	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    uuid.New().String(),
			"type":  "start",
			"label": "Start",
			// missing "next"
		},
		createEndNode(t),
	})
}

func createConditionNodeMissingRule(t *testing.T) []byte {
	t.Helper()
	conditionID := uuid.New()
	endID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, conditionID.String()),
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  endID.String(),
			"nextFalse": endID.String(),
			// missing "conditionRule"
		},
		createEndNode(t),
	})
}

func createConditionNodeMissingNextTrue(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	questionID := uuid.New()
	choiceID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, conditionID.String()),
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextFalse": endID.String(),
			"conditionRule": map[string]interface{}{
				"source":  "choice",
				"nodeId":  startID.String(),
				"key":     questionID.String(),
				"pattern": fmt.Sprintf("^%s$", choiceID.String()),
			},
		},
		createEndNode(t),
	})
}

func createConditionNodeMissingNextFalse(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	questionID := uuid.New()
	choiceID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, conditionID.String()),
		{
			"id":       conditionID.String(),
			"type":     "condition",
			"label":    "Condition",
			"nextTrue": endID.String(),
			"conditionRule": map[string]interface{}{
				"source":  "choice",
				"nodeId":  startID.String(),
				"key":     questionID.String(),
				"pattern": fmt.Sprintf("^%s$", choiceID.String()),
			},
		},
		createEndNode(t),
	})
}

func createConditionNodeInvalidSource(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	questionID := uuid.New()
	choiceID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, conditionID.String()),
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  endID.String(),
			"nextFalse": endID.String(),
			"conditionRule": map[string]interface{}{
				"source":  "invalid_source",
				"nodeId":  startID.String(),
				"key":     questionID.String(),
				"pattern": fmt.Sprintf("^%s$", choiceID.String()),
			},
		},
		createEndNode(t),
	})
}

func createConditionNodeInvalidRegex(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	questionID := uuid.New()
	choiceID := uuid.New()
	return createWorkflowJSON(t, []map[string]interface{}{
		createStartNode(t, conditionID.String()),
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  endID.String(),
			"nextFalse": endID.String(),
			"conditionRule": map[string]interface{}{
				"source":  "choice",
				"nodeId":  startID.String(),
				"key":     questionID.String(),
				"pattern": fmt.Sprintf("[%s", choiceID.String()), // intentionally invalid regex
			},
		},
		createEndNode(t),
	})
}
