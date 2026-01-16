package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"ReAction/internal/storage/sqlc/gen"
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
	TokenHash   string    `json:"token_hash"`
	PhoneNumber string    `json:"phone_number"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
}

func (r *SessionRepository) CreateSession(ctx context.Context, sessionID, phoneNumber string) error {
	_, err := r.queries.CreateSession(ctx, db.CreateSessionParams{
		TokenHash:   sessionID,
		PhoneNumber: phoneNumber,
	})
	return err
}

func (r *SessionRepository) ValidateSession(ctx context.Context, token string) (bool, error) {
	tokenHash := hashToken(token)

	session, err := r.queries.GetSession(ctx, tokenHash)
	if err != nil {
		return false, err
	}

	if time.Since(session.CreatedAt.Time) > 30*24*time.Hour {
		_ = r.queries.DeleteSession(ctx, tokenHash)
		return false, nil
	}

	return true, nil
}

func (r *SessionRepository) GetSession(ctx context.Context, token string) (*Session, error) {
	dbSession, err := r.queries.GetSession(ctx, token)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return dbSessionToSession(dbSession), nil
}

func dbSessionToSession(dbSession db.Session) *Session {
	return &Session{
		TokenHash:   dbSession.TokenHash,
		PhoneNumber: dbSession.PhoneNumber,
		CreatedAt:   dbSession.CreatedAt.Time,
	}
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
