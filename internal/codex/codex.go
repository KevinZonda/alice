package codex

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Runner struct {
	Command      string
	Timeout      time.Duration
	PromptPrefix string
	WorkspaceDir string
}

func (r Runner) Run(ctx context.Context, userText string) (string, error) {
	prompt := strings.TrimSpace(r.PromptPrefix) + "\n\n" + strings.TrimSpace(userText)
	if strings.TrimSpace(prompt) == "" {
		return "", errors.New("empty prompt")
	}

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}

	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(
		tctx,
		r.Command,
		"exec",
		"--json",
		"--skip-git-repo-check",
		"--sandbox",
		"read-only",
		prompt,
	)
	if strings.TrimSpace(r.WorkspaceDir) != "" {
		cmd.Dir = r.WorkspaceDir
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if errors.Is(tctx.Err(), context.DeadlineExceeded) {
		return "", errors.New("codex timeout")
	}
	if err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		if len(detail) > 400 {
			detail = detail[:400]
		}
		return "", fmt.Errorf("codex exec failed: %w (%s)", err, detail)
	}

	message, err := ParseFinalMessage(stdout.String())
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(message), nil
}

func ParseFinalMessage(jsonlOutput string) (string, error) {
	var lastMessage string
	scanner := bufio.NewScanner(strings.NewReader(jsonlOutput))
	scanner.Buffer(make([]byte, 0, 64*1024), 5*1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event["type"] != "item.completed" {
			continue
		}

		item, ok := event["item"].(map[string]any)
		if !ok {
			continue
		}
		if item["type"] != "agent_message" {
			continue
		}
		text, ok := item["text"].(string)
		if !ok {
			continue
		}
		if strings.TrimSpace(text) != "" {
			lastMessage = text
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}
	if strings.TrimSpace(lastMessage) == "" {
		return "", errors.New("codex returned no final agent message")
	}
	return lastMessage, nil
}
