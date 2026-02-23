package automation

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type senderStub struct {
	mu              sync.Mutex
	sendTextCalls   int
	lastReceiveType string
	lastReceiveID   string
	lastText        string
}

func (s *senderStub) SendText(_ context.Context, receiveIDType, receiveID, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendTextCalls++
	s.lastReceiveType = receiveIDType
	s.lastReceiveID = receiveID
	s.lastText = text
	return nil
}

func TestEngine_RunSystemTask(t *testing.T) {
	engine := NewEngine(nil, nil)
	engine.tick = 10 * time.Millisecond

	var mu sync.Mutex
	called := 0
	if err := engine.RegisterSystemTask("sys.test", 10*time.Millisecond, func(context.Context) {
		mu.Lock()
		called++
		mu.Unlock()
	}); err != nil {
		t.Fatalf("register system task failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	engine.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if called == 0 {
		t.Fatal("expected system task to be called")
	}
}

func TestEngine_RunUserTask(t *testing.T) {
	base := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	store := NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	store.now = func() time.Time { return base }

	created, err := store.CreateTask(Task{
		Scope:    Scope{Kind: ScopeKindUser, ID: "ou_actor"},
		Route:    Route{ReceiveIDType: "user_id", ReceiveID: "ou_actor"},
		Creator:  Actor{UserID: "ou_actor"},
		Schedule: Schedule{Type: ScheduleTypeInterval, EverySeconds: 1},
		Action:   Action{Type: ActionTypeSendText, Text: "hello", MentionUserIDs: []string{"ou_actor"}},
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}

	sender := &senderStub{}
	engine := NewEngine(store, sender)
	engine.tick = 10 * time.Millisecond
	engine.now = func() time.Time { return base.Add(2 * time.Second) }

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	engine.Run(ctx)

	sender.mu.Lock()
	if sender.sendTextCalls == 0 {
		sender.mu.Unlock()
		t.Fatal("expected user task to send text")
	}
	sender.mu.Unlock()

	stored, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get task failed: %v", err)
	}
	if stored.LastResult == "" {
		t.Fatalf("expected last result to be recorded, task=%+v", stored)
	}
}
