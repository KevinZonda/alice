package connector

import (
	"encoding/json"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type builtinStatusBotUsage struct {
	BotID   string
	BotName string
	Usage   sessionUsageStats
}

func addSessionUsageStats(dst *sessionUsageStats, src sessionUsageStats) {
	if dst == nil || !src.HasUsage() {
		return
	}
	dst.InputTokens += src.InputTokens
	dst.CachedInputTokens += src.CachedInputTokens
	dst.OutputTokens += src.OutputTokens
	dst.Turns += src.Turns
	if dst.UpdatedAt.Before(src.UpdatedAt) {
		dst.UpdatedAt = src.UpdatedAt
	}
}

func normalizeStatusBotLabel(botID, botName string) (string, string) {
	botID = strings.TrimSpace(botID)
	botName = strings.TrimSpace(botName)
	if botName == "" {
		botName = botID
	}
	if botID == "" {
		botID = botName
	}
	return botID, botName
}

func (p *Processor) builtinStatusUsage(job Job) (sessionUsageStats, []builtinStatusBotUsage, error) {
	scopeKey := builtinStatusVisibilityKey(job)
	if scopeKey == "" {
		return sessionUsageStats{}, nil, os.ErrInvalid
	}

	snapshot := p.runtimeSnapshot()
	byBot := make(map[string]builtinStatusBotUsage)

	currentBotID, currentBotName := normalizeStatusBotLabel(snapshot.statusBotID, snapshot.statusBotName)
	currentUsage := p.collectInMemoryUsageForScope(scopeKey)
	if currentBotID != "" || currentUsage.HasUsage() {
		byBot[currentBotID] = builtinStatusBotUsage{
			BotID:   currentBotID,
			BotName: currentBotName,
			Usage:   currentUsage,
		}
	}

	var firstErr error
	for _, source := range snapshot.statusUsagePeers {
		sourceBotID, sourceBotName := normalizeStatusBotLabel(source.BotID, source.BotName)
		if currentBotID != "" && sourceBotID == currentBotID {
			continue
		}
		usage, err := loadUsageFromSessionState(source.SessionStatePath, scopeKey)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		existing := byBot[sourceBotID]
		existing.BotID = sourceBotID
		existing.BotName = sourceBotName
		addSessionUsageStats(&existing.Usage, usage)
		byBot[sourceBotID] = existing
	}

	total := sessionUsageStats{}
	items := make([]builtinStatusBotUsage, 0, len(byBot))
	for _, item := range byBot {
		addSessionUsageStats(&total, item.Usage)
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		left := items[i]
		right := items[j]
		if left.Usage.TotalTokens() != right.Usage.TotalTokens() {
			return left.Usage.TotalTokens() > right.Usage.TotalTokens()
		}
		return left.BotName < right.BotName
	})
	return total, items, firstErr
}

func (p *Processor) collectInMemoryUsageForScope(scopeKey string) sessionUsageStats {
	scopeKey = strings.TrimSpace(scopeKey)
	if scopeKey == "" {
		return sessionUsageStats{}
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	total := sessionUsageStats{}
	for key, state := range p.sessions {
		stateScopeKey := strings.TrimSpace(state.ScopeKey)
		if stateScopeKey == "" {
			stateScopeKey = scopeKeyFromSessionKey(key)
		}
		if stateScopeKey != scopeKey {
			continue
		}
		addSessionUsageStats(&total, state.Usage)
	}
	return total
}

func loadUsageFromSessionState(path, scopeKey string) (sessionUsageStats, error) {
	path = strings.TrimSpace(path)
	scopeKey = strings.TrimSpace(scopeKey)
	if path == "" || scopeKey == "" {
		return sessionUsageStats{}, os.ErrInvalid
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sessionUsageStats{}, nil
		}
		return sessionUsageStats{}, err
	}

	var snapshot sessionStateSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return sessionUsageStats{}, err
	}

	total := sessionUsageStats{}
	for key, state := range snapshot.Sessions {
		stateScopeKey := strings.TrimSpace(state.ScopeKey)
		if stateScopeKey == "" {
			stateScopeKey = scopeKeyFromSessionKey(key)
		}
		if stateScopeKey != scopeKey {
			continue
		}
		addSessionUsageStats(&total, state.Usage)
	}
	return total, nil
}

func formatBuiltinStatusUsageLine(item builtinStatusBotUsage) string {
	botLabel := strings.TrimSpace(item.BotName)
	if botLabel == "" {
		botLabel = strings.TrimSpace(item.BotID)
	}
	if botLabel == "" {
		botLabel = "unknown"
	}

	parts := []string{
		"- `" + sanitizeInlineCode(botLabel) + "`",
		"total `" + formatBuiltinStatusTokenCount(item.Usage.TotalTokens()) + "`",
		"input `" + formatBuiltinStatusTokenCount(item.Usage.InputTokens) + "`",
		"cached `" + formatBuiltinStatusTokenCount(item.Usage.CachedInputTokens) + "`",
		"output `" + formatBuiltinStatusTokenCount(item.Usage.OutputTokens) + "`",
	}
	if item.Usage.Turns > 0 {
		parts = append(parts, "turns `"+formatBuiltinStatusTokenCount(item.Usage.Turns)+"`")
	}
	if updatedAt := formatBuiltinStatusTime(item.Usage.UpdatedAt); updatedAt != "" {
		parts = append(parts, "updated `"+updatedAt+"`")
	}
	return strings.Join(parts, " | ")
}

func formatBuiltinStatusTokenCount(value int64) string {
	if value < 0 {
		value = 0
	}
	raw := []byte(strconv.FormatInt(value, 10))
	n := len(raw)
	if n <= 3 {
		return string(raw)
	}

	out := make([]byte, 0, n+(n-1)/3)
	prefix := n % 3
	if prefix == 0 {
		prefix = 3
	}
	out = append(out, raw[:prefix]...)
	for idx := prefix; idx < n; idx += 3 {
		out = append(out, ',')
		out = append(out, raw[idx:idx+3]...)
	}
	return string(out)
}

func newestUsageUpdate(items []builtinStatusBotUsage) time.Time {
	latest := time.Time{}
	for _, item := range items {
		if latest.Before(item.Usage.UpdatedAt) {
			latest = item.Usage.UpdatedAt
		}
	}
	return latest
}
