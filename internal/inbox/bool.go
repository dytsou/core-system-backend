package inbox

import (
	"NYCU-SDC/core-system-backend/internal"
	"strconv"
)

type Bool struct {
	paramName string
	boolStr   string
}

// NewBool creates a new Bool from query parameters
func NewBool(paramName, boolStr string) (bool, error) {
	b := &Bool{
		paramName: paramName,
		boolStr:   boolStr,
	}
	parsedBool, err := b.Validate()
	if err != nil {
		return false, err
	}
	return parsedBool, nil
}

// Validate a boolean parameter from string value
func (b *Bool) Validate() (bool, error) {
	if b.boolStr == "" {
		return false, nil
	}
	// strconv.ParseBool will return an error if the string is not a valid boolean
	parsedBool, err := strconv.ParseBool(b.boolStr)
	if err != nil {
		switch b.paramName {
		case "isRead":
			return false, internal.ErrInvalidIsReadParameter
		case "isStarred":
			return false, internal.ErrInvalidIsStarredParameter
		case "isArchived":
			return false, internal.ErrInvalidIsArchivedParameter
		}
		return false, err
	}

	return parsedBool, nil
}
