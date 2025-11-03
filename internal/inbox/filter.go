package inbox

import (
	"net/http"
)

// FilterRequest represents the filter parameters for inbox messages
type FilterRequest struct {
	IsRead     *bool  `json:"isRead,omitempty"`
	IsStarred  *bool  `json:"isStarred,omitempty"`
	IsArchived *bool  `json:"isArchived,omitempty"`
	Search     string `json:"search,omitempty"`
}

// ParseFilterRequest parses filter parameters from HTTP request query parameters
func ParseFilterRequest(r *http.Request) (*FilterRequest, error) {
	query := r.URL.Query()

	isReadStr := query.Get("isRead")
	isStarredStr := query.Get("isStarred")
	isArchivedStr := query.Get("isArchived")
	searchStr := query.Get("search")

	isRead, err := NewBool("isRead", isReadStr)
	if err != nil {
		return nil, err
	}

	isStarred, err := NewBool("isStarred", isStarredStr)
	if err != nil {
		return nil, err
	}

	isArchived, err := NewBool("isArchived", isArchivedStr)
	if err != nil {
		return nil, err
	}

	search, err := NewSearch("search", searchStr)
	if err != nil {
		return nil, err
	}

	// Combine filters into FilterRequest
	filter := &FilterRequest{}

	// Only set filter values if they were explicitly provided in the query
	if isReadStr != "" {
		filter.IsRead = &isRead
	}
	if isStarredStr != "" {
		filter.IsStarred = &isStarred
	}
	if isArchivedStr != "" {
		filter.IsArchived = &isArchived
	}
	filter.Search = *search

	return filter, nil
}
