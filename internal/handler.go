package internal

import (
	"fmt"
	"github.com/google/uuid"
)

type contextKey string

var UserContextKey contextKey = "user"

func ParseUUID(value string) (uuid.UUID, error) {
	parsedUUID, err := uuid.Parse(value)
	if err != nil {
		return parsedUUID, fmt.Errorf("failed to parse UUID: %w", err)
	}

	return parsedUUID, nil
}
