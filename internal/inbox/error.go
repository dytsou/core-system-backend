package inbox

import "fmt"

type ErrInvalidBoolParameter struct {
	Parameter string
	Value     string
	Message   string
}

func (e ErrInvalidBoolParameter) Error() string {
	return fmt.Sprintf("invalid %s parameter: value: %s, message: %s", e.Parameter, e.Value, e.Message)
}

type ErrSearchTooLong struct {
	Parameter string
	Value     string
	Message   string
}

func (e ErrSearchTooLong) Error() string {
	return fmt.Sprintf("search string exceeds maximum length: value: %s, message: %s", e.Value, e.Message)
}
