package readline

import (
	"testing"

	"github.com/reeflective/readline/internal/core"
)

func TestSelfInsertUsesRunePositionsAndKeepsCombiningRunes(t *testing.T) {
	for _, text := range []string{"é", "世", "e\u0301"} {
		t.Run(text, func(t *testing.T) {
			rl := NewShell()
			_ = rl.Config.Set("output-meta", true)
			_ = rl.Config.Set("autopairs", false)
			rl.Line().Set([]rune("ab")...)
			rl.Cursor().Set(1)
			core.MatchedKeys(rl.Keys, []byte(text))
			rl.selfInsert()
			if got, want := string(*rl.Line()), "a"+text+"b"; got != want {
				t.Fatalf("inserted line = %q, want %q", got, want)
			}
			if got, want := rl.Cursor().Pos(), 1+len([]rune(text)); got != want {
				t.Fatalf("cursor rune index = %d, want %d", got, want)
			}
		})
	}
}
