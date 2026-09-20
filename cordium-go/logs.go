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
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
)

// LogStage is the initialization stage that produced a log entry.
type LogStage uint8

const (
	// LogStageUnknown is an entry of an unclassified stage.
	LogStageUnknown LogStage = iota
	// LogStageCloningRepo is an entry that was produced while cloning a
	// repository.
	LogStageCloningRepo
	// LogStagePullingImage is an entry that was produced while pulling the
	// container image.
	LogStagePullingImage
	// LogStageBuildingImage is an entry that was produced while building the
	// container image.
	LogStageBuildingImage
	// LogStageTask is an entry that was produced by a lifecycle Task.
	LogStageTask
)

// String returns the name of the stage.
func (s LogStage) String() string {
	switch s {
	case LogStageCloningRepo:
		return "CLONING_REPO"
	case LogStagePullingImage:
		return "PULLING_IMAGE"
	case LogStageBuildingImage:
		return "BUILDING_IMAGE"
	case LogStageTask:
		return "TASK"
	default:
		return "UNKNOWN"
	}
}

func logStageFrom(typ cordiumv1.ListenLogResponse_Type) LogStage {
	switch typ {
	case cordiumv1.ListenLogResponse_TYPE_CLONING_REPO:
		return LogStageCloningRepo
	case cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE:
		return LogStagePullingImage
	case cordiumv1.ListenLogResponse_TYPE_BUILDING_IMAGE:
		return LogStageBuildingImage
	case cordiumv1.ListenLogResponse_TYPE_TASK:
		return LogStageTask
	default:
		return LogStageUnknown
	}
}

// LogEntry is a single entry of a Workspace's initialization logs.
type LogEntry struct {
	// At is when the entry was produced.
	At time.Time
	// Stage is the initialization stage that produced the entry.
	Stage LogStage
	// Stream is the output stream on which the entry was emitted.
	Stream Stream
	// Data is the raw content of the entry.
	Data []byte
}

// String returns the entry's content as a string.
func (e LogEntry) String() string { return string(e.Data) }

// LogStream carries a Workspace's initialization logs, which is what the
// repository cloning, the image pulling, the image building and the lifecycle
// Tasks produce.
type LogStream struct {
	entries chan LogEntry
	cancel  context.CancelFunc

	mu   sync.Mutex
	err  error
	done bool
}

// Logs opens a stream of the Workspace's initialization logs:
//
//	logs, err := ws.Logs(ctx)
//	if err != nil {
//		return err
//	}
//	defer logs.Close()
//
//	for entry := range logs.Entries() {
//		fmt.Printf("[%s] %s", entry.Stage, entry.Data)
//	}
//	return logs.Err()
//
// It is the first thing to look at when a run fails, since the failure of an
// image build or of a lifecycle Task is explained there rather than in the
// Workspace's status.
//
// The Workspace must be in the PREPARING or the RUNNING state.
func (w *Workspace) Logs(ctx context.Context) (*LogStream, error) {
	if err := w.c.ensureOpen(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	strm, err := w.c.WorkspaceService().ListenLog(ctx, &cordiumv1.ListenLogRequest{
		WorkspaceRef: w.ref(),
	})
	if err != nil {
		cancel()
		return nil, err
	}

	ret := &LogStream{
		entries: make(chan LogEntry, defaultOutputBuffer),
		cancel:  cancel,
	}

	go func() {
		defer close(ret.entries)
		defer cancel()

		for {
			msg, err := strm.Recv()
			if err != nil {
				ret.finish(streamErr(ctx, err))
				return
			}

			entry := LogEntry{
				At:     msg.GetCreatedAt().AsTime(),
				Stage:  logStageFrom(msg.GetType()),
				Stream: StreamStdout,
				Data:   msg.GetData(),
			}
			if msg.GetMode() == cordiumv1.ListenLogResponse_MODE_STDERR {
				entry.Stream = StreamStderr
			}

			select {
			case ret.entries <- entry:
			case <-ctx.Done():
				ret.finish(ctx.Err())
				return
			case <-w.c.closedCh():
				ret.finish(ErrClientClosed)
				return
			}
		}
	}()

	return ret, nil
}

// StreamLogsTo copies the Workspace's initialization logs into a pair of
// writers until the stream ends or ctx is canceled. A nil stderr writer sends
// everything to stdout.
func (w *Workspace) StreamLogsTo(ctx context.Context, stdout, stderr io.Writer) error {
	if stdout == nil {
		return invalidArgumentf("nil stdout writer")
	}
	if stderr == nil {
		stderr = stdout
	}

	logs, err := w.Logs(ctx)
	if err != nil {
		return err
	}
	defer logs.Close()

	for entry := range logs.Entries() {
		writer := stdout
		if entry.Stream == StreamStderr {
			writer = stderr
		}
		if _, err := writer.Write(entry.Data); err != nil {
			return err
		}
	}
	return logs.Err()
}

// Entries returns the channel on which the log entries are published. It is
// closed once the stream ends.
func (s *LogStream) Entries() <-chan LogEntry { return s.entries }

// Err returns the error that ended the stream, if any.
func (s *LogStream) Err() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Close ends the stream. It is safe to call it more than once.
func (s *LogStream) Close() error {
	s.cancel()
	return nil
}

func (s *LogStream) finish(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.done {
		s.done = true
		s.err = err
	}
}
