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

func NewAuthStateManager(cleanupInterval, stateTimeout time.Duration) *AuthStateManager {
	mgr := &AuthStateManager{
		clients: make(map[string]*Client),
	}

	return mgr
}

func (m *AuthStateManager) monitorAuthState(sessionID string, client *Client) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

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
			client.authState = stateStr
			client.UpdatedAt = time.Now()
			client.mu.Unlock()

			if stateStr == string(tdlib.AuthorizationStateReadyType) {
				log.Printf("session_id=%s, authorization completed", sessionID)
				return
			}
		}
	}
	// authStateChan := client.tdlibClient.AddEventReceiver(
	// 	&tdlib.UpdateAuthorizationState{},
	// 	func(msg *tdlib.TdMessage) bool {
	// 		return true
	// 	},
	// 	5,
	// )

	// go func() {
	// 	for authUpdate := range authStateChan.Chan {
	// 		if update, ok := authUpdate.(*tdlib.UpdateAuthorizationState); ok {
	// 			log.Println(update.AuthorizationState)
	// 		}
	// 	}
	// }()

	// for {
	// 	currentState, _ := client.tdlibClient.Authorize()
	// 	log.Println("session_id=%s, state=%s", sessionID, string(currentState.GetAuthorizationStateEnum()))
	// 	client.authState = string(currentState.GetAuthorizationStateEnum())
	// }
}

func (m *AuthStateManager) RegisterAuthorizer(sessionID string, client *Client) {
	m.clients[sessionID] = client

	go m.monitorAuthState(sessionID, client)
}

func (m *AuthStateManager) GetAuthState(id string) (string, error) {
	client, exists := m.clients[id]
	if !exists {
		return "", fmt.Errorf("client not found")
	}

	return client.authState, nil
}

func (m *AuthStateManager) SetPhoneNumber(id, phoneNumber string) (tdlib.AuthorizationState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, exists := m.clients[id]
	if !exists {
		return nil, fmt.Errorf("client not found")
	}

	state, err := m.GetAuthState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get state")
	}

	if state != string(tdlib.AuthorizationStateWaitPhoneNumberType) {
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

	state, err := m.GetAuthState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get state")
	}

	if state != string(tdlib.AuthorizationStateWaitCodeType) {
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

	state, err := m.GetAuthState(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get state")
	}

	if state != string(tdlib.AuthorizationStateWaitPasswordType) {
		return nil, fmt.Errorf("unexpected state %s", state)
	}

	newState, err := client.tdlibClient.SendAuthCode(password)
	if err != nil {
		return nil, fmt.Errorf("Error sending password: %v", err)
	}

	client.UpdatedAt = time.Now()

	return newState, nil
}
