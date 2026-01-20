package workflow_test

import (
	"context"
	"testing"

	"NYCU-SDC/core-system-backend/internal"
	"NYCU-SDC/core-system-backend/internal/form/question"
	"NYCU-SDC/core-system-backend/internal/form/workflow"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// TestActivate_ConditionRuleValidation tests strict condition rule validation
func TestActivate_ConditionRuleValidation(t *testing.T) {
	t.Parallel()

	formID := uuid.New()
	otherFormID := uuid.New()

	type testCase struct {
		name        string
		setup       func() ([]byte, workflow.QuestionStore)
		expectedErr bool
	}

	testCases := []testCase{
		{
			name: "condition rule with non-existent question ID",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				return createWorkflowWithConditionRule(t, questionID),
					&mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: true,
		},
		{
			name: "condition rule with question from different form",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRule(t, questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, otherFormID, question.QuestionTypeShortText),
						},
					}
			},
			expectedErr: true,
		},
		{
			name: "condition rule with source=choice but question type is short_text",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "choice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeShortText),
						},
					}
			},
			expectedErr: true,
		},
		{
			name: "condition rule with source=nonChoice but question type is single_choice",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "nonChoice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeSingleChoice),
						},
					}
			},
			expectedErr: true,
		},
		{
			name: "valid condition rule with source=choice and single_choice question",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "choice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeSingleChoice),
						},
					}
			},
			expectedErr: false,
		},
		{
			name: "valid condition rule with source=choice and multiple_choice question",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "choice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeMultipleChoice),
						},
					}
			},
			expectedErr: false,
		},
		{
			name: "valid condition rule with source=nonChoice and short_text question",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "nonChoice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeShortText),
						},
					}
			},
			expectedErr: false,
		},
		{
			name: "valid condition rule with source=nonChoice and long_text question",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "nonChoice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeLongText),
						},
					}
			},
			expectedErr: false,
		},
		{
			name: "valid condition rule with source=nonChoice and date question",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				questionUUID := mustParseUUID(t, questionID)
				return createWorkflowWithConditionRuleSourceWithQuestionID(t, "nonChoice", questionID),
					&mockQuestionStore{
						questions: map[uuid.UUID]question.Answerable{
							questionUUID: createMockAnswerable(t, formID, question.QuestionTypeDate),
						},
					}
			},
			expectedErr: false,
		},
	}

	validator := workflow.NewValidator()
	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workflowJSON, questionStore := tc.setup()
			err := validator.Activate(ctx, formID, workflowJSON, questionStore)

			if tc.expectedErr {
				require.Error(t, err, "expected validation error but got nil")
			} else {
				require.NoError(t, err, "expected validation to pass but got error: %v", err)
			}
		})
	}
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
		if q.Question().FormID == formID {
			result = append(result, q)
		}
	}
	return result, nil
}

// Helper functions for condition rule validation tests

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

func createMockAnswerable(t *testing.T, formID uuid.UUID, questionType question.QuestionType) question.Answerable {
	t.Helper()
	q := question.Question{
		ID:       uuid.New(),
		FormID:   formID,
		Required: false,
		Type:     questionType,
		Title:    pgtype.Text{String: "Test Question", Valid: true},
		Order:    1,
	}

	// Generate metadata for choice-based questions
	if questionType == question.QuestionTypeSingleChoice || questionType == question.QuestionTypeMultipleChoice {
		metadata, err := question.GenerateMetadata(string(questionType), []question.ChoiceOption{
			{Name: "Option 1"},
			{Name: "Option 2"},
		})
		require.NoError(t, err)
		q.Metadata = metadata
	} else {
		// For non-choice questions, use empty metadata
		q.Metadata = []byte("{}")
	}

	answerable, err := question.NewAnswerable(q)
	require.NoError(t, err)
	return answerable
}

func mustParseUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	require.NoError(t, err)
	return id
}

// TestValidate tests the Validate method which should reuse the same validation logic as Activate
func TestValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		setup       func() ([]byte, workflow.QuestionStore)
		expectedErr bool
	}

	formID := uuid.New()

	testCases := []testCase{
		{
			name: "valid workflow",
			setup: func() ([]byte, workflow.QuestionStore) {
				startID := uuid.New()
				endID := uuid.New()
				nodes := []map[string]interface{}{
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
				}
				return createWorkflowJSON(t, nodes), &mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: false,
		},
		{
			name: "invalid workflow - missing start node",
			setup: func() ([]byte, workflow.QuestionStore) {
				endID := uuid.New()
				nodes := []map[string]interface{}{
					{
						"id":    endID.String(),
						"type":  "end",
						"label": "End",
					},
				}
				return createWorkflowJSON(t, nodes), &mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: true,
		},
		{
			name: "invalid workflow - duplicate node IDs",
			setup: func() ([]byte, workflow.QuestionStore) {
				startID := uuid.New()
				endID := uuid.New()
				nodes := []map[string]interface{}{
					{
						"id":    startID.String(),
						"type":  "start",
						"label": "Start",
						"next":  endID.String(),
					},
					{
						"id":    startID.String(), // Duplicate ID
						"type":  "end",
						"label": "End",
					},
				}
				return createWorkflowJSON(t, nodes), &mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: true,
		},
		{
			name: "invalid workflow - unreachable node",
			setup: func() ([]byte, workflow.QuestionStore) {
				startID := uuid.New()
				endID := uuid.New()
				orphanID := uuid.New()
				nodes := []map[string]interface{}{
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
					{
						"id":    orphanID.String(),
						"type":  "section",
						"label": "Orphan",
						// No connections - unreachable
					},
				}
				return createWorkflowJSON(t, nodes), &mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: true,
		},
		{
			name: "invalid workflow - invalid node reference",
			setup: func() ([]byte, workflow.QuestionStore) {
				return createWorkflowWithInvalidNextRef(t), &mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: true,
		},
		{
			name: "invalid workflow - condition rule with non-existent question",
			setup: func() ([]byte, workflow.QuestionStore) {
				questionID := uuid.New().String()
				return createWorkflowWithConditionRule(t, questionID),
					&mockQuestionStore{questions: make(map[uuid.UUID]question.Answerable)}
			},
			expectedErr: true,
		},
	}

	validator := workflow.NewValidator()
	ctx := context.Background()

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			workflowJSON, questionStore := tc.setup()
			err := validator.Validate(ctx, formID, workflowJSON, questionStore)

			if tc.expectedErr {
				require.Error(t, err, "expected validation error")
			} else {
				require.NoError(t, err, "expected validation to pass but got error: %v", err)
			}
		})
	}
}
