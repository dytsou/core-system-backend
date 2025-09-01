package distribute_test

import (
	"NYCU-SDC/core-system-backend/internal/distribute"
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeUnitStore implements UnitStore for testing purpose
type fakeUnitStore struct {
	orgUsers  map[uuid.UUID][]uuid.UUID
	unitUsers map[uuid.UUID][]uuid.UUID
}

func (f *fakeUnitStore) UsersByOrg(ctx context.Context, orgID uuid.UUID) ([]uuid.UUID, error) {
	fmt.Printf("UsersByOrg called with orgID: %s, returning users: %v\n", orgID, f.orgUsers[orgID])
	return f.orgUsers[orgID], nil
}

func (f *fakeUnitStore) UsersByUnit(ctx context.Context, unitID uuid.UUID) ([]uuid.UUID, error) {
	fmt.Printf("UsersByUnit called with unitID: %s, returning users: %v\n", unitID, f.unitUsers[unitID])
	return f.unitUsers[unitID], nil
}

func TestService_GetRecipients(t *testing.T) {
	u1 := uuid.New()
	u2 := uuid.New()
	u3 := uuid.New()
	u4 := uuid.New()

	orgKey1 := uuid.New()
	orgKey2 := uuid.New()
	unitKey1 := uuid.New()
	unitKey2 := uuid.New()

	tests := []struct {
		name        string
		orgIDs      []uuid.UUID
		unitIDs     []uuid.UUID
		store       *fakeUnitStore
		expect      []uuid.UUID
		expectCount int
	}{
		{
			name:    "Should return empty when no recipients at all",
			orgIDs:  []uuid.UUID{uuid.New()},
			unitIDs: []uuid.UUID{uuid.New()},
			store: &fakeUnitStore{
				orgUsers:  map[uuid.UUID][]uuid.UUID{},
				unitUsers: map[uuid.UUID][]uuid.UUID{},
			},
			expect:      []uuid.UUID{},
			expectCount: 0,
		},
		{
			name:    "Should deduplicate recipients across org and unit",
			orgIDs:  []uuid.UUID{orgKey1},
			unitIDs: []uuid.UUID{unitKey1},
			store: &fakeUnitStore{
				orgUsers: map[uuid.UUID][]uuid.UUID{
					orgKey1: {u1, u2},
				},
				unitUsers: map[uuid.UUID][]uuid.UUID{
					unitKey1: {u2, u3},
				},
			},
			expect:      []uuid.UUID{u1, u2, u3},
			expectCount: 3,
		},
		{
			name:    "Should return recipients only from orgs when no units",
			orgIDs:  []uuid.UUID{orgKey1},
			unitIDs: []uuid.UUID{},
			store: &fakeUnitStore{
				orgUsers: map[uuid.UUID][]uuid.UUID{
					orgKey1: {u1, u2},
				},
				unitUsers: map[uuid.UUID][]uuid.UUID{},
			},
			expect:      []uuid.UUID{u1, u2},
			expectCount: 2,
		},
		{
			name:    "Should return recipients only from units when no orgs",
			orgIDs:  []uuid.UUID{},
			unitIDs: []uuid.UUID{unitKey1},
			store: &fakeUnitStore{
				orgUsers: map[uuid.UUID][]uuid.UUID{},
				unitUsers: map[uuid.UUID][]uuid.UUID{
					unitKey1: {u3, u4},
				},
			},
			expect:      []uuid.UUID{u3, u4},
			expectCount: 2,
		},
		{
			name:    "Should deduplicate when org and unit return identical users",
			orgIDs:  []uuid.UUID{orgKey1},
			unitIDs: []uuid.UUID{unitKey1},
			store: &fakeUnitStore{
				orgUsers: map[uuid.UUID][]uuid.UUID{
					orgKey1: {u1, u2},
				},
				unitUsers: map[uuid.UUID][]uuid.UUID{
					unitKey1: {u1, u2},
				},
			},
			expect:      []uuid.UUID{u1, u2},
			expectCount: 2,
		},
		{
			name:    "Should merge recipients from multiple orgs and units",
			orgIDs:  []uuid.UUID{orgKey1, orgKey2},
			unitIDs: []uuid.UUID{unitKey1, unitKey2},
			store: &fakeUnitStore{
				orgUsers: map[uuid.UUID][]uuid.UUID{
					orgKey1: {u1},
					orgKey2: {u2},
				},
				unitUsers: map[uuid.UUID][]uuid.UUID{
					unitKey1: {u2, u3},
					unitKey2: {u4},
				},
			},
			expect:      []uuid.UUID{u1, u2, u3, u4},
			expectCount: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			svc := distribute.NewService(logger, tt.store)

			got, err := svc.GetRecipients(context.Background(), tt.orgIDs, tt.unitIDs)
			require.NoError(t, err)

			// check total number of recipients
			require.Equal(t, tt.expectCount, len(got),
				"expected %d recipients %v, but got %d, %v", tt.expectCount, tt.expect, len(got), got)

			// check that all expected recipients are included in result
			for _, exp := range tt.expect {
				require.Contains(t, got, exp)
			}
		})
	}
}
