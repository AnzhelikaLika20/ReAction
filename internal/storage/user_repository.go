package storage

import (
	"context"
	"errors"
	"time"

	db "ReAction/internal/storage/sqlc/gen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserRepository struct {
	queries *db.Queries
}

func NewUserRepository(queries *db.Queries) *UserRepository {
	return &UserRepository{
		queries: queries,
	}
}

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email,omitempty"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func rowToUser(id pgtype.UUID, email string, isActive pgtype.Bool, createdAt, updatedAt pgtype.Timestamptz) *User {
	return &User{
		ID:        UUIDToString(id),
		Email:     email,
		IsActive:  isActive.Bool,
		CreatedAt: createdAt.Time,
		UpdatedAt: updatedAt.Time,
	}
}

func (r *UserRepository) CreateUserWithCredentials(ctx context.Context, email, passwordHash string) (*User, error) {
	row, err := r.queries.CreateUserWithCredentials(ctx, db.CreateUserWithCredentialsParams{
		Lower:        email,
		PasswordHash: pgtype.Text{String: passwordHash, Valid: true},
	})
	if err != nil {
		return nil, err
	}
	return rowToUser(row.ID, row.Email, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}
	u := rowToUser(row.ID, row.Email, row.IsActive, row.CreatedAt, row.UpdatedAt)
	hash := ""
	if row.PasswordHash.Valid {
		hash = row.PasswordHash.String
	}
	return u, hash, nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, id string) (*User, error) {
	uid, err := ParseUUID(id)
	if err != nil {
		return nil, err
	}
	row, err := r.queries.GetUserByID(ctx, uid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rowToUser(row.ID, row.Email, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserRepository) UpdateLastAuth(ctx context.Context, userID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.UpdateUserLastAuth(ctx, uid)
}

func (r *UserRepository) PasswordHashByUserID(ctx context.Context, userID string) (string, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return "", err
	}
	row, err := r.queries.GetUserByID(ctx, uid)
	if err != nil {
		return "", err
	}
	if !row.PasswordHash.Valid || row.PasswordHash.String == "" {
		return "", errors.New("user has no password hash")
	}
	return row.PasswordHash.String, nil
}

func (r *UserRepository) DeleteUserByID(ctx context.Context, userID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.DeleteUser(ctx, uid)
}
