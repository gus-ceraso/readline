package core

import (
	"bytes"
	"testing"
)

func TestQueueInputPreservesByteOrderAndPartialUTF8(t *testing.T) {
	keys := &Keys{buf: []byte("first:")}
	word := []byte("世界")
	keys.QueueInput(word[:1])
	keys.QueueInput(word[1:])
	keys.QueueInput([]byte("\r"))
	// Queued input must not retain the caller's mutable slice.
	word[0] = 'X'
	var got []byte
	for {
		key, empty := keys.Pop()
		if empty {
			break
		}
		got = append(got, key)
	}
	if want := []byte("first:世界\r"); !bytes.Equal(got, want) {
		t.Fatalf("queued bytes = %q, want %q", got, want)
	}
}
