package telegram

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Arman92/go-tdlib"
)

type AuthStateManager struct {
	mu      sync.RWMutex
	clients map[string]*Client
}

func NewAuthStateManager() *AuthStateManager {
	mgr := &AuthStateManager{
		clients: make(map[string]*Client),
	}

	return mgr
}

func (m *AuthStateManager) monitorAuthState(sessionID string, phoneNumber string, client *Client) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	defer close(client.authReady)

	for {
		select {
		case <-ticker.C:
			currentState, err := client.tdlibClient.Authorize()
			if err != nil {
				log.Printf("session_id=%s, error getting state: %v", sessionID, err)
				continue
			}

			stateStr := string(currentState.GetAuthorizationStateEnum())
			log.Printf("session_id=%s, state=%s", sessionID, stateStr)

			client.mu.Lock()
			client.authState = ConvertAuthState(currentState.GetAuthorizationStateEnum())
			client.UpdatedAt = time.Now()
			client.mu.Unlock()

			if stateStr == string(tdlib.AuthorizationStateReadyType) {
				log.Printf("session_id=%s, authorization completed", sessionID)
				return
			}
		}
	}
}

func (m *AuthStateManager) RegisterAuthorizer(sessionID string, phoneNumber string, client *Client) {
	m.clients[sessionID] = client

	go m.monitorAuthState(sessionID, phoneNumber, client)
}

func (m *AuthStateManager) GetAuthState(id string) string {
	client, exists := m.clients[id]
	if !exists {
		return "unknown"
	}

	return client.authState
}

func (m *AuthStateManager) SetPhoneNumber(id, phoneNumber string) (tdlib.AuthorizationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	state := m.GetAuthState(id)

	if state != "wait_phone" {
		return nil, fmt.Errorf("unexpected state %s", state)
	}

	newState, err := client.tdlibClient.SendPhoneNumber(phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("Error sending phone number: %v", err)
	}

	client.UpdatedAt = time.Now()

	return newState, nil
}

func (m *AuthStateManager) SetCode(id, code string) (tdlib.AuthorizationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	state := m.GetAuthState(id)

	if state != "wait_code" {
		return nil, fmt.Errorf("unexpected state %s", state)
	}

	newState, err := client.tdlibClient.SendAuthCode(code)
	if err != nil {
		return nil, fmt.Errorf("Error sending auth code: %v", err)
	}

	client.UpdatedAt = time.Now()

	return newState, nil
}

func (m *AuthStateManager) SetPassword(id, password string) (tdlib.AuthorizationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	state := m.GetAuthState(id)

	if state != "wait_password" {
		return nil, fmt.Errorf("unexpected state %s", state)
	}

	newState, err := client.tdlibClient.SendAuthPassword(password)
	if err != nil {
		return nil, fmt.Errorf("Error sending password: %v", err)
	}

	client.UpdatedAt = time.Now()

	return newState, nil
}

func ConvertAuthState(state tdlib.AuthorizationStateEnum) string {
	switch state {
	case tdlib.AuthorizationStateWaitTdlibParametersType:
		return "inited"
	case tdlib.AuthorizationStateWaitPhoneNumberType:
		return "wait_phone"
	case tdlib.AuthorizationStateWaitCodeType:
		return "wait_code"
	case tdlib.AuthorizationStateWaitPasswordType:
		return "wait_password"
	case tdlib.AuthorizationStateReadyType:
		return "ready"
	default:
		log.Println("Unknown state=%s", string(state))
		return "unknown"
	}
}
