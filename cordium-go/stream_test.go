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
	"bytes"
	"errors"
	"io"
	"testing"
	"time"
)

func TestLogsStream(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())
	ws := testWorkspace(t, c)

	logs, err := ws.Logs(t.Context())
	if err != nil {
		t.Fatalf("Logs: %v", err)
	}
	defer logs.Close()

	var entries []LogEntry
	for entry := range logs.Entries() {
		entries = append(entries, entry)
	}
	if err := logs.Err(); err != nil {
		t.Fatalf("Err: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Stage != LogStagePullingImage || entries[0].Stream != StreamStdout {
		t.Errorf("first entry = %+v", entries[0])
	}
	if entries[1].Stage != LogStageTask || entries[1].Stream != StreamStderr {
		t.Errorf("second entry = %+v", entries[1])
	}
	if entries[0].At.IsZero() {
		t.Error("the entry has no timestamp")
	}
}

func TestStreamLogsTo(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())
	ws := testWorkspace(t, c)

	var out, errBuf bytes.Buffer
	if err := ws.StreamLogsTo(t.Context(), &out, &errBuf); err != nil {
		t.Fatalf("StreamLogsTo: %v", err)
	}

	if got, want := out.String(), "pulling python:3.11\n"; got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
	if got, want := errBuf.String(), "npm WARN deprecated\n"; got != want {
		t.Errorf("stderr = %q, want %q", got, want)
	}

	if err := ws.StreamLogsTo(t.Context(), nil, nil); !IsInvalidArgument(err) {
		t.Errorf("a nil writer: %v", err)
	}
}

func TestTerminalRoundTrip(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)
	ws := testWorkspace(t, c)

	term, err := ws.NewTerminal(t.Context(), WithTerminalSize(120, 40))
	if err != nil {
		t.Fatalf("NewTerminal: %v", err)
	}
	defer term.Detach()

	if term.ID() != ws.Name()+"-term1" {
		t.Errorf("terminal ID = %q", term.ID())
	}
	if term.Workspace() != ws {
		t.Error("Workspace() does not point back at the Workspace")
	}

	fake.mu.Lock()
	cols, rows := fake.terminalCols, fake.terminalRows
	fake.mu.Unlock()
	if cols != 120 || rows != 40 {
		t.Errorf("initial size = %dx%d, want 120x40", cols, rows)
	}

	ev := <-term.Events()
	if ev.Type != TerminalOutput || string(ev.Data) != "$ " {
		t.Errorf("first event = %+v", ev)
	}

	if _, err := term.WriteString("echo hi\n"); err != nil {
		t.Fatalf("WriteString: %v", err)
	}

	select {
	case ev := <-term.Events():
		if string(ev.Data) != "echo hi\n" {
			t.Errorf("echoed event = %+v", ev)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the terminal did not echo the input back")
	}

	if err := term.Resize(t.Context(), 100, 30); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	fake.mu.Lock()
	cols, rows = fake.terminalCols, fake.terminalRows
	fake.mu.Unlock()
	if cols != 100 || rows != 30 {
		t.Errorf("resized to %dx%d, want 100x30", cols, rows)
	}

	terminals, err := ws.Terminals(t.Context())
	if err != nil {
		t.Fatalf("Terminals: %v", err)
	}
	if len(terminals) != 1 || terminals[0] != term.ID() {
		t.Errorf("Terminals() = %v", terminals)
	}

	if err := term.Remove(t.Context()); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	fake.mu.Lock()
	removed := fake.removedTerminal
	fake.mu.Unlock()
	if removed != term.ID() {
		t.Errorf("removed terminal = %q", removed)
	}

	if _, err := term.WriteString("late"); !errors.Is(err, ErrTerminalClosed) {
		t.Errorf("writing to a removed terminal: %v", err)
	}
	if err := term.Resize(t.Context(), 10, 10); !errors.Is(err, ErrTerminalClosed) {
		t.Errorf("resizing a removed terminal: %v", err)
	}
}

func TestTerminalRejectsInvalidSize(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())
	ws := testWorkspace(t, c)

	if _, err := ws.NewTerminal(t.Context(), WithTerminalSize(0, 24)); !IsInvalidArgument(err) {
		t.Errorf("a zero column count: %v", err)
	}
	if _, err := ws.AttachTerminal(t.Context(), ""); !IsInvalidArgument(err) {
		t.Errorf("an empty terminal ID: %v", err)
	}
}

var (
	_ io.WriteCloser = (*Terminal)(nil)
	_ io.Closer      = (*LogStream)(nil)
)
