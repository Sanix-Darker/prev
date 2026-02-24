package renders

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderMarkdown(t *testing.T) {
	result := RenderMarkdown("# Hello\n\nThis is **bold** text.")
	assert.NotEmpty(t, result)
}

func TestRenderMarkdown_Empty(t *testing.T) {
	result := RenderMarkdown("")
	// Should not panic on empty input
	assert.NotNil(t, result)
}

func TestRenderMarkdown_CodeBlock(t *testing.T) {
	input := "```go\nfunc main() {\n    fmt.Println(\"hello\")\n}\n```"
	result := RenderMarkdown(input)
	assert.NotEmpty(t, result)
}
