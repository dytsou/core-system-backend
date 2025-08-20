package question

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type ChoiceOption struct {
	Name string `json:"name" validate:"required"`
}

type Choice struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

type SingleChoice struct {
	question Question
	Choices  []Choice
}

func (s SingleChoice) Question() Question {
	return s.question
}

func (s SingleChoice) Validate(value string) error {
	if value == "" {
		return nil // No value means no selection
	}

	for _, choice := range s.Choices {
		if choice.ID.String() == value {
			return nil
		}
	}

	return ErrInvalidChoiceID{
		QuestionID: s.question.ID.String(),
		ChoiceID:   value,
	}
}

func NewSingleChoice(q Question) (SingleChoice, error) {
	metadata := q.Metadata
	if metadata == nil {
		return SingleChoice{}, errors.New("metadata is nil")
	}

	choices, err := ExtractChoices(metadata)
	if err != nil {
		return SingleChoice{}, ErrMetadataBroken{
			QuestionID: q.ID.String(),
			RawData:    metadata,
			Message:    "could not extract choices from metadata",
		}
	}

	if len(choices) == 0 {
		return SingleChoice{}, ErrMetadataBroken{
			QuestionID: q.ID.String(),
			RawData:    metadata,
			Message:    "no choices found in metadata",
		}
	}

	for _, choice := range choices {
		if choice.ID == uuid.Nil {
			return SingleChoice{}, ErrMetadataBroken{
				QuestionID: q.ID.String(),
				RawData:    metadata,
				Message:    "choice ID cannot be nil",
			}
		}

		if strings.TrimSpace(choice.Name) == "" {
			return SingleChoice{}, ErrMetadataBroken{
				QuestionID: q.ID.String(),
				RawData:    metadata,
				Message:    "choice name cannot be empty",
			}
		}
	}

	return SingleChoice{
		question: q,
		Choices:  choices,
	}, nil
}

type MultiChoice struct {
	question Question
	Choices  []Choice
}

func (m MultiChoice) Question() Question {
	return m.question
}

func (m MultiChoice) Validate(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil // No value means no selection
	}

	ids := strings.Split(value, ";")
	for _, v := range ids {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		valid := false
		for _, choice := range m.Choices {
			if choice.ID.String() == v {
				valid = true
				break
			}
		}

		if !valid {
			return ErrInvalidChoiceID{
				QuestionID: m.question.ID.String(),
				ChoiceID:   v,
			}
		}
	}

	return nil
}

func NewMultiChoice(q Question) (MultiChoice, error) {
	metadata := q.Metadata
	if metadata == nil {
		return MultiChoice{}, errors.New("metadata is nil")
	}

	choices, err := ExtractChoices(metadata)
	if err != nil {
		return MultiChoice{}, ErrMetadataBroken{
			QuestionID: q.ID.String(),
			RawData:    metadata,
			Message:    "could not extract choices from metadata",
		}
	}

	if len(choices) == 0 {
		return MultiChoice{}, ErrMetadataBroken{
			QuestionID: q.ID.String(),
			RawData:    metadata,
			Message:    "no choices found in metadata",
		}
	}

	for _, choice := range choices {
		if choice.ID == uuid.Nil {
			return MultiChoice{}, ErrMetadataBroken{
				QuestionID: q.ID.String(),
				RawData:    metadata,
				Message:    "choice ID cannot be nil",
			}
		}

		if strings.TrimSpace(choice.Name) == "" {
			return MultiChoice{}, ErrMetadataBroken{
				QuestionID: q.ID.String(),
				RawData:    metadata,
				Message:    "choice name cannot be empty",
			}
		}
	}

	return MultiChoice{
		question: q,
		Choices:  choices,
	}, nil
}

// Creates and validates metadata JSON for choice-based questions
func GenerateMetadata(questionType string, choiceOptions []ChoiceOption) ([]byte, error) {
	// For non-choice questions, return empty metadata
	if questionType != "single_choice" && questionType != "multiple_choice" {
		if len(choiceOptions) > 0 {
			return nil, ErrUnsupportedQuestionType{QuestionType: questionType}
		}
		return json.Marshal(map[string]any{})
	}

	// For choice questions, require at least one choice
	if len(choiceOptions) == 0 {
		return nil, ErrMetadataValidate{
			QuestionID: questionType,
			RawData:    []byte(fmt.Sprintf("%v", choiceOptions)),
			Message:    "no choices provided for choice question",
		}
	}

	// Generate choices with UUIDs
	choices := make([]Choice, len(choiceOptions))
	for i, option := range choiceOptions {
		name := strings.TrimSpace(option.Name)
		if name == "" {
			return nil, ErrMetadataValidate{
				QuestionID: questionType,
				RawData:    []byte(fmt.Sprintf("%v", choiceOptions)),
				Message:    "choice name cannot be empty",
			}
		}
		choices[i] = Choice{
			ID:   uuid.New(),
			Name: name,
		}
	}

	metadata := map[string]any{
		"choice": choices,
	}

	return json.Marshal(metadata)
}

func ExtractChoices(data []byte) ([]Choice, error) {
	var partial map[string]json.RawMessage
	if err := json.Unmarshal(data, &partial); err != nil {
		return nil, fmt.Errorf("could not parse partial json: %w", err)
	}

	var choices []Choice
	if raw, ok := partial["choice"]; ok {
		if err := json.Unmarshal(raw, &choices); err != nil {
			return nil, fmt.Errorf("could not parse choices: %w", err)
		}
	}

	return choices, nil
}
