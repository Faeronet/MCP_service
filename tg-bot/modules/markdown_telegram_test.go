package modules

import (
	"strings"
	"testing"
)

func TestMarkdownToTelegramHTML(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"*жирный*", "<b>жирный</b>"},
		{"**жирный**", "<b>жирный</b>"},
		{"_курсив_", "<i>курсив</i>"},
		{"`код`", "<code>код</code>"},
		{"```go\nfmt.Println(\"hi\")\n```", "<pre><code>fmt.Println(\"hi\")</code></pre>"},
		{"[ссылка](https://example.com)", `<a href="https://example.com">ссылка</a>`},
		{"~~зачёркнуто~~", "<s>зачёркнуто</s>"},
		{"## Заголовок", "<b>Заголовок</b>"},
		{"a & b < c", "a &amp; b &lt; c"},
	}
	for _, tc := range tests {
		got := MarkdownToTelegramHTML(tc.in)
		if got != tc.want {
			t.Errorf("MarkdownToTelegramHTML(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitMarkdownAwareChunksPreservesCodeBlock(t *testing.T) {
	code := strings.Repeat("x", 100)
	prose := strings.Repeat("word ", 800)
	text := prose + "\n\n```\n" + code + "\n```\n\n" + prose
	chunks := splitMarkdownAwareChunks(text, 500)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	foundCode := false
	for _, ch := range chunks {
		if strings.Contains(ch, "```") && strings.Contains(ch, code) {
			foundCode = true
		}
	}
	if !foundCode {
		t.Fatal("code block should stay intact in at least one chunk")
	}
}
