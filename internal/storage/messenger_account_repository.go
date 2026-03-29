package storage

import (
	"context"
	"time"

	db "ReAction/internal/storage/sqlc/gen"

	"github.com/jackc/pgx/v5/pgtype"
)

type MessengerAccountRepository struct {
	queries *db.Queries
}

func NewMessengerAccountRepository(queries *db.Queries) *MessengerAccountRepository {
	return &MessengerAccountRepository{queries: queries}
}

func (r *MessengerAccountRepository) InsertPendingTelegram(ctx context.Context, userID string) (string, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return "", err
	}
	row, err := r.queries.InsertMessengerAccount(ctx, db.InsertMessengerAccountParams{
		UserID:           uid,
		Provider:         db.MessengerProviderTelegram,
		Label:            pgtype.Text{},
		ConnectionStatus: db.MessengerConnectionStatusPending,
		ConnectedAt:      pgtype.Timestamptz{},
	})
	if err != nil {
		return "", err
	}
	return UUIDToString(row.ID), nil
}

func (r *MessengerAccountRepository) GetLatestPendingTelegramAccountID(ctx context.Context, userID string) (string, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return "", err
	}
	row, err := r.queries.GetLatestPendingTelegramAccountByUserID(ctx, uid)
	if err != nil {
		return "", err
	}
	return UUIDToString(row.ID), nil
}

func (r *MessengerAccountRepository) ListByUserID(ctx context.Context, userID string) ([]db.UserMessengerAccount, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	return r.queries.ListMessengerAccountsByUserID(ctx, uid)
}

func (r *MessengerAccountRepository) EnsureMessengerOwnedByUser(ctx context.Context, accountID, userID string) error {
	aid, err := ParseUUID(accountID)
	if err != nil {
		return err
	}
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = r.queries.GetMessengerAccountByIDForUser(ctx, db.GetMessengerAccountByIDForUserParams{
		ID:     aid,
		UserID: uid,
	})
	return err
}

func (r *MessengerAccountRepository) LatestConnectedTelegramLabel(ctx context.Context, userID string) (string, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return "", err
	}
	label, err := r.queries.GetLatestConnectedTelegramLabelByUserID(ctx, uid)
	if err != nil {
		return "", err
	}
	if !label.Valid {
		return "", nil
	}
	return label.String, nil
}

func (r *MessengerAccountRepository) MarkTelegramConnected(ctx context.Context, accountID, userID string) error {
	aid, err := ParseUUID(accountID)
	if err != nil {
		return err
	}
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = r.queries.UpdateMessengerAccountStatus(ctx, db.UpdateMessengerAccountStatusParams{
		ID:               aid,
		UserID:           uid,
		ConnectionStatus: db.MessengerConnectionStatusConnected,
		ConnectedAt:      pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
	})
	return err
}

func (r *MessengerAccountRepository) SetTelegramPhoneLabel(ctx context.Context, accountID, userID, phoneE164 string) error {
	if phoneE164 == "" {
		return nil
	}
	aid, err := ParseUUID(accountID)
	if err != nil {
		return err
	}
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = r.queries.UpdateMessengerAccountLabel(ctx, db.UpdateMessengerAccountLabelParams{
		ID:     aid,
		UserID: uid,
		Label:  pgtype.Text{String: phoneE164, Valid: true},
	})
	return err
}
