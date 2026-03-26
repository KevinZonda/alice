package campaign

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_CreateListAndGetCampaign(t *testing.T) {
	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	store := NewStore(filepath.Join(t.TempDir(), "campaigns.db"))
	store.now = func() time.Time { return base }

	created, err := store.CreateCampaign(Campaign{
		Objective:         "improve speed and quality",
		Session:           SessionRoute{ScopeKey: "chat_id:oc_chat|thread:omt_1"},
		Creator:           Actor{UserID: "ou_user"},
		MaxParallelTrials: 3,
		Tags:              []string{"latency", "quality", "latency"},
	})
	if err != nil {
		t.Fatalf("create campaign failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected campaign id to be assigned")
	}
	if len(created.Tags) != 2 {
		t.Fatalf("expected tags to be deduplicated, got %#v", created.Tags)
	}

	listed, err := store.ListCampaigns((SessionRoute{ScopeKey: "chat_id:oc_chat|thread:omt_2"}).VisibilityKey(), "", 20)
	if err != nil {
		t.Fatalf("list campaigns failed: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected one campaign, got %d", len(listed))
	}

	hidden, err := store.ListCampaigns((SessionRoute{ScopeKey: "chat_id:oc_other|thread:omt_9"}).VisibilityKey(), "", 20)
	if err != nil {
		t.Fatalf("list hidden campaigns failed: %v", err)
	}
	if len(hidden) != 0 {
		t.Fatalf("expected other chat to see no campaigns, got %d", len(hidden))
	}

	loaded, err := store.GetCampaign(created.ID)
	if err != nil {
		t.Fatalf("get campaign failed: %v", err)
	}
	if loaded.Objective != "improve speed and quality" {
		t.Fatalf("unexpected objective: %q", loaded.Objective)
	}
}

func TestStore_DeleteCampaign(t *testing.T) {
	base := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	store := NewStore(filepath.Join(t.TempDir(), "campaigns.db"))
	store.now = func() time.Time { return base }

	created, err := store.CreateCampaign(Campaign{
		Title:             "Delete Me",
		Objective:         "remove the campaign cleanly",
		Session:           SessionRoute{ScopeKey: "chat_id:oc_chat|thread:omt_1"},
		Creator:           Actor{UserID: "ou_user"},
		MaxParallelTrials: 1,
	})
	if err != nil {
		t.Fatalf("create campaign failed: %v", err)
	}

	deleted, err := store.DeleteCampaign(created.ID)
	if err != nil {
		t.Fatalf("delete campaign failed: %v", err)
	}
	if deleted.ID != created.ID {
		t.Fatalf("expected deleted id %q, got %q", created.ID, deleted.ID)
	}
	if _, err := store.GetCampaign(created.ID); err == nil {
		t.Fatal("expected deleted campaign lookup to fail")
	}
}
