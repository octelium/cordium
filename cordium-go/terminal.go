/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cordium

import (
	"context"
	"io"
	"sync"

	"github.com/octelium/octelium/apis/main/cordiumv1"
)

// TerminalEventType is the type of a terminal event.
type TerminalEventType uint8

const (
	// TerminalOutput is a chunk of the terminal's output.
	TerminalOutput TerminalEventType = iota + 1
	// TerminalResized reports that the terminal's window was resized, which
	// happens when another listener resizes it.
	TerminalResized
	// TerminalClosed reports that the terminal was closed.
	TerminalClosed
)

// TerminalEvent is a single event of a terminal's stream.
type TerminalEvent struct {
	// Type is the type of the event.
	Type TerminalEventType
	// Data is the raw output, for the output events.
	Data []byte
	// Cols and Rows are the new window size, for the resize events.
	Cols, Rows uint32
}

type terminalConfig struct {
	cols, rows uint32
	bufferSize int
}

// TerminalOption configures an interactive terminal.
type TerminalOption func(*terminalConfig) error

// WithTerminalSize sets the initial window size of the terminal. It defaults to
// 80 by 24.
func WithTerminalSize(cols, rows uint32) TerminalOption {
	return func(c *terminalConfig) error {
		if cols == 0 || rows == 0 {
			return invalidArgumentf("terminal size must be positive")
		}
		c.cols, c.rows = cols, rows
		return nil
	}
}

// WithTerminalBuffer sizes the channel that [Terminal.Events] returns.
func WithTerminalBuffer(size int) TerminalOption {
	return func(c *terminalConfig) error {
		if size < 0 {
			return invalidArgumentf("negative terminal buffer size")
		}
		c.bufferSize = size
		return nil
	}
}

// Terminal is an interactive, PTY-backed shell running inside a Workspace.
//
// Unlike an exec session, a terminal outlives the connection that created it:
// several clients can attach to the same terminal at the same time, and
// detaching does not end the shell. [Terminal.Remove] is what terminates it.
type Terminal struct {
	c  *Client
	ws *Workspace
	id string

	ctx    context.Context
	events chan TerminalEvent
	cancel context.CancelFunc

	mu     sync.Mutex
	err    error
	done   bool
	closed bool
}

// NewTerminal creates an interactive terminal inside the Workspace and attaches
// to its output:
//
//	term, err := ws.NewTerminal(ctx, cordium.WithTerminalSize(120, 40))
//	if err != nil {
//		return err
//	}
//	defer term.Remove(context.WithoutCancel(ctx))
//
//	go io.Copy(term, os.Stdin)
//	for ev := range term.Events() {
//		if ev.Type == cordium.TerminalOutput {
//			os.Stdout.Write(ev.Data)
//		}
//	}
//
// The Workspace must be in the PREPARING or the RUNNING state.
func (w *Workspace) NewTerminal(ctx context.Context, opts ...TerminalOption) (*Terminal, error) {
	if err := w.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg := &terminalConfig{cols: 80, rows: 24, bufferSize: defaultOutputBuffer}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	resp, err := w.c.WorkspaceService().CreateTerminal(ctx, &cordiumv1.CreateTerminalRequest{
		WorkspaceRef: w.ref(),
		Cols:         cfg.cols,
		Rows:         cfg.rows,
	})
	if err != nil {
		return nil, err
	}

	return w.attachTerminal(ctx, resp.GetId(), cfg.bufferSize)
}

// AttachTerminal attaches to a terminal that already exists inside the
// Workspace. Several listeners can be attached to the same terminal at once.
func (w *Workspace) AttachTerminal(ctx context.Context, id string, opts ...TerminalOption) (*Terminal, error) {
	if err := w.c.ensureOpen(); err != nil {
		return nil, err
	}
	if id == "" {
		return nil, invalidArgumentf("empty terminal ID")
	}

	cfg := &terminalConfig{cols: 80, rows: 24, bufferSize: defaultOutputBuffer}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(cfg); err != nil {
			return nil, err
		}
	}

	return w.attachTerminal(ctx, id, cfg.bufferSize)
}

// Terminals lists the terminals that are currently open inside the Workspace.
func (w *Workspace) Terminals(ctx context.Context) ([]string, error) {
	if err := w.c.ensureOpen(); err != nil {
		return nil, err
	}

	resp, err := w.c.WorkspaceService().ListTerminal(ctx, &cordiumv1.ListTerminalRequest{
		WorkspaceRef: w.ref(),
	})
	if err != nil {
		return nil, err
	}

	ret := make([]string, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		ret = append(ret, item.GetId())
	}
	return ret, nil
}

func (w *Workspace) attachTerminal(ctx context.Context, id string, bufferSize int) (*Terminal, error) {
	ctx, cancel := context.WithCancel(ctx)

	strm, err := w.c.WorkspaceService().ListenTerminal(ctx, &cordiumv1.ListenTerminalRequest{Id: id})
	if err != nil {
		cancel()
		return nil, err
	}

	ret := &Terminal{
		c:      w.c,
		ws:     w,
		id:     id,
		ctx:    ctx,
		events: make(chan TerminalEvent, bufferSize),
		cancel: cancel,
	}

	go func() {
		defer close(ret.events)
		defer cancel()

		for {
			msg, err := strm.Recv()
			if err != nil {
				ret.finish(streamErr(ctx, err))
				return
			}

			var ev TerminalEvent
			switch msg.GetType().(type) {
			case *cordiumv1.ListenTerminalResponse_Stdout_:
				ev = TerminalEvent{Type: TerminalOutput, Data: msg.GetStdout().GetData()}
			case *cordiumv1.ListenTerminalResponse_WindowSize_:
				ev = TerminalEvent{
					Type: TerminalResized,
					Cols: msg.GetWindowSize().GetCols(),
					Rows: msg.GetWindowSize().GetRows(),
				}
			case *cordiumv1.ListenTerminalResponse_Close_:
				ret.emit(ctx, TerminalEvent{Type: TerminalClosed})
				ret.finish(nil)
				return
			default:
				continue
			}

			if !ret.emit(ctx, ev) {
				return
			}
		}
	}()

	return ret, nil
}

// ID returns the terminal's identifier, which is prefixed by the name of the
// Workspace that owns it.
func (t *Terminal) ID() string { return t.id }

// Workspace returns the Workspace that the terminal runs in.
func (t *Terminal) Workspace() *Workspace { return t.ws }

// Events returns the channel on which the terminal's events are published. It
// is closed once the stream ends.
func (t *Terminal) Events() <-chan TerminalEvent { return t.events }

// Write writes to the terminal's standard input, which makes a Terminal an
// [io.Writer].
func (t *Terminal) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if t.isClosed() {
		return 0, ErrTerminalClosed
	}

	if _, err := t.c.WorkspaceService().WriteTerminalData(t.ctx,
		&cordiumv1.WriteTerminalDataRequest{
			Id:   t.id,
			Data: p,
		}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// WriteString writes a string to the terminal's standard input.
func (t *Terminal) WriteString(s string) (int, error) { return t.Write([]byte(s)) }

// Resize changes the terminal's window size.
func (t *Terminal) Resize(ctx context.Context, cols, rows uint32) error {
	if t.isClosed() {
		return ErrTerminalClosed
	}
	if cols == 0 || rows == 0 {
		return invalidArgumentf("terminal size must be positive")
	}

	_, err := t.c.WorkspaceService().SetTerminalWindowSize(ctx,
		&cordiumv1.SetTerminalWindowSizeRequest{
			Id:   t.id,
			Cols: cols,
			Rows: rows,
		})
	return err
}

// Detach stops listening to the terminal without terminating it. The shell and
// everything running in it stay alive.
func (t *Terminal) Detach() error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	t.cancel()
	return nil
}

// Close is an alias for [Terminal.Detach], which makes a Terminal an
// [io.Closer]. Use [Terminal.Remove] to terminate the shell itself.
func (t *Terminal) Close() error { return t.Detach() }

// Remove terminates the terminal and detaches from it.
func (t *Terminal) Remove(ctx context.Context) error {
	_, err := t.c.WorkspaceService().RemoveTerminal(ctx, &cordiumv1.RemoveTerminalRequest{Id: t.id})
	_ = t.Detach()
	return err
}

// Err returns the error that ended the terminal's stream, if any.
func (t *Terminal) Err() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// Pipe connects the terminal to a pair of local streams and blocks until the
// terminal ends or ctx is canceled. It is what a command line client that wants
// a plain interactive shell needs; the caller is responsible for putting its
// own terminal into raw mode.
func (t *Terminal) Pipe(ctx context.Context, in io.Reader, out io.Writer) error {
	if out == nil {
		return invalidArgumentf("nil output writer")
	}

	if in != nil {
		go func() {
			buf := make([]byte, 4096)
			for {
				n, err := in.Read(buf)
				if n > 0 {
					if _, werr := t.Write(buf[:n]); werr != nil {
						return
					}
				}
				if err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}()
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev, ok := <-t.events:
			if !ok {
				return t.Err()
			}
			switch ev.Type {
			case TerminalOutput:
				if _, err := out.Write(ev.Data); err != nil {
					return err
				}
			case TerminalClosed:
				return t.Err()
			}
		}
	}
}

func (t *Terminal) emit(ctx context.Context, ev TerminalEvent) bool {
	select {
	case t.events <- ev:
		return true
	case <-ctx.Done():
		t.finish(ctx.Err())
		return false
	case <-t.c.closedCh():
		t.finish(ErrClientClosed)
		return false
	}
}

func (t *Terminal) isClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

func (t *Terminal) finish(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.done {
		t.done = true
		t.err = err
	}
}
