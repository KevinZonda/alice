package campaignrepo

import (
	"fmt"
	"strings"
)

func campaignNotificationLabel(title, id string) string {
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		return trimmed
	}
	if trimmed := strings.TrimSpace(id); trimmed != "" {
		return trimmed
	}
	return "campaign"
}

func dispatchTaskTitle(campaignTitle, campaignID string, kind DispatchKind, taskID string, round int) string {
	campaignLabel := campaignNotificationLabel(campaignTitle, campaignID)
	taskLabel := strings.TrimSpace(taskID)
	if taskLabel == "" {
		taskLabel = "task"
	}

	switch kind {
	case DispatchKindPlanner:
		return fmt.Sprintf("%s · 规划 · 第 %d 轮", campaignLabel, maxInt(round, 1))
	case DispatchKindPlannerReviewer:
		return fmt.Sprintf("%s · 规划评审 · 第 %d 轮", campaignLabel, maxInt(round, 1))
	case DispatchKindReviewer:
		return fmt.Sprintf("%s · %s · 评审 · 第 %d 轮", campaignLabel, taskLabel, maxInt(round, 1))
	default:
		return fmt.Sprintf("%s · %s · 执行 · 第 %d 轮", campaignLabel, taskLabel, maxInt(round, 1))
	}
}

func wakeNotificationTitle(campaignTitle, campaignID, taskID string) string {
	campaignLabel := campaignNotificationLabel(campaignTitle, campaignID)
	taskLabel := strings.TrimSpace(taskID)
	if taskLabel == "" {
		taskLabel = "task"
	}
	return fmt.Sprintf("%s · %s · 唤醒", campaignLabel, taskLabel)
}
