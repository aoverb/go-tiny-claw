package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type traceKey struct{}

type Span struct {
	Name       string
	StartTime  time.Time
	EndTime    time.Time
	Duration   int64
	Attributes map[string]any
	Children   []*Span

	mu sync.Mutex
}

func StartSpan(ctx context.Context, name string) (context.Context, *Span) {
	span := &Span{
		Name:       name,
		StartTime:  time.Now(),
		Attributes: make(map[string]any),
	}

	if parentSpan, exist := ctx.Value(traceKey{}).(*Span); exist {
		parentSpan.mu.Lock()
		defer parentSpan.mu.Unlock()
		parentSpan.Children = append(parentSpan.Children, span)
	}

	newCtx := context.WithValue(ctx, traceKey{}, span)
	return newCtx, span
}

func (s *Span) EndSpan() {
	s.EndTime = time.Now()
	s.Duration = s.EndTime.Sub(s.StartTime).Milliseconds()
}

func (s *Span) AddAttribute(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Attributes[key] = value
}

func ExportTraceToFile(rootSpan *Span, workDir string, sessionID string) error {
	traceDir := filepath.Join(workDir, ".claw", "trace")
	os.MkdirAll(traceDir, 0755)

	data, err := json.MarshalIndent(rootSpan, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(traceDir, fmt.Sprintf("trace_%s_%d.json", sessionID, time.Now().Unix())), data, 0644)
}
