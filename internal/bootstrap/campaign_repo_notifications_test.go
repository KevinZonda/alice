package bootstrap

import (
	"encoding/json"
	"testing"

	"github.com/Alice-space/alice/internal/campaignrepo"
)

func TestCampaignEventCardTitle_UsesCampaignName(t *testing.T) {
	title := campaignEventCardTitle("Demo Campaign", "camp_demo", campaignrepo.ReconcileEvent{
		Kind:  campaignrepo.EventTaskDispatched,
		Title: "任务已派发执行",
	})
	if title != "Demo Campaign · 任务已派发执行" {
		t.Fatalf("unexpected campaign event title: %q", title)
	}
}

func TestBuildCampaignEventCard_HumanApprovalAddsButtons(t *testing.T) {
	cardContent, err := buildCampaignEventCard("Demo Campaign · 方案评审通过", campaignrepo.ReconcileEvent{
		Kind:       campaignrepo.EventHumanApprovalNeeded,
		CampaignID: "camp_demo",
		PlanRound:  3,
		Title:      "方案评审通过",
		Detail:     "等待人工批准",
		Severity:   "success",
	})
	if err != nil {
		t.Fatalf("build campaign event card failed: %v", err)
	}

	var card struct {
		Body struct {
			Elements []map[string]any `json:"elements"`
		} `json:"body"`
	}
	if err := json.Unmarshal([]byte(cardContent), &card); err != nil {
		t.Fatalf("unmarshal card failed: %v", err)
	}
	if len(card.Body.Elements) != 2 {
		t.Fatalf("expected markdown + action elements, got %d", len(card.Body.Elements))
	}
	action := card.Body.Elements[1]
	if got := action["tag"]; got != "action" {
		t.Fatalf("expected action element, got %#v", got)
	}
	actions, ok := action["actions"].([]any)
	if !ok || len(actions) != 2 {
		t.Fatalf("expected 2 action buttons, got %#v", action["actions"])
	}
	first, ok := actions[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected first action payload: %#v", actions[0])
	}
	value, ok := first["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected first action value map, got %#v", first["value"])
	}
	if value["alice_action"] != "campaign_plan_approval" {
		t.Fatalf("unexpected alice_action: %#v", value["alice_action"])
	}
	if value["campaign_id"] != "camp_demo" {
		t.Fatalf("unexpected campaign id: %#v", value["campaign_id"])
	}
	if value["decision"] != "approve" {
		t.Fatalf("unexpected first decision: %#v", value["decision"])
	}
	if value["plan_round"] != float64(3) {
		t.Fatalf("unexpected plan round: %#v", value["plan_round"])
	}
}
