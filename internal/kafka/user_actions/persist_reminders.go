package user_actions

import (
	"context"
	"errors"
	"log"
	"time"

	"ReAction/internal/storage"

	"github.com/jackc/pgx/v5"
)

func NewSaveReminderToDBHandler(
	reminderRepo *storage.ReminderRepository,
	scenarioRepo *storage.ScenarioRepository,
) UserActionHandler {
	return func(action *UserActionEvent) error {
		if action.Reminder == nil {
			return nil
		}
		if action.UserID == "" || action.ScenarioID == "" {
			log.Printf("[USER-ACTIONS] skip reminder persist: missing user_id or scenario_id")
			return nil
		}

		ctx := context.Background()
		sc, err := scenarioRepo.GetByID(ctx, action.ScenarioID, action.UserID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				log.Printf("[USER-ACTIONS] scenario %s not found for user", action.ScenarioID)
				return nil
			}
			return err
		}

		notifyBefore, err := storage.ReminderMinutesBeforeFromParams(sc.Params)
		if err != nil {
			log.Printf("[USER-ACTIONS] scenario %s params: %v", action.ScenarioID, err)
			return nil
		}

		startsAt := action.Reminder.Date
		endsAt := action.Reminder.EndDate
		if endsAt.IsZero() {
			endsAt = startsAt.Add(time.Hour)
		}

		_, err = reminderRepo.Create(ctx, storage.CreateReminderRecordParams{
			ScenarioID:          action.ScenarioID,
			ChatID:              action.ChatID,
			Title:               action.Reminder.Title,
			Description:         action.Reminder.Description,
			StartsAt:            startsAt,
			EndsAt:              endsAt,
			NotifyBeforeMinutes: notifyBefore,
		})
		if err != nil {
			return err
		}

		log.Printf("[USER-ACTIONS] reminder saved to DB: scenario_id=%s title=%q", action.ScenarioID, action.Reminder.Title)
		return nil
	}
}
