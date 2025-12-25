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
	go func() {
		select {
		case a.State <- state:
		default:
		}
	}()

	switch state.AuthorizationStateType() {
	case client.TypeAuthorizationStateWaitTdlibParameters:
		_, err := tdlibClient.SetTdlibParameters(&client.SetTdlibParametersRequest{
			UseFileDatabase:        true,
			UseChatInfoDatabase:    true,
			UseMessageDatabase:     true,
			UseSecretChats:         false,
			SystemLanguageCode:           "en",
			DeviceModel:                  "Server",
			SystemVersion:                "1.0",
			ApplicationVersion:           "1.0",   
			ApiId:                  a.cfg.APIID,
			ApiHash:                a.cfg.APIHash,
		})
		if err != nil {
			log.Printf("Failed to set TDLib parameters: %v", err)
			return err
		}
		log.Printf("TDLib parameters set successfully")
		return nil

	case client.TypeAuthorizationStateWaitPhoneNumber:
		log.Printf("Waiting for phone number")
		select {
		case phone := <-a.PhoneNumber:
			log.Printf("Got phone number: %s", phone)
			_, err := tdlibClient.SetAuthenticationPhoneNumber(&client.SetAuthenticationPhoneNumberRequest{
				PhoneNumber: phone,
				Settings: &client.PhoneNumberAuthenticationSettings{
					AllowFlashCall:       false,
					IsCurrentPhoneNumber: false,
					AllowSmsRetrieverApi: false,
				},
			})
			if err != nil {
				log.Printf("Failed to set phone number: %v", err)
			}
			return err
		case <-time.After(5 * time.Minute):
			log.Printf("Timeout waiting for phone number")
			return errors.New("timeout waiting for phone number")
		}

	case client.TypeAuthorizationStateWaitCode:
		log.Printf("Waiting for code")
		select {
		case code := <-a.Code:
			log.Printf("Got code: %s", code)
			_, err := tdlibClient.CheckAuthenticationCode(&client.CheckAuthenticationCodeRequest{
				Code: code,
			})
			if err != nil {
				log.Printf("Failed to check authentication code: %v", err)
			}
			return err
		case <-time.After(5 * time.Minute):
			log.Printf("Timeout waiting for code")
			return errors.New("timeout waiting for code")
		}

	case client.TypeAuthorizationStateWaitPassword:
		log.Printf("Waiting for password")
		select {
		case password := <-a.Password:
			log.Printf("Got password")
			_, err := tdlibClient.CheckAuthenticationPassword(&client.CheckAuthenticationPasswordRequest{
				Password: password,
			})
			if err != nil {
				log.Printf("Failed to check authentication password: %v", err)
			}
			return err
		case <-time.After(5 * time.Minute):
			log.Printf("Timeout waiting for password")
			return errors.New("timeout waiting for password")
		}

	case client.TypeAuthorizationStateReady:
		log.Printf("Authorization ready!")
		return nil

	case client.TypeAuthorizationStateClosed:
		log.Printf("Authorization closed")
		return nil
		
	case client.TypeAuthorizationStateWaitEmailAddress:
		log.Printf("Email authorization not supported")
		return errors.New("email authorization not supported")
		
	case client.TypeAuthorizationStateWaitEmailCode:
		log.Printf("Email code authorization not supported")
		return errors.New("email code authorization not supported")
		
	case client.TypeAuthorizationStateWaitOtherDeviceConfirmation:
		log.Printf("Other device confirmation not supported")
		return errors.New("other device confirmation not supported")
		
	case client.TypeAuthorizationStateWaitRegistration:
		log.Printf("Registration not supported")
		return errors.New("registration not supported")
		
	case client.TypeAuthorizationStateLoggingOut:
		log.Printf("Logging out")
		return nil
		
	case client.TypeAuthorizationStateClosing:
		log.Printf("Closing")
		return nil
	}

	log.Printf("Unhandled authorization state: %s", state.AuthorizationStateType())
	return nil
}

func (a *SimpleAuthorizer) Close() {
	close(a.PhoneNumber)
	close(a.Code)
	close(a.Password)
	close(a.State)
}