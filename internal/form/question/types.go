package question

func NewAnswerable(q Question) (Answerable, error) {
	switch q.Type {
	case QuestionTypeShortText:
		return NewShortText(q), nil
	case QuestionTypeLongText:
		return NewLongText(q), nil
	case QuestionTypeSingleChoice:
		return NewSingleChoice(q)
	case QuestionTypeMultipleChoice:
		return NewMultiChoice(q)
	case QuestionTypeDate:
		return NewDate(q)
	case QuestionTypeDropdown:
		return NewSingleChoice(q)
	case QuestionTypeDetailedMultipleChoice:
		return NewDetailedMultiChoice(q)
	case QuestionTypeLinearScale:
		return NewLinearScale(q)
	case QuestionTypeRating:
		return NewRating(q)
	case QuestionTypeRanking:
		return NewRanking(q)
	case QuestionTypeOauthConnect:
		return NewOAuthConnect(q)
	case QuestionTypeUploadFile:
		return NewUploadFile(q)
	}

	return nil, ErrUnsupportedQuestionType{
		QuestionType: string(q.Type),
	}
}
