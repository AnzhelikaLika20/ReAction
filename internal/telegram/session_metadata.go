package telegram

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

const sessionMetadataFileName = "reaction-session.json"

type SessionMetadata struct {
	MessengerAccountID string `json:"messenger_account_id"`
	AppUserID          string `json:"app_user_id"`
	TelegramPhone      string `json:"telegram_phone,omitempty"`
}

func sessionMetadataPath(sessionsRoot, messengerAccountID string) string {
	return filepath.Join(sessionsRoot, "db", messengerAccountID, sessionMetadataFileName)
}

func WriteSessionMetadata(sessionsRoot, messengerAccountID, appUserID, telegramPhone string) error {
	if sessionsRoot == "" || messengerAccountID == "" {
		return errors.New("sessions root and messenger account id are required")
	}
	dir := filepath.Join(sessionsRoot, "db", messengerAccountID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	meta := SessionMetadata{
		MessengerAccountID: messengerAccountID,
		AppUserID:          appUserID,
		TelegramPhone:      telegramPhone,
	}
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	tmp := sessionMetadataPath(sessionsRoot, messengerAccountID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, sessionMetadataPath(sessionsRoot, messengerAccountID))
}

func RemoveSessionMetadata(sessionsRoot, messengerAccountID string) {
	if sessionsRoot == "" || messengerAccountID == "" {
		return
	}
	_ = os.Remove(sessionMetadataPath(sessionsRoot, messengerAccountID))
}

func ReadSessionMetadata(sessionsRoot, messengerAccountID string) (*SessionMetadata, error) {
	path := sessionMetadataPath(sessionsRoot, messengerAccountID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta SessionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

func ListSessionMetadata(sessionsRoot string) ([]SessionMetadata, error) {
	if sessionsRoot == "" {
		return nil, nil
	}
	dbRoot := filepath.Join(sessionsRoot, "db")
	entries, err := os.ReadDir(dbRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []SessionMetadata
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirName := e.Name()
		if _, err := uuid.Parse(dirName); err != nil {
			continue
		}
		meta, err := readSessionMetadataFile(filepath.Join(dbRoot, dirName, sessionMetadataFileName))
		if err != nil {
			continue
		}
		if meta.MessengerAccountID != dirName {
			continue
		}
		out = append(out, *meta)
	}
	return out, nil
}

func readSessionMetadataFile(path string) (*SessionMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta SessionMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}
