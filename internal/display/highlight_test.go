package display

import (
	"strings"
	"testing"

	"github.com/reeflective/readline/inputrc"
	"github.com/reeflective/readline/internal/color"
	"github.com/reeflective/readline/internal/core"
)

func TestEmptyCommentMarkerDisablesHighlighting(t *testing.T) {
	engine := &Engine{opts: inputrc.NewConfig()}
	line := "ordinary text # not a comment"
	// Exercise disabling a previously compiled comment pattern, too.
	for _, comment := range []string{"#", "", "\"\""} {
		_ = engine.opts.Set("comment-begin", comment)
		got := engine.highlightLine([]rune(line), core.Selection{})
		if comment == "#" {
			if !strings.Contains(got, "\x1b[38;05;244m") {
				t.Fatalf("configured comment was not highlighted: %q", got)
			}
		} else if want := line + color.Reset; got != want {
			t.Fatalf("disabled comment highlighting = %q, want %q", got, want)
		}
	}
}
