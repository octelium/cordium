/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package portal

import (
	"sync"
	"time"

	"context"

	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
)

type terminal struct {
	id string
	mu sync.Mutex
	c  *suputils.WorkspaceSupClient

	msgSrvCh chan<- *cordiumv1.ServerMessage

	// sendCh chan struct{}
	// recvCh chan struct{}

	cancelFn context.CancelFunc
	isClosed bool

	stream ccordiumv1.TerminalService_ListenTerminalClient
}

func newTerminal(
	tid string,
	c *suputils.WorkspaceSupClient,
	msgSrvCh chan<- *cordiumv1.ServerMessage) *terminal {
	ret := &terminal{
		c:        c,
		msgSrvCh: msgSrvCh,
	}

	ret.id = tid
	zap.L().Debug("Created new terminal listener",
		zap.String("id", ret.id))

	return ret
}

func (t *terminal) close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.isClosed {
		// zap.S().Debugf("Terminal: %s is already closed. Nothing to be done.", t.id)
		return nil
	}

	zap.S().Debugf("Closing terminal listener: %s", t.id)

	t.isClosed = true

	t.cancelFn()

	zap.S().Debugf("Terminal listener: %s closed", t.id)

	return nil
}

func (t *terminal) waitAndClose(ctx context.Context) error {

	// zap.S().Debugf("Starting wait to close for terminal: %s", t.id)

	<-ctx.Done()
	// zap.S().Debugf("terminal listener: %s wait ended by ctx", t.id)

	return t.close()
}

func (t *terminal) run(ctx context.Context) error {
	var err error
	zap.S().Debugf("Starting running terminal listener: %s", t.id)

	ctx, cancelFn := context.WithCancel(ctx)
	t.cancelFn = cancelFn

	t.stream, err = t.c.TermC().ListenTerminal(ctx, &ccordiumv1.ListenTerminalRequest{
		Id: t.id,
	})
	if err != nil {
		return err
	}

	go t.startSendLoop(ctx)
	go t.waitAndClose(ctx)

	return nil
}

func (t *terminal) startSendLoop(ctx context.Context) {

	// defer close(t.sendCh)

	zap.L().Debug("Starting send loop for terminal", zap.String("uid", t.id))
	// stupid fix to make sure that TerminalCreated msg is sent first before any data
	time.Sleep(100 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("ctx done. Exiting terminal send loop", zap.String("uid", t.id))
			return
		default:
			msg, err := t.stream.Recv()
			if err != nil {
				if grpcerr.IsCanceled(err) {
					return
				}
				zap.L().Error("Could not recv in terminal send loop. Exiting...", zap.Error(err))
				return
			}
			t.handleSend(msg)

		}
	}
}

func (t *terminal) handleSend(msg *ccordiumv1.ListenTerminalResponse) error {

	switch msg.Type.(type) {
	case *ccordiumv1.ListenTerminalResponse_Stdout_:
		t.sendMsg(&cordiumv1.ListenTerminalResponse{
			Type: &cordiumv1.ListenTerminalResponse_Stdout_{
				Stdout: &cordiumv1.ListenTerminalResponse_Stdout{
					Data: msg.GetStdout().Data,
				},
			},
		})
	case *ccordiumv1.ListenTerminalResponse_WindowSize_:
		t.sendMsg(&cordiumv1.ListenTerminalResponse{
			Type: &cordiumv1.ListenTerminalResponse_WindowSize_{
				WindowSize: &cordiumv1.ListenTerminalResponse_WindowSize{
					Cols: msg.GetWindowSize().Cols,
					Rows: msg.GetWindowSize().Rows,
				},
			},
		})
	case *ccordiumv1.ListenTerminalResponse_Close_:
		t.sendMsg(&cordiumv1.ListenTerminalResponse{
			Type: &cordiumv1.ListenTerminalResponse_Close_{
				Close: &cordiumv1.ListenTerminalResponse_Close{},
			},
		})
	}

	return nil
}

/*
func (t *terminal) startReceiveLoop(ctx context.Context) {
	defer close(t.recvCh)
	zap.L().Debug("Starting rcv loop for terminal", zap.String("uid", t.id))
	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("ctx done. Exiting terminal recv loop", zap.String("uid", t.id))
			return
		case msg, ok := <-t.msgClientCh:
			if !ok {
				return
			}
			if err := t.handleReceive(ctx, msg); err != nil {
				zap.L().Debug("Could not handle received msg", zap.Error(err))
			}
		}
	}
}
*/

/*
func (t *terminal) handleReceive(ctx context.Context, msg *cordiumv1.ClientMessage) error {
	switch msg.Type.(type) {
	case *cordiumv1.ClientMessage_SetTerminalWindowSizeRequest:

		_, err := t.workspaceCtx.c.TermC().SetWindowSize(ctx, &ccordiumv1.SetWindowSizeRequest{
			Id:     t.id,
			Height: msg.GetChangeWindow().Rows,
			Width:  msg.GetChangeWindow().Cols,
		})
		return err

	case *cordiumv1.ClientMessage_WriteTerminalDataRequest:
		t.activityCtl.Set(t.workspaceCtx.ws.Metadata.Name)

		req := msg.GetWriteTerminalDataRequest()

		_, err := t.workspaceCtx.c.TermC().WriteDataTerminal(ctx, &ccordiumv1.WriteDataTerminalRequest{
			Id:   t.id,
			Data: req.Data,
		})

		return err

	}
	return nil
}
*/

func (t *terminal) sendMsg(resp *cordiumv1.ListenTerminalResponse) {
	// zap.S().Debugf("Sending msg data: %s", string(data))
	msg := &cordiumv1.ServerMessage{
		Type: &cordiumv1.ServerMessage_ListenTerminalEvent_{
			ListenTerminalEvent: &cordiumv1.ServerMessage_ListenTerminalEvent{
				Id:                     t.id,
				ListenTerminalResponse: resp,
			},
		},
	}

	t.msgSrvCh <- msg
}

func (m *terminalMap) remove(tid string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if term, ok := m.terminalMap[tid]; ok {
		term.close()
		delete(m.terminalMap, tid)
	}
	return nil
}
