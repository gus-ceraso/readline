package core

import "testing"

func TestWordOffsetsCountRunes(t *testing.T) {
	line := Line([]rune("café 世界 fin"))
	for _, test := range []struct {
		name      string
		tokenizer Tokenizer
	}{
		{"word", line.Tokenize},
		{"blank-word", line.TokenizeSpace},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, move := range []struct{ pos, want int }{
				{4, -4}, {5, -5}, {6, -1}, {7, -2}, {8, -3},
			} {
				if got := line.Backward(test.tokenizer, move.pos); got != move.want {
					t.Errorf("Backward at rune %d = %d, want %d", move.pos, got, move.want)
				}
			}
			if got := line.Forward(test.tokenizer, 0); got != 5 {
				t.Errorf("Forward = %d, want 5", got)
			}
			if got := line.ForwardEnd(test.tokenizer, 0); got != 3 {
				t.Errorf("ForwardEnd = %d, want 3", got)
			}
		})
	}
}
