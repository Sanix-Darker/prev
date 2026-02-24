package renders

import (
	"fmt"
	"os"

	"github.com/sanix-darker/prev/internal/provider"
	"golang.org/x/term"
)

// RenderStream reads chunks from a StreamResult, printing content progressively.
// For TTY environments, it renders inline. For non-TTY, it accumulates and renders at the end.
func RenderStream(result provider.StreamResult) error {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))

	if isTTY {
		return renderStreamTTY(result)
	}
	return renderStreamRaw(result)
}

// renderStreamTTY prints chunks directly as they arrive.
func renderStreamTTY(result provider.StreamResult) error {
	for chunk := range result.Chunks {
		if chunk.Content != "" {
			fmt.Print(chunk.Content)
		}
	}
	fmt.Println()

	if err := <-result.Err; err != nil {
		return err
	}
	return nil
}

// renderStreamRaw accumulates content and renders markdown at the end.
func renderStreamRaw(result provider.StreamResult) error {
	var content string
	for chunk := range result.Chunks {
		content += chunk.Content
	}

	if err := <-result.Err; err != nil {
		return err
	}

	if content != "" {
		fmt.Print(RenderMarkdown(content))
	}
	return nil
}
