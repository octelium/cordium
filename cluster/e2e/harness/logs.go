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

package harness

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type LogStream struct {
	h  *H
	ws *cordiumv1.Workspace

	mu   sync.Mutex
	out  strings.Builder
	logs []*cordiumv1.ListenLogResponse
}

func (h *H) ListenLog(t *testing.T, ws *cordiumv1.Workspace) *LogStream {
	t.Helper()

	strm, err := h.WorkspaceC().ListenLog(t.Context(), &cordiumv1.ListenLogRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	})
	if err != nil {
		t.Fatalf("Could not listen to the logs of the Workspace %s: %+v",
			ws.Metadata.Name, err)
	}

	ret := &LogStream{
		h:  h,
		ws: ws,
	}

	go func() {
		for {
			msg, err := strm.Recv()
			if err != nil {
				zap.L().Debug("Workspace log loop ended",
					zap.String("name", ws.Metadata.Name), zap.Error(err))
				return
			}

			ret.mu.Lock()
			ret.logs = append(ret.logs, msg)
			ret.out.Write(msg.Data)
			ret.mu.Unlock()
		}
	}()

	return ret
}

func (l *LogStream) Output() string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.out.String()
}

func (l *LogStream) Entries() []*cordiumv1.ListenLogResponse {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]*cordiumv1.ListenLogResponse{}, l.logs...)
}

func (l *LogStream) WaitOutput(t *testing.T, want string) {
	t.Helper()

	l.h.Eventually(t, "the Workspace log stream to contain "+want, ExecBudget,
		func(ctx context.Context) error {
			if strings.Contains(l.Output(), want) {
				return nil
			}

			return errors.Errorf("the log stream does not contain %q yet", want)
		})
}
