package storage

import (
	"context"
	"errors"
	"time"

	db "ReAction/internal/storage/sqlc/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrRefreshTokenNotFound = errors.New("refresh token not found or expired")

type RefreshTokenRepository struct {
	queries *db.Queries
}

func NewRefreshTokenRepository(queries *db.Queries) *RefreshTokenRepository {
	return &RefreshTokenRepository{queries: queries}
}

func (r *RefreshTokenRepository) Insert(ctx context.Context, userID, tokenHash string, expiresAt time.Time) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.InsertRefreshToken(ctx, db.InsertRefreshTokenParams{
		UserID:    uid,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
}

func (r *RefreshTokenRepository) GetByHash(ctx context.Context, tokenHash string) (string, error) {
	row, err := r.queries.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrRefreshTokenNotFound
		}
		return "", err
	}
	return UUIDToString(row.UserID), nil
}

func (r *RefreshTokenRepository) Delete(ctx context.Context, tokenHash string) error {
	return r.queries.DeleteRefreshToken(ctx, tokenHash)
}

func (r *RefreshTokenRepository) DeleteAllForUser(ctx context.Context, userID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.DeleteAllRefreshTokensForUser(ctx, pgtype.UUID(uid))
}
