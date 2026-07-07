package views

import (
	"strings"
	"testing"
)

// TestRenderMarkdownStructure locks in the HTML element structure the prompter
// (and other views) style: headings, lists, blockquotes and rules must survive
// rendering, or the .prompter-content CSS that makes segment markers visible has
// nothing to hook onto.
func TestRenderMarkdownStructure(t *testing.T) {
	md := strings.Join([]string{
		"# Intro",
		"",
		"## Segment 1",
		"",
		"Some *emphasis* and **bold**.",
		"",
		"- one",
		"- two",
		"",
		"> a quote",
		"",
		"---",
		"",
		"## Segment 2",
	}, "\n")

	got := string(renderMarkdown(md))

	for _, want := range []string{"<h1>", "<h2>", "<ul>", "<li>", "<blockquote>", "<hr>", "<em>", "<strong>"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered markdown missing %q\n---\n%s", want, got)
		}
	}
}

func TestInitial(t *testing.T) {
	cases := map[string]string{
		"Östen": "Ö", // multi-byte first rune must survive intact, not become "Ã"
		"alice": "A",
		"":      "",
		"李雷":    "李",
	}
	for in, want := range cases {
		if got := initial(in); got != want {
			t.Errorf("initial(%q) = %q, want %q", in, got, want)
		}
	}
}
