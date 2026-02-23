package connector

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func normalizeReasoning(step string) string {
	step = strings.TrimSpace(step)
	step = strings.Trim(step, "*")
	step = strings.TrimSpace(step)
	if step == "" {
		return ""
	}
	return clipText(step, 600)
}

func buildProgressCardContent(thinkingText, answerText string, failed bool, interrupted bool, elapsed time.Duration) string {
	status := "思考中"
	if interrupted {
		status = "已中断"
	} else if failed {
		status = "失败"
	} else if strings.TrimSpace(answerText) != "" {
		status = "已完成"
	}

	thinking := clipText(strings.TrimSpace(thinkingText), 4000)
	answer := clipText(strings.TrimSpace(answerText), 4000)
	if thinking == "" {
		thinking = "（暂无）"
	}
	durationLabel := "已思考：" + formatElapsed(elapsed)
	if interrupted || failed || strings.TrimSpace(answer) != "" {
		durationLabel = "总耗时：" + formatElapsed(elapsed)
	}

	elements := []any{
		cardMarkdown("**状态**：" + status + "（" + durationLabel + "）"),
		cardMarkdown("**Codex 思考**\n" + thinking),
	}
	if strings.TrimSpace(answer) != "" {
		elements = append(elements, cardMarkdown("**回复**\n"+answer))
	}

	card := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"enable_forward": true,
			"update_multi":   true,
		},
		"body": map[string]any{
			"elements": elements,
		},
	}
	raw, _ := json.Marshal(card)
	return string(raw)
}

func buildReplyCardContent(markdown string) string {
	reply := strings.TrimSpace(markdown)
	if reply == "" {
		reply = " "
	}
	card := map[string]any{
		"schema": "2.0",
		"config": map[string]any{
			"enable_forward": true,
			"update_multi":   true,
		},
		"body": map[string]any{
			"elements": []any{
				cardMarkdown("**回复**\n" + reply),
			},
		},
	}
	raw, _ := json.Marshal(card)
	return string(raw)
}

func cardMarkdown(content string) map[string]any {
	return map[string]any{
		"tag":     "markdown",
		"content": content,
	}
}

func clipText(text string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func formatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		minutes := int(d / time.Minute)
		seconds := int((d % time.Minute) / time.Second)
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}

	hours := int(d / time.Hour)
	minutes := int((d % time.Hour) / time.Minute)
	seconds := int((d % time.Minute) / time.Second)
	return fmt.Sprintf("%dh%02dm%02ds", hours, minutes, seconds)
}
