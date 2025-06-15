package internal

import (
	"errors"

	"github.com/NYCU-SDC/summer/pkg/problem"
)

var (
	// Auth Errors
	ErrInvalidRefreshToken  = errors.New("invalid refresh token")
	ErrProviderNotFound     = errors.New("provider not found")
	ErrInvalidExchangeToken = errors.New("invalid exchange token")
	ErrInvalidCallbackInfo  = errors.New("invalid callback info")
	ErrPermissionDenied     = errors.New("permission denied")
)

func NewProblemWriter() *problem.HttpWriter {
	return problem.NewWithMapping(ErrorHandler)
}

func ErrorHandler(err error) problem.Problem {
	switch {
	case errors.Is(err, ErrInvalidRefreshToken):
		return problem.NewNotFoundProblem("refresh token not found")
	case errors.Is(err, ErrProviderNotFound):
		return problem.NewNotFoundProblem("provider not found")
	case errors.Is(err, ErrInvalidExchangeToken):
		return problem.NewValidateProblem("invalid exchange token")
	case errors.Is(err, ErrInvalidCallbackInfo):
		return problem.NewValidateProblem("invalid callback info")
	case errors.Is(err, ErrPermissionDenied):
		return problem.NewForbiddenProblem("permission denied")
	}
	return problem.Problem{}
}

// RFC 7807 Problem Details for HTTP APIs
// https://tools.ietf.org/html/rfc7807

// ProblemDetail represents the base RFC 7807 problem detail structure
type ProblemDetail struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// Unauthorized represents a 401 Unauthorized error
type Unauthorized struct {
	ProblemDetail
	Realm string `json:"realm,omitempty"`
}

// NotFound represents a 404 Not Found error
type NotFound struct {
	ProblemDetail
	Resource string `json:"resource,omitempty"`
}

// NewProblemDetail creates a new generic problem detail
func NewProblemDetail(problemType, title string, status int) ProblemDetail {
	return ProblemDetail{
		Type:   problemType,
		Title:  title,
		Status: status,
	}
}

// NewUnauthorized creates a new 401 Unauthorized problem
func NewUnauthorized(detail string) Unauthorized {
	return Unauthorized{
		ProblemDetail: NewProblemDetail(
			"about:blank",
			"Unauthorized",
			401,
		),
		Realm: "API",
	}
}

// NewNotFound creates a new 404 Not Found problem
func NewNotFound(resource string) NotFound {
	return NotFound{
		ProblemDetail: NewProblemDetail(
			"about:blank",
			"Not Found",
			404,
		),
		Resource: resource,
	}
}
