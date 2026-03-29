package reminders

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"ReAction/internal/storage"
	db "ReAction/internal/storage/sqlc/gen"

	ics "github.com/arran4/golang-ical"
	"github.com/google/uuid"
)

type Service struct {
	secret []byte
	repo   *storage.ReminderRepository
}

func NewRemindersService(secret string, repo *storage.ReminderRepository) *Service {
	return &Service{secret: []byte(secret), repo: repo}
}

func (s *Service) BuildCalendarURL(userID, baseURL string) string {
	b64 := base64.URLEncoding.EncodeToString([]byte(userID))
	sig := s.signature(userID)
	return strings.TrimSuffix(baseURL, "/") + "/webcal/" + b64 + "/" + sig + "/calendar.ics"
}

func (s *Service) VerifySignature(userIDBase64, signature string) (userID string, ok bool) {
	raw, err := base64.URLEncoding.DecodeString(userIDBase64)
	if err != nil {
		return "", false
	}
	userID = string(raw)
	expected := s.signature(userID)
	if !hmac.Equal([]byte(signature), []byte(expected)) {
		return "", false
	}
	return userID, true
}

func (s *Service) signature(subject string) string {
	h := hmac.New(sha256.New, s.secret)
	h.Write([]byte(subject))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *Service) GetCalendarICS(ctx context.Context, userID string) (string, error) {
	if s.repo == nil {
		return "", fmt.Errorf("reminder repository is not configured")
	}
	now := time.Now().UTC()
	from := now.AddDate(0, -1, 0)
	to := now.AddDate(1, 0, 0)

	rows, err := s.repo.ListForUserInRange(ctx, userID, from, to)
	if err != nil {
		return "", fmt.Errorf("list reminders: %w", err)
	}

	return s.buildICS(rows, now), nil
}

func (s *Service) buildICS(rows []db.Reminder, stamp time.Time) string {
	cal := ics.NewCalendar()
	cal.SetMethod(ics.MethodPublish)
	cal.SetCalscale("GREGORIAN")
	cal.SetXWRCalName("ReAction")
	cal.SetXWRTimezone("UTC")
	cal.SetRefreshInterval("PT15M")
	cal.SetXPublishedTTL("PT15M")

	for _, r := range rows {
		if !r.ID.Valid {
			continue
		}
		idStr := uuid.UUID(r.ID.Bytes).String()
		ev := cal.AddEvent(idStr + "@reaction")

		start := r.StartsAt.Time.UTC()
		end := r.EndsAt.Time.UTC()
		if !end.After(start) {
			end = start.Add(time.Hour)
		}

		created := r.CreatedAt.Time.UTC()
		if created.IsZero() {
			created = stamp
		}
		ev.SetCreatedTime(created)
		ev.SetDtStampTime(stamp)
		ev.SetStartAt(start)
		ev.SetEndAt(end)
		ev.SetSummary(r.Title)
		if r.Description.Valid && strings.TrimSpace(r.Description.String) != "" {
			ev.SetDescription(r.Description.String)
		}

		if r.NotifyBeforeMinutes > 0 {
			al := ev.AddAlarm()
			al.SetTrigger(fmt.Sprintf("-PT%dM", r.NotifyBeforeMinutes))
			al.SetAction(ics.ActionDisplay)
			al.SetDescription("Напоминание")
		}
	}

	return cal.Serialize()
}
