package connector

import (
	"regexp"
	"strings"
)

func stripHiddenReplyMetadata(reply string, contract outputContract) string {
	if strings.TrimSpace(reply) == "" {
		return ""
	}
	stripped := reply
	for _, tag := range contract.hiddenTags() {
		stripped = hiddenTagBlockPattern(tag).ReplaceAllString(stripped, "")
	}
	return strings.TrimSpace(stripped)
}

func extractTaggedBlockContent(reply string, tag string) string {
	if strings.TrimSpace(reply) == "" {
		return ""
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	matches := hiddenTagCapturePattern(tag).FindStringSubmatch(reply)
	if len(matches) < 2 {
		return ""
	}
	return strings.TrimSpace(matches[1])
}

func hiddenTagBlockPattern(tag string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `\b[^>]*>.*?</` + regexp.QuoteMeta(tag) + `>`)
}

func hiddenTagCapturePattern(tag string) *regexp.Regexp {
	return regexp.MustCompile(`(?is)<` + regexp.QuoteMeta(tag) + `\b[^>]*>(.*?)</` + regexp.QuoteMeta(tag) + `>`)
}
