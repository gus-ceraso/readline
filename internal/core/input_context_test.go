//go:build unix

package core

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/reeflective/readline/inputrc"
)

func TestWaitPreservesIncompleteUTF8(t *testing.T) {
	for _, queued := range []bool{false, true} {
		for _, text := range []string{"世", "é"} {
			for split := 1; split < len(text); split++ {
				keys := &Keys{}
				prefix, suffix := []byte(text[:split]), []byte(text[split:])
				if queued {
					keys.QueueInput(prefix)
					withStubStdin(t, readStep{data: suffix})
					if !keys.Empty() {
						t.Fatal("partial rune was dispatchable")
					}
				} else {
					withStubStdin(t, readStep{data: prefix}, readStep{data: suffix})
				}
				cfg := inputrc.NewConfig()
				_ = cfg.Set("convert-meta", false)
				WaitAvailableKeys(keys, cfg)
				if !bytes.Equal(keys.buf, []byte(text)) {
					t.Fatalf("queued=%t, text=%q split=%d: buffered %x", queued, text, split, keys.buf)
				}
			}
		}
	}
}

func TestNestedReadHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	keys := &Keys{}
	keys.SetContext(ctx)
	withStubStdin(t, readStep{data: []byte("must not be read")})
	if _, abort := keys.ReadKey(); !abort {
		t.Fatal("ReadKey ignored canceled context")
	}
	WaitAvailableKeys(keys, inputrc.NewConfig())
	if !errors.Is(keys.ReadError(), context.Canceled) {
		t.Fatalf("WaitAvailableKeys error = %v", keys.ReadError())
	}
}
