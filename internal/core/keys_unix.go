//go:build unix

package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"golang.org/x/sys/unix"

	"github.com/reeflective/readline/internal/term"
)

// GetCursorPos returns the current cursor position in the terminal.
// It is safe to call this function even if the shell is reading input.
func (k *Keys) GetCursorPos() (x, y int) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || k.cursorUnsupported || k.inputContext().Err() != nil {
		return -1, -1
	}
	queryCtx, cancel := context.WithTimeout(k.inputContext(), time.Second)
	defer cancel()
	stopWake := context.AfterFunc(queryCtx, k.RequestRefresh)
	defer stopWake()
	var cursor, pending []byte
	var match [][]string
	disable := func() (int, int) {
		k.QueueInput(pending)
		if k.inputContext().Err() == nil {
			k.cursorUnsupported = true
			os.Stderr.WriteString("\r\nterminal did not answer cursor-position query; disabling cursor probing\r\n")
		}
		return -1, -1
	}

	// Flush any buffered frame output first: the cursor position we are about
	// to query is only correct once the prompt printed so far is actually on
	// screen, not still sitting in the output buffer.
	term.FlushBuffer()

	// Echo the query and wait for the main key
	// reading routine to send us the response back.
	fmt.Print("\x1b[6n")

	// In order not to get stuck with an input that might be user-one
	// (like when the user typed before the shell is fully started, and yet not having
	// queried cursor yet), we keep reading from stdin until we find the cursor response.
	// Everything else is passed back as user input.
	for {
		switch {
		case k.waiting, k.reading:
			select {
			case cursor = <-k.cursor:
			case <-queryCtx.Done():
				return disable()
			}
		default:
			buf := make([]byte, keyScanBufSize)

			read, err := k.readStdinContext(buf, queryCtx)
			if errors.Is(err, errInputWake) {
				continue
			}
			if err != nil {
				return disable()
			}

			cursor = buf[:read]
		}

		// We have read (or have been passed) something.
		if len(cursor) == 0 {
			return disable()
		}

		// A report may be split across reads or share a read with typeahead.
		// Keep the partial report together and return every non-report byte to
		// the input queue, including bytes after the report.
		pending = append(pending, cursor...)
		match = rxRcvCursorPos.FindAllStringSubmatch(string(pending), 1)
		if len(match) == 0 {
			continue
		}
		location := rxRcvCursorPos.FindIndex(pending)
		k.mutex.Lock()
		k.buf = append(k.buf, pending[:location[0]]...)
		k.buf = append(k.buf, pending[location[1]:]...)
		k.mutex.Unlock()
		break
	}

	// We know that we have a cursor answer, process it.
	y, err := strconv.Atoi(match[0][1])
	if err != nil {
		return disable()
	}

	x, err = strconv.Atoi(match[0][2])
	if err != nil {
		return disable()
	}

	return x, y
}

func (k *Keys) readInputFiltered() (keys []byte, err error) {
	// Wait for input to be readable, or for an async refresh request, then
	// read one chunk. A refresh request returns errInputWake with no keys.
	buf := make([]byte, keyScanBufSize)

	read, err := k.readStdin(buf)
	if err != nil {
		return nil, err
	}

	// Always attempt to extract cursor position info.
	// If found, strip it and keep the remaining keys.
	cursor, keys := k.extractCursorPos(buf[:read])

	if len(cursor) > 0 {
		// A late report must not block input when no query is waiting.
		select {
		case k.cursor <- cursor:
		default:
		}
	}

	return keys, nil
}

// readStdin reads one chunk of input, but first waits (via poll) for stdin to
// become readable OR for an async refresh to be requested with RequestRefresh.
// On a refresh request it returns errInputWake with no data. Without a wake
// pipe, a bounded poll still observes cancellation. A non-file Stdin reader
// must provide its own cancellation support; it is read directly.
func (k *Keys) readStdin(buf []byte) (int, error) {
	return k.readStdinContext(buf, k.inputContext())
}

func (k *Keys) readStdinContext(buf []byte, ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	k.wakeMu.Lock()
	ready := k.wakeReady
	wakeR := k.wakeR
	k.wakeMu.Unlock()

	fd, ok := stdinFd()
	if !ok {
		return Stdin.Read(buf)
	}

	for {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
		timeout := 50 // Also supports cancellable reads without a wake pipe.
		if ready {
			fds = append(fds, unix.PollFd{Fd: int32(wakeR), Events: unix.POLLIN})
			timeout = -1
		}
		if deadline, ok := ctx.Deadline(); ok {
			remaining := max(1, int((time.Until(deadline)+time.Millisecond-1)/time.Millisecond))
			if timeout < 0 || remaining < timeout {
				timeout = remaining
			}
		}
		if _, err := unix.Poll(fds, timeout); err != nil {
			if errors.Is(err, unix.EINTR) {
				continue
			}

			return 0, err
		}
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		if fds[0].Revents&unix.POLLNVAL != 0 {
			return 0, unix.EBADF
		}

		// Real input takes priority, so a coincident wake never delays a
		// keystroke; the wake byte stays pending and is serviced once idle.
		if fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0 {
			return Stdin.Read(buf)
		}

		if ready && fds[1].Revents&unix.POLLIN != 0 {
			k.drainWake()
			return 0, errInputWake
		}
	}
}

// InitWake creates the pipe used to interrupt a blocking input read when an
// async refresh is requested. Safe to call repeatedly; on failure it leaves
// wake support disabled (async repaints then coalesce into the next keystroke).
func (k *Keys) InitWake() {
	k.CloseWake()

	var p [2]int
	if err := unix.Pipe(p[:]); err != nil {
		return
	}

	_ = unix.SetNonblock(p[0], true)
	_ = unix.SetNonblock(p[1], true)

	k.wakeMu.Lock()
	k.wakeR, k.wakeW = p[0], p[1]
	k.wakeReady = true
	k.wakeMu.Unlock()
}

// CloseWake tears down the wake pipe.
func (k *Keys) CloseWake() {
	k.wakeMu.Lock()
	defer k.wakeMu.Unlock()

	if !k.wakeReady {
		return
	}

	_ = unix.Close(k.wakeR)
	_ = unix.Close(k.wakeW)
	k.wakeReady = false
}

// RequestRefresh wakes an idle Readline loop so it repaints its UI. It is safe
// to call from any goroutine and is the primitive behind async UI updates (for
// instance ui.Hint.SetTransient). It is a no-op when wake support is
// unavailable, in which case the update is shown at the next keystroke.
func (k *Keys) RequestRefresh() {
	k.wakeMu.Lock()
	defer k.wakeMu.Unlock()

	if !k.wakeReady {
		return
	}

	// Non-blocking single-byte write. The pipe is drained on wake, and a full
	// pipe already means a wake is pending, so dropping the write is harmless.
	_, _ = unix.Write(k.wakeW, []byte{0})
}

// drainWake empties the wake pipe so a single request does not re-trigger.
func (k *Keys) drainWake() {
	var scratch [16]byte

	for {
		n, err := unix.Read(k.wakeR, scratch[:])
		if n <= 0 || err != nil {
			return
		}
	}
}

// stdinFd returns the file descriptor backing Stdin, if it exposes one.
func stdinFd() (int, bool) {
	if f, ok := Stdin.(interface{ Fd() uintptr }); ok {
		return int(f.Fd()), true
	}

	return 0, false
}
