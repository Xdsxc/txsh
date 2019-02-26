package shell

import (
	"sync"
)

type SessionManager struct {
	sessions map[string]*Session
	mtx      sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mtx.RLock()
	s, ok := sm.sessions[id]
	sm.mtx.RUnlock()
	if ok {
		if s.Alive() {
			return s, nil
		}

		sm.mtx.Lock()
		delete(sm.sessions, id)
		sm.mtx.Unlock()
	}

	return sm.spawnSession(id)
}

func (sm *SessionManager) StopAll() error {
	sm.mtx.Lock()
	defer sm.mtx.Unlock()
	deleteList := []string{}
	for id, s := range sm.sessions {
		deleteList = append(deleteList, id)
		s.Stop()
	}

	for _, id := range deleteList {
		delete(sm.sessions, id)
	}

	return nil
}

func (sm *SessionManager) StopSessionByID(id string) error {
	sm.mtx.Lock()
	defer sm.mtx.Unlock()
	s, ok := sm.sessions[id]
	if !ok {
		return nil
	}

	if err := s.Stop(); err != nil {
		return err
	}

	delete(sm.sessions, id)
	return nil
}

func (sm *SessionManager) spawnSession(id string) (*Session, error) {
	s, err := NewSession()
	if err != nil {
		return nil, err
	}

	if err := s.Start(); err != nil {
		return nil, err
	}

	sm.mtx.Lock()
	sm.sessions[id] = &s
	sm.mtx.Unlock()

	return &s, nil
}
