package telegram

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

type AuthState struct {
	ID          string    `json:"id"`
	State       string    `json:"state"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	Code        string    `json:"code,omitempty"`
	Password    string    `json:"password,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AuthStateManager struct {
	mu              sync.RWMutex
	states          map[string]*AuthState
	authorizers     map[string]*SimpleAuthorizer
	cleanupInterval time.Duration
	stateTimeout    time.Duration
}

func NewAuthStateManager(cleanupInterval, stateTimeout time.Duration) *AuthStateManager {
	mgr := &AuthStateManager{
		states:          make(map[string]*AuthState),
		authorizers:     make(map[string]*SimpleAuthorizer),
		cleanupInterval: cleanupInterval,
		stateTimeout:    stateTimeout,
	}

	go mgr.cleanupExpiredStates()

	return mgr
}

func (m *AuthStateManager) CreateAuthState() *AuthState {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID()
	state := &AuthState{
		ID:        id,
		State:     "waiting_for_phone",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.states[id] = state
	log.Printf("Created auth state: %s", id)
	return state
}

func (m *AuthStateManager) GetAuthState(id string) (*AuthState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[id]
	return state, exists
}

func (m *AuthStateManager) UpdateAuthState(id string, updateFn func(*AuthState)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[id]
	if !exists {
		return false
	}

	updateFn(state)
	state.UpdatedAt = time.Now()
	return true
}

func (m *AuthStateManager) RegisterAuthorizer(sessionID string, authorizer *SimpleAuthorizer) {	
	m.mu.Lock()
	defer m.mu.Unlock()
	m.authorizers[sessionID] = authorizer
	
	go m.monitorAuthState(sessionID, authorizer)
}

func (m *AuthStateManager) monitorAuthState(sessionID string, authorizer *SimpleAuthorizer) {
	for state := range authorizer.State {
		log.Printf("Session %s: Received auth state: %T", sessionID, state)
		
		if err := m.UpdateState(sessionID, state); err != nil {
			log.Printf("Failed to update auth state: %v", err)
		}
	}
	log.Printf("State monitoring stopped for session %s", sessionID)
}

func (m *AuthStateManager) SetPhoneNumber(id, phoneNumber string) error {	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	state, exists := m.states[id]
	if !exists {
		return fmt.Errorf("auth state not found")
	}
	
	if state.State != "waiting_for_phone" {
		return fmt.Errorf("wrong auth state: %s, expected waiting_for_phone", state.State)
	}
	
	state.PhoneNumber = phoneNumber
	state.State = "waiting_for_code"
	state.UpdatedAt = time.Now()

	authorizer, exists := m.authorizers[id]
	if !exists {
		return fmt.Errorf("authorizer not found")
	}

	select {
	case authorizer.PhoneNumber <- phoneNumber:
		return nil
	case <-time.After(10 * time.Second):
		state.State = "waiting_for_phone"
		return fmt.Errorf("timeout sending phone number")
	}
}

func (m *AuthStateManager) SetCode(id, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	state, exists := m.states[id]
	if !exists {
		return fmt.Errorf("auth state not found")
	}

	if state.State != "waiting_for_code" {
		return fmt.Errorf("wrong auth state: %s, expected waiting_for_code", state.State)
	}
	
	state.Code = code
	state.State = "processing"
	state.UpdatedAt = time.Now()

	authorizer, exists := m.authorizers[id]
	if !exists {
		return fmt.Errorf("authorizer not found")
	}

	select {
	case authorizer.Code <- code:
		log.Printf("Code sent for auth state %s", id)
		return nil
	case <-time.After(10 * time.Second):
		state.State = "waiting_for_code"
		return fmt.Errorf("timeout sending code")
	}
}

func (m *AuthStateManager) SetPassword(id, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	state, exists := m.states[id]
	if !exists {
		return fmt.Errorf("auth state not found")
	}

	if state.State != "waiting_for_password" {
		return fmt.Errorf("wrong auth state: %s, expected waiting_for_password", state.State)
	}
	
	state.Password = password
	state.State = "processing"
	state.UpdatedAt = time.Now()

	authorizer, exists := m.authorizers[id]
	if !exists {
		return fmt.Errorf("authorizer not found")
	}

	select {
	case authorizer.Password <- password:
		log.Printf("Password sent for auth state %s", id)
		return nil
	case <-time.After(10 * time.Second):
		state.State = "waiting_for_password"
		return fmt.Errorf("timeout sending password")
	}
}

func (m *AuthStateManager) WaitForReady(id string, timeout time.Duration) (bool, error) {
	start := time.Now()
	
	for time.Since(start) < timeout {
		state, exists := m.GetAuthState(id)
		if !exists {
			return false, fmt.Errorf("auth state not found")
		}
		
		if state.State == "ready" {
			return true, nil
		}
		
		time.Sleep(500 * time.Millisecond)
	}
	
	return false, fmt.Errorf("timeout waiting for authorization")
}

func (m *AuthStateManager) UpdateState(id string, authState client.AuthorizationState) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	state, exists := m.states[id]
	if !exists {
		return fmt.Errorf("auth state not found")
	}

	switch authState.(type) {
	case *client.AuthorizationStateWaitPhoneNumber:
		state.State = "waiting_for_phone"
		log.Printf("Auth state %s: waiting for phone number", id)
		
	case *client.AuthorizationStateWaitCode:
		state.State = "waiting_for_code"
		log.Printf("Auth state %s: waiting for code", id)
		
	case *client.AuthorizationStateWaitPassword:
		state.State = "waiting_for_password"
		log.Printf("Auth state %s: waiting for password", id)
		
	case *client.AuthorizationStateReady:
		state.State = "ready"
		log.Printf("Auth state %s: authorization ready", id)
		
	case *client.AuthorizationStateClosed:
		state.State = "closed"
		log.Printf("Auth state %s: closed", id)
		
	default:
		log.Printf("Auth state %s: received unknown state type: %T", id, authState)
	}
	
	state.UpdatedAt = time.Now()
	return nil
}

func (m *AuthStateManager) cleanupExpiredStates() {
	ticker := time.NewTicker(m.cleanupInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for id, state := range m.states {
			if now.Sub(state.UpdatedAt) > m.stateTimeout {
				if authorizer, exists := m.authorizers[id]; exists {
					close(authorizer.PhoneNumber)
					close(authorizer.Code)
					close(authorizer.Password)
					close(authorizer.State)
					delete(m.authorizers, id)
				}
				delete(m.states, id)
				log.Printf("Cleaned up expired auth state: %s", id)
			}
		}
		m.mu.Unlock()
	}
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}