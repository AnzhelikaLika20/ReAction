package storage

import (
	"context"
	"time"

	"ReAction/internal/storage/sqlc/gen"
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
	PhoneNumber string    `json:"phone_number"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (r *UserRepository) CreateUser(ctx context.Context, phoneNumber string) (*User, error) {
	dbUser, err := r.queries.CreateUser(ctx, phoneNumber)
	if err != nil {
		return nil, err
	}
	return dbUserToUser(dbUser), nil
}

func (r *UserRepository) GetOrCreateUser(ctx context.Context, phoneNumber string) (*User, error) {
	user, err := r.queries.GetUserByPhone(ctx, phoneNumber)
	if err == nil {
		return dbUserToUser(user), nil
	}

	return r.CreateUser(ctx, phoneNumber)
}

func (r *UserRepository) GetUserByPhone(ctx context.Context, phoneNumber string) (*User, error) {
	dbUser, err := r.queries.GetUserByPhone(ctx, phoneNumber)
	if err != nil {
		return nil, err
	}
	return dbUserToUser(dbUser), nil
}

func (r *UserRepository) UpdateLastAuth(ctx context.Context, phoneNumber string) error {
	return r.queries.UpdateUserLastAuth(ctx, phoneNumber)
}

func dbUserToUser(dbUser db.User) *User {
	return &User{
		PhoneNumber: dbUser.PhoneNumber,
		IsActive:    dbUser.IsActive.Bool,
		CreatedAt:   dbUser.CreatedAt.Time,
		UpdatedAt:   dbUser.UpdatedAt.Time,
	}
}
