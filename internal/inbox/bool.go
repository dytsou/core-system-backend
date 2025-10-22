package inbox

import (
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
		return false, ErrInvalidBoolParameter{
			Parameter: b.paramName,
			Value:     b.boolStr,
			Message:   "must be 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False",
		}
	}

	return parsedBool, nil
}
