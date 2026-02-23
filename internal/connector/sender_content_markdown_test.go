package connector

import (
	"encoding/json"
	"testing"
)

type postContentPayload struct {
	ZhCN struct {
		Content [][]postElement `json:"content"`
	} `json:"zh_cn"`
}

type postElement struct {
	Tag  string `json:"tag"`
	Text string `json:"text"`
	Href string `json:"href"`
}

func parsePostContent(t *testing.T, raw string) postContentPayload {
	t.Helper()
	var payload postContentPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("unmarshal post content failed: %v", err)
	}
	return payload
}

func TestNormalizeMarkdownLineKeepsInlineCode(t *testing.T) {
	got := normalizeMarkdownLine("- `./bin/alice`")
	if got != "• `./bin/alice`" {
		t.Fatalf("unexpected normalized line: %q", got)
	}
}

func TestRichTextMarkdownMessageContentPreservesInlineCode(t *testing.T) {
	payload := parsePostContent(t, richTextMarkdownMessageContent("路径：`/home/codexbot/alice`"))
	if len(payload.ZhCN.Content) != 1 {
		t.Fatalf("expected one paragraph, got %#v", payload.ZhCN.Content)
	}
	if len(payload.ZhCN.Content[0]) != 1 {
		t.Fatalf("expected one text element, got %#v", payload.ZhCN.Content[0])
	}
	if payload.ZhCN.Content[0][0].Text != "路径：`/home/codexbot/alice`" {
		t.Fatalf("unexpected text element: %#v", payload.ZhCN.Content[0][0])
	}
}

func TestRichTextMarkdownMessageContentPreservesLinkSpacing(t *testing.T) {
	payload := parsePostContent(
		t,
		richTextMarkdownMessageContent("目录 `./bin` 请看 [文档](https://example.com/docs) 后继续"),
	)
	if len(payload.ZhCN.Content) != 1 {
		t.Fatalf("expected one paragraph, got %#v", payload.ZhCN.Content)
	}
	line := payload.ZhCN.Content[0]
	if len(line) != 3 {
		t.Fatalf("expected 3 elements (text/link/text), got %#v", line)
	}
	if line[0].Tag != "text" || line[0].Text != "目录 `./bin` 请看 " {
		t.Fatalf("unexpected first element: %#v", line[0])
	}
	if line[1].Tag != "a" || line[1].Text != "文档" || line[1].Href != "https://example.com/docs" {
		t.Fatalf("unexpected link element: %#v", line[1])
	}
	if line[2].Tag != "text" || line[2].Text != " 后继续" {
		t.Fatalf("unexpected tail text element: %#v", line[2])
	}
}
