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

package portal

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/gorilla/websocket"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type dctx struct {
	id string

	mu sync.Mutex

	isClosed bool

	sessUID string
	usrRef  *metav1.ObjectReference
	// devUID  string

	// wsMtx sync.Mutex

	websocket struct {
		conn *websocket.Conn
		sync.Mutex
	}
	// workspaceMap *workspaceMap
	terminalMap  terminalMap
	supClientMap *suputils.SupervisorCMap

	cancelFn context.CancelFunc

	msgSrvCh chan *cordiumv1.ServerMessage

	pingCh     chan struct{}
	sendDoneCh chan struct{}
	recvDoneCh chan struct{}

	recvCh chan *cordiumv1.ClientMessage

	activityCtl *wsutils.ActivityCtl
}

type terminalMap struct {
	terminalMap map[string]*terminal
	mu          sync.Mutex
}

func newDctx(ctx context.Context,
	sess *corev1.Session, wsConn *websocket.Conn,
	activityCtl *wsutils.ActivityCtl,
	supClientMap *suputils.SupervisorCMap) (*dctx, error) {

	ret := &dctx{
		id:      fmt.Sprintf("%s-%s", sess.Metadata.Name, utilrand.GetRandomStringLowercase(6)),
		usrRef:  sess.Status.UserRef,
		sessUID: sess.Metadata.Uid,
		// devUID:  ucorev1.ToSession(sess).GetDeviceUID(),
		websocket: struct {
			conn *websocket.Conn
			sync.Mutex
		}{
			conn: wsConn,
		},
		msgSrvCh: make(chan *cordiumv1.ServerMessage, 1024),

		pingCh:     make(chan struct{}, 3),
		sendDoneCh: make(chan struct{}, 3),
		recvDoneCh: make(chan struct{}, 3),

		recvCh: make(chan *cordiumv1.ClientMessage, 1024),

		supClientMap: supClientMap,

		terminalMap: terminalMap{
			terminalMap: make(map[string]*terminal),
		},

		activityCtl: activityCtl,
	}

	zap.S().Debugf("new dctx %s created", ret.id)

	return ret, nil
}

func (c *dctx) waitAndClose(ctx context.Context) error {

	zap.S().Debugf("Starting waiting to close for dctx: %s", c.id)
	select {
	case <-ctx.Done():
		zap.S().Debugf("dctx: %s wait ended by ctx", c.id)
	case <-c.pingCh:
		zap.S().Debugf("dctx: %s wait ended by ping ch", c.id)
	case <-c.sendDoneCh:
		zap.S().Debugf("dctx: %s wait ended by send ch", c.id)
	case <-c.recvDoneCh:
		zap.S().Debugf("dctx: %s wait ended by recv ch", c.id)
	}

	c.cancelFn()
	zap.S().Debugf("Closing dctx: %s", c.id)
	return c.close()
}

func (c *dctx) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed {
		zap.S().Debugf("dctx: %s is already closed. Nothing to be done", c.id)
		return nil
	}

	c.isClosed = true

	zap.S().Debugf("Closing dctx: %s", c.id)

	if c.websocket.conn != nil {
		c.websocket.conn.Close()
	}

	c.terminalMap.mu.Lock()
	for _, term := range c.terminalMap.terminalMap {
		term.close()
	}
	c.terminalMap.mu.Unlock()

	zap.S().Debugf("dctx: %s is now closed", c.id)
	return nil
}

func (c *dctx) run(ctx context.Context) error {

	ctx, cancelFn := context.WithCancel(ctx)
	c.cancelFn = cancelFn

	zap.S().Debugf("Starting running dctx: %s", c.id)
	go c.doPingLoop(ctx)
	go c.startReceiveLoop(ctx)
	go c.startProcessRecvLoop(ctx)
	go c.startSendLoop(ctx)

	return nil
}

func (c *dctx) doPingLoop(ctx context.Context) {
	tickerCh := time.NewTicker(30 * time.Second)
	defer tickerCh.Stop()

	defer close(c.pingCh)

	// zap.S().Debugf("Starting ping loop for dctx: %s", c.id)
	for {
		select {
		case <-tickerCh.C:
			if err := c.websocket.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				zap.S().Debugf("Could not send ping: %+v. Exiting ping loop for dctx: %s", err, c.id)
				return
			}
		case <-ctx.Done():
			// zap.S().Debugf("ping done reached for dctx: %s", c.id)
			return
		}
	}
}

func (c *dctx) doWriteToWs(data []byte) error {
	c.websocket.Lock()
	if err := c.websocket.conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
		c.websocket.Unlock()
		return err
	}
	c.websocket.Unlock()
	return nil
}

func (c *dctx) sendMessageServer(msg *cordiumv1.ServerMessage) error {
	// zap.S().Debugf("Sending response msg for dcx: %s: %+v", c.id, msg)
	payload, err := encodeServerMessage(msg)
	if err != nil {
		return err
	}

	return c.doWriteToWs(payload)
}

func (c *dctx) startReceiveLoop(ctx context.Context) {
	// zap.S().Debugf("Starting recv loop for dctx: %s", c.id)
	defer close(c.recvDoneCh)

	errN := 0

	for {
		select {
		case <-ctx.Done():
			zap.L().Debug("Exiting recv loop. ctx done", zap.String("dctxID", c.id))
			return
		default:
			msg, err := c.readRequestMsg()
			if err != nil {
				if websocket.IsCloseError(err,
					websocket.CloseAbnormalClosure,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway) {
					return
				}
				zap.S().Debugf("Error getting request msg: %+v", err)
				time.Sleep(100 * time.Millisecond)
				errN = errN + 1
				if errN >= 16 {
					zap.L().Debug("Too many errs in recv loop. Exiting", zap.String("dctxID", c.id))
					return
				}
				continue
			} else {
				errN = 0
			}

			c.recvCh <- msg
		}
	}

}

func (c *dctx) startProcessRecvLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.recvCh:
			if !ok {
				return
			}
			// zap.S().Debugf("Got request msg for dctx: %s: %+v", c.id, msg)

			if err := c.handleClientMsg(ctx, msg); err != nil {
				zap.L().Warn("Could not handle client msg",
					zap.Any("msg", msg), zap.Error(err))
			}

		}
	}
}

func (c *dctx) handleClientMsg(ctx context.Context, msg *cordiumv1.ClientMessage) error {

	switch msg.Type.(type) {
	case *cordiumv1.ClientMessage_SetTerminalWindowSizeRequest:
		req := msg.GetSetTerminalWindowSizeRequest()
		supC, err := c.getSupCFromTerminalID(ctx, req.Id)
		if err != nil {
			return err
		}

		_, err = supC.TermC().SetWindowSize(ctx, &ccordiumv1.SetWindowSizeRequest{
			Id:   req.Id,
			Cols: req.Cols,
			Rows: req.Rows,
		})
		if err != nil {
			return err
		}
		c.activityCtl.Set(supC.GetUID())
	case *cordiumv1.ClientMessage_WriteTerminalDataRequest:
		req := msg.GetWriteTerminalDataRequest()

		dataLen := len(req.Data)
		if dataLen == 0 {
			return nil
		}

		if dataLen > 6000 {
			return errors.Errorf("Data length is too high")
		}

		supC, err := c.getSupCFromTerminalID(ctx, req.Id)
		if err != nil {
			return err
		}

		_, err = supC.TermC().WriteDataTerminal(ctx, &ccordiumv1.WriteDataTerminalRequest{
			Id:   req.Id,
			Data: req.Data,
		})
		if err != nil {
			return err
		}

		c.activityCtl.Set(supC.GetUID())
	case *cordiumv1.ClientMessage_ListenTerminalRequest:
		req := msg.GetListenTerminalRequest()
		c.terminalMap.mu.Lock()
		if _, ok := c.terminalMap.terminalMap[req.Id]; ok {
			zap.L().Debug("Already listening to terminal. Nothing to be done...", zap.String("id", req.Id))
			c.terminalMap.mu.Unlock()
			return nil
		}
		supC, err := c.getSupCFromTerminalID(ctx, req.Id)
		if err != nil {
			return err
		}
		term := newTerminal(req.Id, supC, c.msgSrvCh)
		if err := term.run(ctx); err != nil {
			c.terminalMap.mu.Unlock()
			return err
		}
		c.terminalMap.terminalMap[req.Id] = term
		c.terminalMap.mu.Unlock()
		c.activityCtl.Set(supC.GetUID())
	case *cordiumv1.ClientMessage_ListenTerminalEndRequest_:
		req := msg.GetListenTerminalEndRequest()
		if err := c.terminalMap.remove(req.Id); err != nil {
			return err
		}
	}
	return nil
}

func (c *dctx) removeTerminalsByWorkspaceUID(wsUID string) error {
	var err error
	c.terminalMap.mu.Lock()
	defer c.terminalMap.mu.Unlock()
	zap.L().Debug("Removing terminal listeners for Workspace", zap.String("wsUID", wsUID))
	for _, term := range c.terminalMap.terminalMap {
		if term.c.GetUID() == wsUID {
			err = term.close()
			delete(c.terminalMap.terminalMap, term.id)
		}
	}

	return err
}

func (c *dctx) readRequestMsg() (*cordiumv1.ClientMessage, error) {
	typ, data, err := c.websocket.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if typ != websocket.BinaryMessage {
		return nil, errors.Errorf("websocket msg is not binary")
	}

	return decodeClientMessage(data)
}

func (c *dctx) startSendLoop(ctx context.Context) {
	defer close(c.sendDoneCh)

	zap.S().Debugf("Starting send loop for dctx: %s", c.id)
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-c.msgSrvCh:
			if err := c.sendMessageServer(msg); err != nil {
				if websocket.IsCloseError(err,
					websocket.CloseAbnormalClosure,
					websocket.CloseNormalClosure,
					websocket.CloseGoingAway) {
					return
				}
				zap.L().Warn("Could not send websocket msg", zap.Error(err))
				return
			}
		}
	}
}

func decodeClientMessage(data []byte) (*cordiumv1.ClientMessage, error) {
	ret := &cordiumv1.ClientMessage{}
	if err := pbutils.Unmarshal(data, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func encodeServerMessage(msg *cordiumv1.ServerMessage) ([]byte, error) {
	return pbutils.Marshal(msg)
}

func (c *dctx) getSupCFromTerminalID(ctx context.Context, tid string) (*suputils.WorkspaceSupClient, error) {

	if err := wsutils.CheckTerminalID(tid); err != nil {
		return nil, err
	}

	args := strings.Split(tid, "-")
	if len(args) != 2 {
		return nil, errors.Errorf("Invalid Terminal ID")
	}

	_, supC, err := c.supClientMap.GetByNameOrUID(args[0], c.usrRef)
	if err != nil {
		return nil, err
	}

	return supC, nil
}
