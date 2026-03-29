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
	ID          string    `json:"id"`
	Email       string    `json:"email,omitempty"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func textString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func rowToUser(id pgtype.UUID, email, phone pgtype.Text, isActive pgtype.Bool, createdAt, updatedAt pgtype.Timestamptz) *User {
	return &User{
		ID:          UUIDToString(id),
		Email:       textString(email),
		PhoneNumber: textString(phone),
		IsActive:    isActive.Bool,
		CreatedAt:   createdAt.Time,
		UpdatedAt:   updatedAt.Time,
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
	return rowToUser(row.ID, row.Email, row.PhoneNumber, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserRepository) CreateUserWithPhone(ctx context.Context, phoneNumber string) (*User, error) {
	row, err := r.queries.CreateUserWithPhone(ctx, pgtype.Text{String: phoneNumber, Valid: true})
	if err != nil {
		return nil, err
	}
	return rowToUser(row.ID, row.Email, row.PhoneNumber, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserRepository) GetUserByEmail(ctx context.Context, email string) (*User, string, error) {
	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, "", nil
		}
		return nil, "", err
	}
	u := rowToUser(row.ID, row.Email, row.PhoneNumber, row.IsActive, row.CreatedAt, row.UpdatedAt)
	hash := ""
	if row.PasswordHash.Valid {
		hash = row.PasswordHash.String
	}
	return u, hash, nil
}

func (r *UserRepository) GetUserByPhone(ctx context.Context, phoneNumber string) (*User, error) {
	row, err := r.queries.GetUserByPhone(ctx, pgtype.Text{String: phoneNumber, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rowToUser(row.ID, row.Email, row.PhoneNumber, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
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
	return rowToUser(row.ID, row.Email, row.PhoneNumber, row.IsActive, row.CreatedAt, row.UpdatedAt), nil
}

func (r *UserRepository) UpdateTelegramPhone(ctx context.Context, userID, telegramPhone string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	_, err = r.queries.UpdateUserTelegramPhone(ctx, db.UpdateUserTelegramPhoneParams{
		ID:          uid,
		PhoneNumber: pgtype.Text{String: telegramPhone, Valid: true},
	})
	return err
}

func (r *UserRepository) UpdateLastAuth(ctx context.Context, userID string) error {
	uid, err := ParseUUID(userID)
	if err != nil {
		return err
	}
	return r.queries.UpdateUserLastAuth(ctx, uid)
}
