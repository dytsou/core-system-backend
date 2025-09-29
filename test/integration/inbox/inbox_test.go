package inbox

import (
	"NYCU-SDC/core-system-backend/internal/inbox"
	"NYCU-SDC/core-system-backend/internal/unit"
	"NYCU-SDC/core-system-backend/test/integration"
	formbuilder "NYCU-SDC/core-system-backend/test/testdata/dbbuilder/form"
	unitbuilder "NYCU-SDC/core-system-backend/test/testdata/dbbuilder/unit"
	userbuilder "NYCU-SDC/core-system-backend/test/testdata/dbbuilder/user"
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

var rollback func()

func TestMain(m *testing.M) {
	resourceManager, _, err := integration.GetOrInitResource()
	if err != nil {
		panic(err)
	}

	_, rb, err := resourceManager.SetupPostgres()
	if err != nil {
		panic(err)
	}
	rollback = rb

	code := m.Run()

	rollback()
	resourceManager.Cleanup()

	os.Exit(code)
}

func withTx(t *testing.T, fn func(tx pgx.Tx)) {
	resourceManager, logger, err := integration.GetOrInitResource()
	require.NoError(t, err)

	tx, cleanup, err := resourceManager.SetupPostgres()
	require.NoError(t, err)
	defer cleanup()

	fn(tx)

	_ = logger // silence unused when not used directly in tests
}

func seedOrgUnitUsers(t *testing.T, tx pgx.Tx) (unit.Unit, unit.Unit, uuid.UUID, uuid.UUID) {
	unitBuilder := unitbuilder.New(t, tx)
	org := unitBuilder.Create(unit.UnitTypeOrganization)
	unit := unitBuilder.Create(unit.UnitTypeUnit, unitbuilder.WithParent(org.ID))

	unitBuilder.AddMember(unit.ID, userbuilder.New(t, tx).Create().ID)
	userA := userbuilder.New(t, tx).Create().ID
	unitBuilder.AddMember(unit.ID, userA)
	unitBuilder.AddMember(unit.ID, userbuilder.New(t, tx).Create().ID)
	unitBuilder.AddMember(unit.ID, userbuilder.New(t, tx).Create().ID)
	unitBuilder.AddMember(unit.ID, userbuilder.New(t, tx).Create().ID)
	userB := userbuilder.New(t, tx).Create().ID
	unitBuilder.AddMember(unit.ID, userB)

	return org, unit, userA, userB
}

func TestService_CreateListGetUpdate(t *testing.T) {
	withTx(t, func(tx pgx.Tx) {
		resourceManager, logger, err := integration.GetOrInitResource()
		require.NoError(t, err)
		_ = resourceManager

		_, unit, userA, userB := seedOrgUnitUsers(t, tx)

		formBuilder := formbuilder.New(t, tx)
		formRow := formBuilder.Create(
			formbuilder.WithUnitID(unit.ID),
			formbuilder.WithLastEditor(userA),
		)

		service := inbox.NewService(logger, tx)

		// Create a message for userA and userB
		messageID, err := service.Create(context.Background(), inbox.ContentTypeForm, formRow.ID, []uuid.UUID{userA, userB}, unit.ID)
		require.NoError(t, err)
		require.NotEqual(t, uuid.Nil, messageID)

		// List for userA and check if the message is created
		rowsA, err := service.List(context.Background(), userA)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(rowsA), 1)
		found := false
		var uimID uuid.UUID
		for _, r := range rowsA {
			if r.ContentID == formRow.ID {
				found = true
				uimID = r.ID
				require.Equal(t, inbox.ContentTypeForm, r.Type)
				_ = r.PreviewMessage
				_ = r.Title
				_ = r.OrgName
				_ = r.UnitName
			}
		}
		require.True(t, found)

		// GetByID for that inbox item and check if the message is created
		detail, err := service.GetByID(context.Background(), uimID, userA)
		require.NoError(t, err)
		require.Equal(t, formRow.ID, detail.ContentID)
		require.Equal(t, inbox.ContentTypeForm, detail.Type)

		// UpdateByID flags and check if the message is updated
		updated, err := service.UpdateByID(context.Background(), uimID, userA, inbox.UserInboxMessageFilter{IsRead: true, IsStarred: true, IsArchived: false})
		require.NoError(t, err)
		require.True(t, updated.IsRead)
		require.True(t, updated.IsStarred)
		require.False(t, updated.IsArchived)
	})
}

func TestInboxService_ListEmpty(t *testing.T) {
	resourceManager, logger, err := integration.GetOrInitResource()
	if err != nil {
		t.Fatalf("failed to get resource manager: %v", err)
	}

	db, rollback, err := resourceManager.SetupPostgres()
	if err != nil {
		t.Fatalf("failed to setup postgres: %v", err)
	}
	defer rollback()

	// Create an org and unit with a user but do not create any inbox messages and check if the list is empty
	unitB := unitbuilder.New(t, db)
	org := unitB.Create(unit.UnitTypeOrganization)
	unit := unitB.Create(unit.UnitTypeUnit, unitbuilder.WithParent(org.ID))
	userID := userbuilder.New(t, db).Create().ID
	unitB.AddMember(unit.ID, userID)

	service := inbox.NewService(logger, db)
	rows, err := service.List(context.Background(), userID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestInboxService_UpdateByIDFlags(t *testing.T) {
	type params struct {
		update inbox.UserInboxMessageFilter
		expect inbox.UserInboxMessageFilter
	}

	testCases := []struct {
		name   string
		params params
	}{
		{
			name: "mark read and starred",
			params: params{
				update: inbox.UserInboxMessageFilter{IsRead: true, IsStarred: true, IsArchived: false},
				expect: inbox.UserInboxMessageFilter{IsRead: true, IsStarred: true, IsArchived: false},
			},
		},
		{
			name: "archive only",
			params: params{
				update: inbox.UserInboxMessageFilter{IsRead: false, IsStarred: false, IsArchived: true},
				expect: inbox.UserInboxMessageFilter{IsRead: false, IsStarred: false, IsArchived: true},
			},
		},
		{
			name: "unstar and read",
			params: params{
				update: inbox.UserInboxMessageFilter{IsRead: true, IsStarred: false, IsArchived: false},
				expect: inbox.UserInboxMessageFilter{IsRead: true, IsStarred: false, IsArchived: false},
			},
		},
	}

	resourceManager, logger, err := integration.GetOrInitResource()
	if err != nil {
		t.Fatalf("failed to get resource manager: %v", err)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, rollback, err := resourceManager.SetupPostgres()
			if err != nil {
				t.Fatalf("failed to setup postgres: %v", err)
			}
			defer rollback()

			// Seed unit and user and create a message
			unitB := unitbuilder.New(t, db)
			org := unitB.Create(unit.UnitTypeOrganization)
			unit := unitB.Create(unit.UnitTypeUnit, unitbuilder.WithParent(org.ID))
			userID := userbuilder.New(t, db).Create().ID
			unitB.AddMember(unit.ID, userID)

			// Create form and deliver and create a message
			formBuilder := formbuilder.New(t, db)
			form := formBuilder.Create(
				formbuilder.WithUnitID(unit.ID),
				formbuilder.WithLastEditor(userID),
			)

			service := inbox.NewService(logger, db)
			_, err = service.Create(context.Background(), inbox.ContentTypeForm, form.ID, []uuid.UUID{userID}, unit.ID)
			require.NoError(t, err)

			// List for userID and check if the message is created
			rows, err := service.List(context.Background(), userID)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			uimID := rows[0].ID

			// UpdateByID and check if the message is updated
			updated, err := service.UpdateByID(context.Background(), uimID, userID, tc.params.update)
			require.NoError(t, err)
			require.Equal(t, tc.params.expect.IsRead, updated.IsRead)
			require.Equal(t, tc.params.expect.IsStarred, updated.IsStarred)
			require.Equal(t, tc.params.expect.IsArchived, updated.IsArchived)
		})
	}
}

func TestInboxService_DuplicateCreatesProduceMultipleMessages(t *testing.T) {
	resourceManager, logger, err := integration.GetOrInitResource()
	if err != nil {
		t.Fatalf("failed to get resource manager: %v", err)
	}

	db, rollback, err := resourceManager.SetupPostgres()
	if err != nil {
		t.Fatalf("failed to setup postgres: %v", err)
	}
	defer rollback()

	unitB := unitbuilder.New(t, db)
	org := unitB.Create(unit.UnitTypeOrganization)
	unit := unitB.Create(unit.UnitTypeUnit, unitbuilder.WithParent(org.ID))
	userID := userbuilder.New(t, db).Create().ID
	unitB.AddMember(unit.ID, userID)

	formB := formbuilder.New(t, db)
	f := formB.Create(
		formbuilder.WithUnitID(unit.ID),
		formbuilder.WithLastEditor(userID),
	)

	svc := inbox.NewService(logger, db)

	// Create two separate messages with same content/recipient intentionally
	id1, err := svc.Create(context.Background(), inbox.ContentTypeForm, f.ID, []uuid.UUID{userID}, unit.ID)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id1)
	id2, err := svc.Create(context.Background(), inbox.ContentTypeForm, f.ID, []uuid.UUID{userID}, unit.ID)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id2)
	require.NotEqual(t, id1, id2)

	rows, err := svc.List(context.Background(), userID)
	require.NoError(t, err)

	// Count messages for this contentID
	count := 0
	for _, r := range rows {
		if r.ContentID == f.ID {
			count++
		}
	}
	require.GreaterOrEqual(t, count, 2, "expect multiple messages for repeated Create calls")
}

func TestInboxService_ArchiveVisibilityInList(t *testing.T) {
	resourceManager, logger, err := integration.GetOrInitResource()
	if err != nil {
		t.Fatalf("failed to get resource manager: %v", err)
	}

	db, rollback, err := resourceManager.SetupPostgres()
	if err != nil {
		t.Fatalf("failed to setup postgres: %v", err)
	}
	defer rollback()

	unitB := unitbuilder.New(t, db)
	org := unitB.Create(unit.UnitTypeOrganization)
	unit := unitB.Create(unit.UnitTypeUnit, unitbuilder.WithParent(org.ID))
	userID := userbuilder.New(t, db).Create().ID
	unitB.AddMember(unit.ID, userID)

	formB := formbuilder.New(t, db)
	f := formB.Create(
		formbuilder.WithUnitID(unit.ID),
		formbuilder.WithLastEditor(userID),
	)

	svc := inbox.NewService(logger, db)
	_, err = svc.Create(context.Background(), inbox.ContentTypeForm, f.ID, []uuid.UUID{userID}, unit.ID)
	require.NoError(t, err)

	rows, err := svc.List(context.Background(), userID)
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	var uimID uuid.UUID
	for _, r := range rows {
		if r.ContentID == f.ID {
			uimID = r.ID
			break
		}
	}
	require.NotEqual(t, uuid.Nil, uimID)

	// Archive the message
	_, err = svc.UpdateByID(context.Background(), uimID, userID, inbox.UserInboxMessageFilter{IsRead: true, IsStarred: false, IsArchived: true})
	require.NoError(t, err)

	// Service List (no filters) still returns archived messages per current query
	rows2, err := svc.List(context.Background(), userID)
	require.NoError(t, err)
	found := false
	for _, r := range rows2 {
		if r.ID == uimID {
			found = true
			require.True(t, r.IsArchived)
			break
		}
	}
	require.False(t, found, "archived message should not appear in List")
}
