package automation

import (
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_CreateListPatchClaim(t *testing.T) {
	base := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	store := NewStore(filepath.Join(t.TempDir(), "automation_state.json"))
	store.now = func() time.Time { return base }

	created, err := store.CreateTask(Task{
		Title: "每分钟提醒",
		Scope: Scope{Kind: ScopeKindUser, ID: "ou_actor"},
		Route: Route{ReceiveIDType: "user_id", ReceiveID: "ou_actor"},
		Creator: Actor{
			UserID: "ou_actor",
			Name:   "Alice",
		},
		Schedule: Schedule{Type: ScheduleTypeInterval, EverySeconds: 60},
		Action:   Action{Type: ActionTypeSendText, Text: "ping", MentionUserIDs: []string{"ou_actor"}},
	})
	if err != nil {
		t.Fatalf("create task failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated task id")
	}

	list, err := store.ListTasks(Scope{Kind: ScopeKindUser, ID: "ou_actor"}, "", 20)
	if err != nil {
		t.Fatalf("list task failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 task, got %d", len(list))
	}

	store.now = func() time.Time { return base.Add(2 * time.Minute) }
	claimed, err := store.ClaimDueTasks(base.Add(2*time.Minute), 10)
	if err != nil {
		t.Fatalf("claim due tasks failed: %v", err)
	}
	if len(claimed) != 1 || claimed[0].ID != created.ID {
		t.Fatalf("unexpected claimed tasks: %+v", claimed)
	}

	if err := store.RecordTaskResult(created.ID, base.Add(2*time.Minute), errors.New("x")); err != nil {
		t.Fatalf("record result failed: %v", err)
	}
	updated, err := store.GetTask(created.ID)
	if err != nil {
		t.Fatalf("get updated task failed: %v", err)
	}
	if updated.ConsecutiveFailures != 1 {
		t.Fatalf("unexpected failure count: %d", updated.ConsecutiveFailures)
	}

	patched, err := store.PatchTask(created.ID, func(task *Task) error {
		task.Status = TaskStatusPaused
		return nil
	})
	if err != nil {
		t.Fatalf("patch task failed: %v", err)
	}
	if patched.Status != TaskStatusPaused {
		t.Fatalf("unexpected patched status: %s", patched.Status)
	}
}
