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

type Terminal struct {
	ID string

	h  *H
	ws *cordiumv1.Workspace

	mu  sync.Mutex
	out strings.Builder
}

func (h *H) NewTerminal(t *testing.T, ws *cordiumv1.Workspace) *Terminal {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	res, err := h.WorkspaceC().CreateTerminal(ctx, &cordiumv1.CreateTerminalRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
		Cols:         120,
		Rows:         40,
	})
	if err != nil {
		t.Fatalf("Could not create a terminal in the Workspace %s: %+v",
			ws.Metadata.Name, err)
	}

	ret := &Terminal{
		ID: res.Id,
		h:  h,
		ws: ws,
	}

	strm, err := h.WorkspaceC().ListenTerminal(t.Context(), &cordiumv1.ListenTerminalRequest{
		Id: ret.ID,
	})
	if err != nil {
		t.Fatalf("Could not listen to the terminal %s: %+v", ret.ID, err)
	}

	go func() {
		for {
			msg, err := strm.Recv()
			if err != nil {
				zap.L().Debug("Terminal listen loop ended",
					zap.String("id", ret.ID), zap.Error(err))
				return
			}

			switch msg.Type.(type) {
			case *cordiumv1.ListenTerminalResponse_Stdout_:
				ret.mu.Lock()
				ret.out.Write(msg.GetStdout().Data)
				ret.mu.Unlock()
			case *cordiumv1.ListenTerminalResponse_Close_:
				return
			}
		}
	}()

	return ret
}

func (tm *Terminal) Write(t *testing.T, data string) {
	t.Helper()

	ctx, cancel := tm.h.Ctx(t)
	defer cancel()

	if _, err := tm.h.WorkspaceC().WriteTerminalData(ctx,
		&cordiumv1.WriteTerminalDataRequest{
			Id:   tm.ID,
			Data: []byte(data),
		}); err != nil {
		t.Fatalf("Could not write to the terminal %s: %+v", tm.ID, err)
	}
}

func (tm *Terminal) Output() string {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	return tm.out.String()
}

func (tm *Terminal) WaitOutput(t *testing.T, want string) {
	t.Helper()

	tm.h.Eventually(t, "the terminal output to contain "+want, ExecBudget,
		func(ctx context.Context) error {
			if strings.Contains(tm.Output(), want) {
				return nil
			}

			return errors.Errorf("the terminal output does not contain %q yet", want)
		})
}

func (tm *Terminal) SetWindowSize(t *testing.T, cols, rows uint32) {
	t.Helper()

	ctx, cancel := tm.h.Ctx(t)
	defer cancel()

	if _, err := tm.h.WorkspaceC().SetTerminalWindowSize(ctx,
		&cordiumv1.SetTerminalWindowSizeRequest{
			Id:   tm.ID,
			Cols: cols,
			Rows: rows,
		}); err != nil {
		t.Fatalf("Could not set the window size of the terminal %s: %+v", tm.ID, err)
	}
}

func (tm *Terminal) Remove(t *testing.T) {
	t.Helper()

	ctx, cancel := tm.h.Ctx(t)
	defer cancel()

	if _, err := tm.h.WorkspaceC().RemoveTerminal(ctx, &cordiumv1.RemoveTerminalRequest{
		Id: tm.ID,
	}); err != nil {
		t.Fatalf("Could not remove the terminal %s: %+v", tm.ID, err)
	}
}

func (h *H) Terminals(t *testing.T, ws *cordiumv1.Workspace) []*cordiumv1.Terminal {
	t.Helper()

	ctx, cancel := h.Ctx(t)
	defer cancel()

	res, err := h.WorkspaceC().ListTerminal(ctx, &cordiumv1.ListTerminalRequest{
		WorkspaceRef: umetav1.GetObjectReference(ws),
	})
	if err != nil {
		t.Fatalf("Could not list the terminals of the Workspace %s: %+v",
			ws.Metadata.Name, err)
	}

	return res.Items
}
