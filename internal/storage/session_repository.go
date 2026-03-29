package storage

import (
	"context"
	"errors"
	"time"

	db "ReAction/internal/storage/sqlc/gen"

	"github.com/jackc/pgx/v5"
)

type SessionRepository struct {
	queries *db.Queries
}

func NewSessionRepository(queries *db.Queries) *SessionRepository {
	return &SessionRepository{
		queries: queries,
	}
}

type Session struct {
	TokenHash string    `json:"token_hash"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *SessionRepository) CreateSession(ctx context.Context, sessionID, userID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = r.queries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash: sessionID,
		UserID:    uid,
	})
	return err
}

func (r *SessionRepository) DeleteSession(ctx context.Context, sessionID string) error {
	return r.queries.DeleteSession(ctx, sessionID)
}

func (r *SessionRepository) GetSession(ctx context.Context, tokenHash string) (*Session, error) {
	dbSession, err := r.queries.GetSession(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &Session{
		TokenHash: dbSession.TokenHash,
		UserID:    UUIDToString(dbSession.UserID),
		CreatedAt: dbSession.CreatedAt.Time,
	}, nil
}
