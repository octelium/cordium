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

package oproxy

import (
	"context"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/authv1"
	"github.com/octelium/octelium/octelium-go/authc"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type OcteliumProxy struct {
	opts *Opts

	srv *http.Server
	lis net.Listener

	resp accessTokenResponse

	mu       sync.Mutex
	isClosed bool

	c *authc.Client
}

const SocketPath = "/tmp/octelium-proxy.socket"

type accessTokenResponse struct {
	sync.RWMutex
	createdAt time.Time
	resp      *authv1.SessionToken
}

type Opts struct {
	Domain string

	ClientInfo *ccordiumv1.InitializeRequest_ClientInfo
}

func NewOcteliumProxy(opts *Opts) (*OcteliumProxy, error) {

	return &OcteliumProxy{
		opts: opts,
	}, nil
}

func (p *OcteliumProxy) Run(ctx context.Context) error {

	zap.L().Debug("Starting running Octelium proxy")
	var err error
	if err := p.doSetInitAccessToken(ctx); err != nil {
		return err
	}

	p.c, err = authc.NewClient(ctx, p.opts.Domain, &authc.Opts{
		GetRefreshToken: func(ctx context.Context, domain string) (string, error) {
			p.resp.RLock()
			defer p.resp.RUnlock()

			if p.resp.resp != nil && p.resp.resp.RefreshToken != "" {
				return p.resp.resp.RefreshToken, nil
			}
			return "", errors.Errorf("Could not find refresh token")
		},
	})
	if err != nil {
		return err
	}

	if err := p.runProxy(ctx); err != nil {
		return err
	}

	go p.startRefreshLoop(ctx)

	zap.L().Debug("Octelium proxy is now running")

	return nil
}

func (p *OcteliumProxy) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.isClosed {
		return nil
	}
	p.isClosed = true

	p.srv.Close()

	if p.lis != nil {
		p.lis.Close()
	}

	os.RemoveAll(SocketPath)
	return nil
}

func (p *OcteliumProxy) doSetInitAccessToken(ctx context.Context) error {

	p.resp.createdAt = time.Now()
	p.resp.resp = &authv1.SessionToken{
		AccessToken:  p.opts.ClientInfo.AccessToken,
		RefreshToken: p.opts.ClientInfo.RefreshToken,

		ExpiresIn: p.opts.ClientInfo.ExpiresIn,
	}

	return nil
}

func (p *OcteliumProxy) runProxy(ctx context.Context) error {
	var err error

	zap.L().Debug("oProxy: Starting listening on unix socket", zap.String("path", SocketPath))
	p.lis, err = net.Listen("unix", SocketPath)
	if err != nil {
		return errors.Errorf("Could not listen on unix socket path: %+v", err)
	}

	if err := os.Chmod(SocketPath, 0766); err != nil {
		return errors.Errorf("Could not chmod socke path: %+v", err)
	}

	handler, err := p.getHTTPHandler(ctx)
	if err != nil {
		return err
	}

	p.srv = &http.Server{
		Handler: handler,
	}

	go func() {
		zap.L().Debug("Starting serving auth HTTP proxy")
		p.srv.Serve(p.lis)
		zap.S().Debugf("srv done serving")
	}()

	return nil
}

func (p *OcteliumProxy) startRefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Minute)
	defer ticker.Stop()
	zap.L().Debug("Starting Octelium Proxy refresh loop")
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.doRefresh(ctx); err != nil {
				zap.L().Warn("Could not doRefresh", zap.Error(err))
			}
		}
	}
}

func (p *OcteliumProxy) doRefresh(ctx context.Context) error {

	at := p.resp.resp
	expiresAt := p.resp.createdAt.Add(time.Duration(at.ExpiresIn * int64(time.Second))).Add(-6 * time.Minute)

	if time.Now().Before(expiresAt) {
		// zap.L().Debug("No need to start an auth request")
		return nil
	}

	resp, err := p.c.C().AuthenticateWithRefreshToken(ctx, &authv1.AuthenticateWithRefreshTokenRequest{})
	if err != nil {
		if grpcerr.AlreadyExists(err) {
			zap.L().Debug("AuthenticateWithRefreshToken alreadyExists", zap.Error(err))
			return nil
		}

		return err
	}

	zap.L().Debug("Got a new session token")
	p.resp.Lock()
	p.resp.resp = resp
	p.resp.createdAt = time.Now()
	p.resp.Unlock()

	zap.L().Debug("oProxy doRefresh successfully done",
		zap.Time("shouldAt", expiresAt),
		zap.Time("now", time.Now()))

	return nil
}
