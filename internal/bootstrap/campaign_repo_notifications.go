package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Alice-space/alice/internal/campaign"
	"github.com/Alice-space/alice/internal/campaignrepo"
	"github.com/Alice-space/alice/internal/connector"
	"github.com/Alice-space/alice/internal/logging"
)

func (b *connectorRuntimeBuilder) sendCampaignNotifications(item campaign.Campaign, events []campaignrepo.ReconcileEvent) {
	if b == nil || b.sender == nil || len(events) == 0 {
		return
	}
	target, ok := automationTargetFromCampaign(item)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, event := range events {
		title := campaignEventCardTitle(item.Title, item.ID, event)
		cardContent, err := buildCampaignEventCard(title, event)
		if err != nil {
			logging.Warnf("build campaign event card failed campaign=%s kind=%s: %v", item.ID, event.Kind, err)
			continue
		}
		if err := b.sender.SendCard(ctx, target.Route.ReceiveIDType, target.Route.ReceiveID, cardContent); err != nil {
			// Fallback to text
			text := fmt.Sprintf("**%s**\n%s", title, event.Detail)
			if sendErr := b.sender.SendText(ctx, target.Route.ReceiveIDType, target.Route.ReceiveID, text); sendErr != nil {
				logging.Warnf("send campaign notification failed campaign=%s kind=%s: %v", item.ID, event.Kind, sendErr)
			}
		}
	}
}

func buildCampaignEventCard(title string, event campaignrepo.ReconcileEvent) (string, error) {
	colorTemplate := severityToCardTemplate(event.Severity)
	title = strings.TrimSpace(title)
	detail := strings.TrimSpace(event.Detail)
	if detail == "" {
		detail = " "
	}
	elements := []any{
		campaignEventCardMarkdown(detail),
	}
	if taskID := strings.TrimSpace(event.TaskID); taskID != "" {
		elements = []any{
			campaignEventCardMarkdown(fmt.Sprintf("**任务**: %s", taskID)),
			campaignEventCardMarkdown(detail),
		}
	}
	if action := campaignEventActionElement(event); action != nil {
		elements = append(elements, action)
	}
	card := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"enable_forward": true,
			"update_multi":   true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": title,
			},
			"template": colorTemplate,
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
	raw, err := json.Marshal(card)
	if err != nil {
		return "", fmt.Errorf("marshal campaign event card failed: %w", err)
	}
	return string(raw), nil
}

func campaignEventCardTitle(campaignTitle, campaignID string, event campaignrepo.ReconcileEvent) string {
	base := strings.TrimSpace(event.Title)
	if base == "" {
		base = string(event.Kind)
	}
	campaignLabel := strings.TrimSpace(campaignTitle)
	if campaignLabel == "" {
		campaignLabel = strings.TrimSpace(campaignID)
	}
	if campaignLabel == "" {
		return base
	}
	return fmt.Sprintf("%s · %s", campaignLabel, base)
}

func severityToCardTemplate(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "success":
		return "green"
	case "warning":
		return "orange"
	case "error":
		return "red"
	default:
		return "blue"
	}
}

func campaignEventCardMarkdown(content string) map[string]any {
	return map[string]any{
		"tag":     "markdown",
		"content": content,
	}
}

func campaignEventActionElement(event campaignrepo.ReconcileEvent) map[string]any {
	if event.Kind != campaignrepo.EventHumanApprovalNeeded || strings.TrimSpace(event.CampaignID) == "" {
		return nil
	}
	value := map[string]any{
		"alice_action": connector.CardActionKindCampaignPlanApproval,
		"campaign_id":  strings.TrimSpace(event.CampaignID),
		"plan_round":   event.PlanRound,
	}
	approveValue := cloneCardActionValue(value)
	approveValue["decision"] = connector.CardActionDecisionApprove
	rejectValue := cloneCardActionValue(value)
	rejectValue["decision"] = connector.CardActionDecisionReject
	return map[string]any{
		"tag":    "action",
		"layout": "bisected",
		"actions": []any{
			campaignEventActionButton("批准", "primary", approveValue,
				"确认批准当前计划？", "批准后会把计划切到执行阶段，并继续派发任务。"),
			campaignEventActionButton("不批准", "danger", rejectValue,
				"确认不批准当前计划？", "不批准后会退回 planning，并进入下一轮规划。"),
		},
	}
}

func campaignEventActionButton(label, buttonType string, value map[string]any, confirmTitle, confirmText string) map[string]any {
	button := map[string]any{
		"tag":   "button",
		"type":  strings.TrimSpace(buttonType),
		"text":  campaignEventPlainText(strings.TrimSpace(label)),
		"value": value,
	}
	if strings.TrimSpace(confirmTitle) != "" || strings.TrimSpace(confirmText) != "" {
		button["confirm"] = map[string]any{
			"title": campaignEventPlainText(strings.TrimSpace(confirmTitle)),
			"text":  campaignEventPlainText(strings.TrimSpace(confirmText)),
		}
	}
	return button
}

func campaignEventPlainText(content string) map[string]any {
	return map[string]any{
		"tag":     "plain_text",
		"content": strings.TrimSpace(content),
	}
}

func cloneCardActionValue(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
