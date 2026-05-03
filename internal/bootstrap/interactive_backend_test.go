package bootstrap

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	agentbridge "github.com/Alice-space/agentbridge"
)

func TestInteractiveProviderBackendForwardsAssistantTextAndDropsToolUse(t *testing.T) {
	for _, provider := range []string{
		agentbridge.ProviderCodex,
		agentbridge.ProviderClaude,
		agentbridge.ProviderOpenCode,
	} {
		t.Run(provider, func(t *testing.T) {
			sessionKey := "session-" + provider
			driver := &interactiveBackendTestDriver{
				provider: provider,
				events:   make(chan agentbridge.TurnEvent, 8),
			}
			session := agentbridge.NewInteractiveSession(driver)
			defer session.Close()

			backend := &interactiveProviderBackend{
				provider: provider,
				sessions: map[string]*agentbridge.InteractiveSession{
					sessionKey: session,
				},
				runMu: map[string]*sync.Mutex{},
			}

			var progress []string
			var raw []string
			result, err := backend.runInteractive(context.Background(), sessionKey, agentbridge.RunRequest{
				UserText: "hello",
				OnProgress: func(step string) {
					progress = append(progress, step)
				},
				OnRawEvent: func(event agentbridge.RawEvent) {
					raw = append(raw, strings.TrimSpace(event.Kind)+":"+strings.TrimSpace(event.Detail))
				},
			})
			if err != nil {
				t.Fatalf("runInteractive returned error: %v", err)
			}
			if result.Reply != provider+" middle" {
				t.Fatalf("reply = %q, want %q", result.Reply, provider+" middle")
			}
			if len(progress) != 1 || progress[0] != provider+" middle" {
				t.Fatalf("progress = %#v, want only assistant text", progress)
			}
			wantRaw := []string{
				"user_text:hello",
				"tool_use:tool_use tool=`bash` command=`pwd`",
				"turn_completed:",
			}
			if strings.Join(raw, "\n") != strings.Join(wantRaw, "\n") {
				t.Fatalf("raw events = %#v, want %#v", raw, wantRaw)
			}
		})
	}
}

func TestInteractiveProviderBackendClosesIdleSession(t *testing.T) {
	sessionKey := "session-opencode"
	driver := &interactiveBackendTestDriver{
		provider: agentbridge.ProviderOpenCode,
		events:   make(chan agentbridge.TurnEvent, 8),
	}
	session := agentbridge.NewInteractiveSession(driver)

	backend := &interactiveProviderBackend{
		provider: agentbridge.ProviderOpenCode,
		idleTTL:  10 * time.Millisecond,
		sessions: map[string]*agentbridge.InteractiveSession{
			sessionKey: session,
		},
		runMu: map[string]*sync.Mutex{},
	}

	result, err := backend.runInteractive(context.Background(), sessionKey, agentbridge.RunRequest{UserText: "hello"})
	if err != nil {
		t.Fatalf("runInteractive returned error: %v", err)
	}
	if result.Reply != agentbridge.ProviderOpenCode+" middle" {
		t.Fatalf("reply = %q", result.Reply)
	}

	waitForBootstrap(t, time.Second, func() bool {
		return backend.session(sessionKey) == nil && driver.closeCount() == 1
	}, "idle interactive session should be closed")
}

type interactiveBackendTestDriver struct {
	provider  string
	events    chan agentbridge.TurnEvent
	closeOnce sync.Once
	closeMu   sync.Mutex
	closes    int
}

func (d *interactiveBackendTestDriver) SteerMode() agentbridge.SteerMode {
	return agentbridge.SteerModeNative
}

func (d *interactiveBackendTestDriver) StartTurn(_ context.Context, req agentbridge.RunRequest) (agentbridge.TurnRef, error) {
	turn := agentbridge.TurnRef{ThreadID: "thread-1", TurnID: "turn-1"}
	go func() {
		d.events <- agentbridge.TurnEvent{
			Provider: d.provider,
			ThreadID: turn.ThreadID,
			TurnID:   turn.TurnID,
			Kind:     agentbridge.TurnEventUserText,
			Text:     "hello",
		}
		d.events <- agentbridge.TurnEvent{
			Provider: d.provider,
			ThreadID: turn.ThreadID,
			TurnID:   turn.TurnID,
			Kind:     agentbridge.TurnEventToolUse,
			Text:     "tool_use tool=`bash` command=`pwd`",
		}
		d.events <- agentbridge.TurnEvent{
			Provider: d.provider,
			ThreadID: turn.ThreadID,
			TurnID:   turn.TurnID,
			Kind:     agentbridge.TurnEventAssistantText,
			Text:     d.provider + " middle",
		}
		d.events <- agentbridge.TurnEvent{
			Provider: d.provider,
			ThreadID: turn.ThreadID,
			TurnID:   turn.TurnID,
			Kind:     agentbridge.TurnEventCompleted,
		}
	}()
	_ = req
	return turn, nil
}

func (d *interactiveBackendTestDriver) SteerTurn(context.Context, agentbridge.TurnRef, agentbridge.RunRequest) error {
	return nil
}

func (d *interactiveBackendTestDriver) InterruptTurn(context.Context, agentbridge.TurnRef) error {
	return nil
}

func (d *interactiveBackendTestDriver) Events() <-chan agentbridge.TurnEvent {
	return d.events
}

func (d *interactiveBackendTestDriver) Close() error {
	d.closeOnce.Do(func() {
		d.closeMu.Lock()
		d.closes++
		d.closeMu.Unlock()
		close(d.events)
	})
	return nil
}

func (d *interactiveBackendTestDriver) closeCount() int {
	d.closeMu.Lock()
	defer d.closeMu.Unlock()
	return d.closes
}

func waitForBootstrap(t *testing.T, timeout time.Duration, ok func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}
