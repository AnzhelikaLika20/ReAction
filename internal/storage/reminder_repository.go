package storage

import (
	"context"
	"fmt"
	"time"

	db "ReAction/internal/storage/sqlc/gen"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ReminderRepository struct {
	q *db.Queries
}

func NewReminderRepository(q *db.Queries) *ReminderRepository {
	return &ReminderRepository{q: q}
}

type CreateReminderRecordParams struct {
	ScenarioID          string
	ChatID              int64
	Title               string
	Description         string
	StartsAt            time.Time
	EndsAt              time.Time
	NotifyBeforeMinutes int32
}

func (r *ReminderRepository) Create(ctx context.Context, p CreateReminderRecordParams) (db.Reminder, error) {
	sid, err := uuid.Parse(p.ScenarioID)
	if err != nil {
		return db.Reminder{}, fmt.Errorf("scenario id: %w", err)
	}

	var chatID pgtype.Int8
	if p.ChatID != 0 {
		chatID = pgtype.Int8{Int64: p.ChatID, Valid: true}
	}

	desc := pgtype.Text{String: p.Description, Valid: p.Description != ""}

	row, err := r.q.CreateReminder(ctx, db.CreateReminderParams{
		ScenarioID:          pgtype.UUID{Bytes: sid, Valid: true},
		ChatID:              chatID,
		Title:               p.Title,
		Description:         desc,
		StartsAt:            pgtype.Timestamptz{Time: p.StartsAt, Valid: true},
		EndsAt:              pgtype.Timestamptz{Time: p.EndsAt, Valid: true},
		NotifyBeforeMinutes: p.NotifyBeforeMinutes,
	})
	if err != nil {
		return db.Reminder{}, fmt.Errorf("create reminder: %w", err)
	}
	return row, nil
}

func (r *ReminderRepository) ListRecentForChat(ctx context.Context, userID string, chatID int64, since time.Time) ([]db.Reminder, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	return r.q.ListRecentRemindersForChat(ctx, db.ListRecentRemindersForChatParams{
		UserID:    uid,
		ChatID:    pgtype.Int8{Int64: chatID, Valid: true},
		CreatedAt: pgtype.Timestamptz{Time: since.UTC(), Valid: true},
	})
}

func (r *ReminderRepository) ListForUserInRange(ctx context.Context, userID string, from, to time.Time) ([]db.Reminder, error) {
	uid, err := ParseUUID(userID)
	if err != nil {
		return nil, err
	}
	return r.q.ListRemindersForUserInRange(ctx, db.ListRemindersForUserInRangeParams{
		UserID:     uid,
		StartsAt:   pgtype.Timestamptz{Time: from.UTC(), Valid: true},
		StartsAt_2: pgtype.Timestamptz{Time: to.UTC(), Valid: true},
	})
}
