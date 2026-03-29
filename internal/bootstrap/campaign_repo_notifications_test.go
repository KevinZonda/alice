package bootstrap

import (
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
