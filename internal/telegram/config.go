package telegram

import (
	"ReAction/internal/config"
	"path/filepath"

	"github.com/zelenin/go-tdlib/client"
)

func DefaultTDLibConfig(cfg config.TelegramConfig) *client.SetTdlibParametersRequest {
	return &client.SetTdlibParametersRequest{
		UseTestDc:           false,
		DatabaseDirectory:   filepath.Join(".tdlib", "database"),
		FilesDirectory:      filepath.Join(".tdlib", "files"),
		UseFileDatabase:     true,
		UseChatInfoDatabase: true,
		UseMessageDatabase:  true,
		UseSecretChats:      false,
		ApiId:               cfg.APIID,
		ApiHash:             cfg.APIHash,
		SystemLanguageCode:  "en",
		DeviceModel:         "ReAction",
		SystemVersion:       "1.0.0",
		ApplicationVersion:  "1.0.0",
	}
}

func SetupLogging() error {
	_, err := client.SetLogVerbosityLevel(&client.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: 1,
	})
	return err
}
