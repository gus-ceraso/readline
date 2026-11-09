//go:build unix

package display_test

import (
	"fmt"
	"strings"
	"testing"
)

func (c *console) rawOutput() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.output.String()
}

func TestCachedCursorOriginAndPreservedCursor(t *testing.T) {
	for _, prefill := range []int{0, 9} {
		t.Run(fmt.Sprint(prefill), func(t *testing.T) {
			c := startConsole(t, consoleConfig{
				prompt: "> ", cols: 60, rows: 10, prefill: prefill, bobo: true,
			})
			c.waitUntil(func(screen string) bool {
				return strings.Contains(screen, "> ") && c.probeQueries() > 0
			})
			probes := c.probeQueries()
			text := ""
			for _, ch := range "hello # ordinary text" {
				text += string(ch)
				c.send(string(ch))
				c.waitForScreen("> " + text)
				if got := c.probeQueries(); got != probes {
					t.Fatalf("typing interrupted a repaint with a cursor query: %d -> %d", probes, got)
				}
			}
			c.send("\r")
			c.waitForScreen("[LINE:" + text + "]")
			for style := 0; style <= 6; style++ {
				if strings.Contains(c.rawOutput(), fmt.Sprintf("\x1b[%d q", style)) {
					t.Fatalf("readline changed the terminal cursor style to %d", style)
				}
			}
		})
	}
}

func TestCustomMultilineColumnSkipsWrappedRows(t *testing.T) {
	for _, prefill := range []int{0, 9} {
		t.Run(fmt.Sprint(prefill), func(t *testing.T) {
			c := startConsole(t, consoleConfig{
				prompt: "> ", cols: 40, rows: 10, prefill: prefill, bobo: true,
			})
			c.waitForScreen("> ")
			text := strings.Repeat("x", 48) + "\nsecond\nthird"
			c.send("\x1b[200~" + text + "\x1b[201~")
			want := "> " + strings.Repeat("x", 38) + "\n" + strings.Repeat("x", 10) + "\n> second\n> third"
			c.waitUntil(func(screen string) bool {
				rows := strings.Split(screen, "\n")
				for i := range rows {
					rows[i] = strings.TrimRight(rows[i], " ")
				}
				return strings.Contains(strings.Join(rows, "\n"), want)
			})
			c.send("\r")
			c.waitForScreen("third]")
		})
	}
}

func TestBeforeReadCanStopWithoutAKey(t *testing.T) {
	c := startConsole(t, consoleConfig{
		prompt: "> ", cols: 60, rows: 10, bobo: true, beforeReadError: true,
	})
	c.waitForScreen("[ERR:before-read stop]")
}
