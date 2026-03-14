package alice

import (
	"embed"
	"io/fs"
)

// PromptFS exposes the bundled prompt templates from the repository's prompts directory.
//
//go:embed prompts
var embeddedFiles embed.FS

var PromptFS = mustSub(embeddedFiles, "prompts")

func mustSub(root fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(root, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
