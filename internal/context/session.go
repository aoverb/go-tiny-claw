package context

import (
	"sync"
	"time"

	"github.com/aoverb/go-tiny-claw/internal/schema"
)

type Session struct {
	ID        string
	WorkDir   string
	CreatedAt time.Time
	UpdatedAt time.Time

	TotalPromptTokens     int
	TotalCompletionTokens int
	TotalCostCNY          float64

	history []schema.Message
	mu      sync.Mutex
}

func NewSession(id, workDir string) *Session {
	return &Session{
		ID:        id,
		WorkDir:   workDir,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *Session) RecordUsage(promptTokens int, completionTokens int, costCNY float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalPromptTokens += promptTokens
	s.TotalCompletionTokens += completionTokens
	s.TotalCostCNY += costCNY
}

func (s *Session) Append(msgs ...schema.Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.history = append(s.history, msgs...)
	s.UpdatedAt = time.Now()
}

func (s *Session) GetWorkingMemory(limit int) []schema.Message {
	s.mu.Lock()
	defer s.mu.Unlock()

	historyCount := len(s.history)
	if limit <= 0 || historyCount <= limit {
		retMsg := make([]schema.Message, historyCount)
		copy(retMsg, s.history)
		return retMsg
	}

	retMsg := make([]schema.Message, limit)
	retMsg[0] = s.history[0] // 保留第一条 User Message
	retMsg = append(retMsg, s.history[historyCount-limit+1:]...)

	for len(retMsg) > 1 && retMsg[1].Role == schema.RoleAssistant && retMsg[1].ToolCallID != "" {
		retMsg = retMsg[2:]
	}
	return retMsg
}

type SessionManager struct {
	sessions map[string]*Session
	mu       sync.Mutex
}

var GlobalSessionMgr = &SessionManager{
	sessions: make(map[string]*Session),
}

func (s *SessionManager) GetOrCreate(id, workDir string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.sessions[id]; !ok {
		s.sessions[id] = NewSession(id, workDir)
	}
	return s.sessions[id]
}
