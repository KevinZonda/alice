package connector

import (
	"strings"

	"go.yaml.in/yaml/v3"
)

type soulDocument struct {
	Loaded         bool
	Body           string
	ImageRefs      []string
	OutputContract outputContract
}

type soulFrontmatter struct {
	ImageRefs      []string       `yaml:"image_refs"`
	OutputContract outputContract `yaml:"output_contract"`
}

type outputContract struct {
	HiddenTags     []string `yaml:"hidden_tags"`
	ReplyWillTag   string   `yaml:"reply_will_tag"`
	ReplyWillField string   `yaml:"reply_will_field"`
	MotionTag      string   `yaml:"motion_tag"`
	SuppressToken  string   `yaml:"suppress_token"`
}

func parseSoulDocument(raw string) soulDocument {
	text := strings.TrimSpace(raw)
	if text == "" {
		return soulDocument{Loaded: true}
	}

	lines := strings.Split(text, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return soulDocument{
			Loaded: true,
			Body:   text,
		}
	}

	end := -1
	for idx := 1; idx < len(lines); idx++ {
		if strings.TrimSpace(lines[idx]) == "---" {
			end = idx
			break
		}
	}
	if end <= 0 {
		return soulDocument{
			Loaded: true,
			Body:   text,
		}
	}

	frontmatterText := strings.Join(lines[1:end], "\n")
	body := strings.TrimSpace(strings.Join(lines[end+1:], "\n"))
	var frontmatter soulFrontmatter
	if err := yaml.Unmarshal([]byte(frontmatterText), &frontmatter); err != nil {
		return soulDocument{
			Loaded: true,
			Body:   text,
		}
	}
	return soulDocument{
		Loaded:         true,
		Body:           body,
		ImageRefs:      normalizeSoulImageRefs(frontmatter.ImageRefs),
		OutputContract: normalizeOutputContract(frontmatter.OutputContract),
	}
}

func normalizeSoulImageRefs(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, raw := range in {
		path := strings.TrimSpace(raw)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func normalizeOutputContract(in outputContract) outputContract {
	in.ReplyWillTag = strings.TrimSpace(in.ReplyWillTag)
	in.ReplyWillField = strings.TrimSpace(in.ReplyWillField)
	in.MotionTag = strings.TrimSpace(in.MotionTag)
	in.SuppressToken = strings.TrimSpace(in.SuppressToken)
	in.HiddenTags = normalizeSoulImageRefs(in.HiddenTags)
	return in
}

func (c outputContract) effectiveSuppressToken(fallback string) string {
	if c.SuppressToken != "" {
		return c.SuppressToken
	}
	return strings.TrimSpace(fallback)
}

func (c outputContract) hiddenTags() []string {
	candidates := make([]string, 0, len(c.HiddenTags)+2)
	candidates = append(candidates, c.HiddenTags...)
	if c.ReplyWillTag != "" {
		candidates = append(candidates, c.ReplyWillTag)
	}
	if c.MotionTag != "" {
		candidates = append(candidates, c.MotionTag)
	}
	return normalizeSoulImageRefs(candidates)
}
