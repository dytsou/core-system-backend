package inbox

import (
	"strings"
)

const (
	MaxSearchLength = 255
)

type Search struct {
	paramName string
	searchStr string
}

// NewSearch creates a new Search from query parameter
func NewSearch(paramName string, searchStr string) (*string, error) {
	s := &Search{
		paramName: paramName,
		searchStr: strings.TrimSpace(searchStr),
	}
	err := s.Validate()
	if err != nil {
		return nil, err
	}
	return &s.searchStr, nil
}

// Validate the search filter
func (s *Search) Validate() error {
	if len(s.searchStr) > MaxSearchLength {
		return ErrSearchTooLong{
			Parameter: s.paramName,
			Value:     s.searchStr,
			Message:   "search string exceeds maximum length",
		}
	}
	return nil
}
