package telegram

import (
	"ReAction/internal/config"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

type SimpleAuthorizer struct {
	PhoneNumber chan string
	Code        chan string
	Password    chan string
	State       chan client.AuthorizationState
	cfg         config.TelegramConfig
	mu          sync.RWMutex
}

func NewSimpleAuthorizer(cfg config.TelegramConfig) *SimpleAuthorizer {
	return &SimpleAuthorizer{
		PhoneNumber: make(chan string, 1),
		Code:        make(chan string, 1),
		Password:    make(chan string, 1),
		State:       make(chan client.AuthorizationState, 100),
		cfg:         cfg,
	}
}

func (a *SimpleAuthorizer) Handle(tdlibClient *client.Client, state client.AuthorizationState) error {
	select {
	case a.State <- state:
	default:
		log.Printf("[AUTHORIZER] Warning: state channel is full, dropping state")
	}

	switch state.AuthorizationStateType() {
	case client.TypeAuthorizationStateWaitTdlibParameters:
		_, err := tdlibClient.SetTdlibParameters(&client.SetTdlibParametersRequest{
			UseFileDatabase:     true,
			UseChatInfoDatabase: true,
			UseMessageDatabase:  true,
			UseSecretChats:      false,
			SystemLanguageCode:  "en",
			DeviceModel:         "Server",
			SystemVersion:       "1.0",
			ApplicationVersion:  "1.0",
			ApiId:               a.cfg.APIID,
			ApiHash:             a.cfg.APIHash,
		})
		if err != nil {
			log.Printf("[AUTHORIZER] Failed to set TDLib parameters: %v", err)
			return err
		}
		log.Printf("[AUTHORIZER] TDLib parameters set successfully")
		return nil

	case client.TypeAuthorizationStateWaitPhoneNumber:
		select {
		case phone := <-a.PhoneNumber:
			log.Printf("[AUTHORIZER] Got phone number: %s", phone)
			_, err := tdlibClient.SetAuthenticationPhoneNumber(&client.SetAuthenticationPhoneNumberRequest{
				PhoneNumber: phone,
				Settings: &client.PhoneNumberAuthenticationSettings{
					AllowFlashCall:       false,
					IsCurrentPhoneNumber: false,
					AllowSmsRetrieverApi: false,
				},
			})
			if err != nil {
				log.Printf("[AUTHORIZER] Failed to set phone number: %v", err)
			}
			return err
		case <-time.After(5 * time.Minute):
			return errors.New("[AUTHORIZER] timeout waiting for phone number")
		}

	case client.TypeAuthorizationStateWaitCode:
		select {
		case code := <-a.Code:
			log.Printf("[AUTHORIZER] Got code: %s", code)
			_, err := tdlibClient.CheckAuthenticationCode(&client.CheckAuthenticationCodeRequest{
				Code: code,
			})
			if err != nil {
				log.Printf("[AUTHORIZER] Failed to check authentication code: %v", err)
			}
			return err
		case <-time.After(5 * time.Minute):
			return errors.New("[AUTHORIZER] timeout waiting for code")
		}

	case client.TypeAuthorizationStateWaitPassword:
		select {
		case password := <-a.Password:
			log.Printf("[AUTHORIZER] Got password")
			_, err := tdlibClient.CheckAuthenticationPassword(&client.CheckAuthenticationPasswordRequest{
				Password: password,
			})
			if err != nil {
				log.Printf("[AUTHORIZER] Failed to check authentication password: %v", err)
			}
			return err
		case <-time.After(5 * time.Minute):
			return errors.New("[AUTHORIZER] timeout waiting for password")
		}

	case client.TypeAuthorizationStateReady:
		log.Printf("[AUTHORIZER] Authorization ready!")
		return nil

	case client.TypeAuthorizationStateClosed:
		log.Printf("[AUTHORIZER] Authorization closed")
		return nil

	case client.TypeAuthorizationStateWaitEmailAddress:
		log.Printf("[AUTHORIZER] Email authorization not supported")
		return errors.New("[AUTHORIZER] email authorization not supported")

	case client.TypeAuthorizationStateWaitEmailCode:
		log.Printf("[AUTHORIZER] Email code authorization not supported")
		return errors.New("[AUTHORIZER] email code authorization not supported")

	case client.TypeAuthorizationStateWaitOtherDeviceConfirmation:
		log.Printf("[AUTHORIZER] Other device confirmation not supported")
		return errors.New("[AUTHORIZER] other device confirmation not supported")

	case client.TypeAuthorizationStateWaitRegistration:
		log.Printf("[AUTHORIZER] Registration not supported")
		return errors.New("[AUTHORIZER] registration not supported")

	case client.TypeAuthorizationStateLoggingOut:
		log.Printf("[AUTHORIZER] Logging out")
		return nil

	case client.TypeAuthorizationStateClosing:
		log.Printf("[AUTHORIZER] Closing")
		return nil
	}

	log.Printf("[AUTHORIZER] Unhandled authorization state: %s", state.AuthorizationStateType())
	return nil
}

func (a *SimpleAuthorizer) Close() {
	close(a.PhoneNumber)
	close(a.Code)
	close(a.Password)
	close(a.State)
}
