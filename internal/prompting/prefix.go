package prompting

import "strings"

func ComposePromptPrefix(loader *Loader, promptPrefix string, personality string, noReplyToken string) (string, error) {
	if loader == nil {
		loader = DefaultLoader()
	}

	parts := make([]string, 0, 2)
	if prefix := strings.TrimSpace(promptPrefix); prefix != "" {
		parts = append(parts, prefix)
	}

	personality = strings.ToLower(strings.TrimSpace(personality))
	if personality != "" {
		rendered, err := loader.RenderFile("llm/personalities/"+personality+".md.tmpl", map[string]any{
			"NoReplyToken": strings.TrimSpace(noReplyToken),
		})
		if err != nil {
			return "", err
		}
		if rendered != "" {
			parts = append(parts, rendered)
		}
	}

	return strings.Join(parts, "\n\n"), nil
}
