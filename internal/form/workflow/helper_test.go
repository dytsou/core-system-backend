package workflow_test

import (
	"context"
	"encoding/json"
	"testing"

	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/form/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace"
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

func (m *mockValidator) Activate(ctx context.Context, formID uuid.UUID, workflow []byte, questionStore workflow.QuestionStore) error {
	args := m.Called(ctx, formID, workflow, questionStore)
	return args.Error(0)
}

func (m *mockValidator) Validate(ctx context.Context, formID uuid.UUID, workflow []byte, questionStore workflow.QuestionStore) error {
	args := m.Called(ctx, formID, workflow, questionStore)
	return args.Error(0)
}

func (m *mockValidator) ValidateNodeIDsUnchanged(ctx context.Context, currentWorkflow, newWorkflow []byte) error {
	args := m.Called(ctx, currentWorkflow, newWorkflow)
	return args.Error(0)
}

func (m *mockValidator) ValidateUpdateNodeIDs(ctx context.Context, currentWorkflow, newWorkflow []byte) error {
	args := m.Called(ctx, currentWorkflow, newWorkflow)
	return args.Error(0)
}

// mockQuestionStore is a mock implementation of workflow.QuestionStore for testing
type mockQuestionStore struct {
	questions map[uuid.UUID]question.Answerable
}

func (m *mockQuestionStore) GetByID(ctx context.Context, id uuid.UUID) (question.Answerable, error) {
	if q, ok := m.questions[id]; ok {
		return q, nil
	}
	return nil, internal.ErrQuestionNotFound
}

func (m *mockQuestionStore) ListByFormID(ctx context.Context, formID uuid.UUID) ([]question.Answerable, error) {
	var result []question.Answerable
	for _, q := range m.questions {
		if q.FormID() == formID {
			result = append(result, q)
		}
	}
	return result, nil
}

// createTestService creates a workflow.Service with mocked dependencies
func createTestService(t *testing.T, logger *zap.Logger, tracer trace.Tracer, mockQuerier *mockQuerier, mockValidator *mockValidator, questionStore workflow.QuestionStore) *workflow.Service {
	t.Helper()
	return workflow.NewServiceForTesting(logger, tracer, mockQuerier, mockValidator, questionStore)
}

// createWorkflowJSON marshals nodes to JSON and fails the test on error
func createWorkflowJSON(t *testing.T, nodes []map[string]interface{}) []byte {
	t.Helper()
	jsonBytes, err := json.Marshal(nodes)
	require.NoError(t, err)
	return jsonBytes
}

// createSimpleValidWorkflow returns a minimal valid workflow (start -> end)
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

// createComplexValidWorkflow returns a workflow with start, section, condition, and end
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

// createWorkflowWithInvalidNextRef returns a workflow where start references a non-existent node in next
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
			"next":  nonExistentID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

// createWorkflowWithInvalidNextTrueRef returns a workflow where condition references non-existent node in nextTrue
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
			"nextTrue":  nonExistentID.String(),
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

// createWorkflowWithInvalidNextFalseRef returns a workflow where condition references non-existent node in nextFalse
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
			"nextFalse": nonExistentID.String(),
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

// createWorkflowWithInvalidConditionRefs returns a workflow where condition references non-existent nodes in both nextTrue and nextFalse
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
			"nextTrue":  nonExistentID1.String(),
			"nextFalse": nonExistentID2.String(),
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

// createWorkflowWithConditionRule returns a workflow with a condition rule using the given questionID as key
func createWorkflowWithConditionRule(t *testing.T, questionID string) []byte {
	t.Helper()
	if questionID == "" {
		questionID = uuid.New().String()
	}
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	sectionID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
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
				"nodeId":  sectionID.String(),
				"key":     questionID,
				"pattern": "yes",
			},
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

// createWorkflowWithConditionRuleSourceWithQuestionID returns a workflow with condition rule with given source and questionID
func createWorkflowWithConditionRuleSourceWithQuestionID(t *testing.T, source string, questionID string) []byte {
	t.Helper()
	startID := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()
	sectionID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
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
				"source":  source,
				"nodeId":  sectionID.String(),
				"key":     questionID,
				"pattern": "yes",
			},
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

// createMockAnswerable creates a question.Answerable for testing
func createMockAnswerable(t *testing.T, formID uuid.UUID, questionType question.QuestionType) question.Answerable {
	t.Helper()
	q := question.Question{
		ID:       uuid.New(),
		Required: false,
		Type:     questionType,
		Title:    pgtype.Text{String: "Test Question", Valid: true},
		Order:    1,
	}

	if questionType == question.QuestionTypeSingleChoice || questionType == question.QuestionTypeMultipleChoice {
		metadata, err := question.GenerateChoiceMetadata(string(questionType), []question.ChoiceOption{
			{Name: "Option 1"},
			{Name: "Option 2"},
		})
		require.NoError(t, err)
		q.Metadata = metadata
	} else {
		q.Metadata = []byte("{}")
	}

	answerable, err := question.NewAnswerable(q, formID)
	require.NoError(t, err)
	return answerable
}

// mustParseUUID parses a UUID string and fails the test if parsing fails
func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err, "failed to parse UUID: %s", s)
	return id
}

// createSimpleWorkflowForNodeIDTest returns a minimal start->end workflow for node ID tests
func createSimpleWorkflowForNodeIDTest(t *testing.T) []byte {
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

// createWorkflowWithSection returns a workflow with start -> section -> end
func createWorkflowWithSection(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	sectionID := uuid.New()
	endID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
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
			"next":  endID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}

// createWorkflowWithMultipleNodes returns a workflow with start, two sections, condition, and end
func createWorkflowWithMultipleNodes(t *testing.T) []byte {
	t.Helper()
	startID := uuid.New()
	sectionID1 := uuid.New()
	sectionID2 := uuid.New()
	conditionID := uuid.New()
	endID := uuid.New()

	return createWorkflowJSON(t, []map[string]interface{}{
		{
			"id":    startID.String(),
			"type":  "start",
			"label": "Start",
			"next":  sectionID1.String(),
		},
		{
			"id":    sectionID1.String(),
			"type":  "section",
			"label": "Section 1",
			"next":  conditionID.String(),
		},
		{
			"id":        conditionID.String(),
			"type":      "condition",
			"label":     "Condition",
			"nextTrue":  sectionID2.String(),
			"nextFalse": endID.String(),
		},
		{
			"id":    sectionID2.String(),
			"type":  "section",
			"label": "Section 2",
			"next":  endID.String(),
		},
		{
			"id":    endID.String(),
			"type":  "end",
			"label": "End",
		},
	})
}
