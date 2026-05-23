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
	"embed"
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"go.uber.org/zap"
)

//go:embed web
var fsWeb embed.FS

type octeliumManifest struct {
	Cluster octeliumManifestCluster `json:"cluster"`
}

type octeliumManifestCluster struct {
	Domain string `json:"domain"`
}

func (s *Server) handleManifest(w http.ResponseWriter, r *http.Request) {

	ret := &octeliumManifest{
		Cluster: octeliumManifestCluster{
			Domain: s.clusterDomain,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ret)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "octelium_domain",
		Value:    s.clusterDomain,
		Secure:   true,
		Domain:   s.clusterDomain,
		Path:     "/",
		SameSite: http.SameSiteNoneMode,
	})

	blob, err := fs.ReadFile(fsWeb, "web/index.html")
	if err != nil {
		zap.L().Debug("Could not read index.html file from web fs", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	w.Write(blob)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {

	zap.S().Debugf("Starting connect")
	ctx := r.Context()

	reqCtx := middlewares.GetCtxRequestContext(ctx)

	sess := reqCtx.Session

	zap.L().Debug("initializing socket conn", zap.String("sessName", sess.Metadata.Name))

	wsConn, err := s.initWebSocketConn(w, r)
	if err != nil {
		zap.S().Debugf("Could not initiate a websocket conn: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dctx, err := newDctx(ctx, sess, wsConn, s.activityCtl, s.supClientMap)
	if err != nil {
		zap.S().Debugf("Could not create a new dctx: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.dctxMap.mu.Lock()
	s.dctxMap.dctxMap[dctx.id] = dctx
	s.dctxMap.mu.Unlock()

	defer func() {
		s.dctxMap.mu.Lock()
		delete(s.dctxMap.dctxMap, dctx.id)
		s.dctxMap.mu.Unlock()
	}()

	if err := dctx.run(ctx); err != nil {
		zap.S().Debugf("Could not run dctx: %+v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	dctx.waitAndClose(ctx)
	zap.L().Debug("Exiting handleConnect", zap.String("dctxID", dctx.id))
}

func (s *Server) initWebSocketConn(w http.ResponseWriter, r *http.Request) (*websocket.Conn, error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			hdr := r.Header.Get("X-Octelium-Origin")
			if hdr == "" {
				return false
			}
			u, err := url.ParseRequestURI(hdr)
			if err != nil {
				return false
			}
			return strings.HasSuffix(u.Hostname(), s.clusterDomain)
		},
	}

	return upgrader.Upgrade(w, r, nil)
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {

	blob, err := fs.ReadFile(fsWeb, filepath.Join("web", r.URL.Path))
	if err != nil {
		zap.L().Debug("Could not read blob from web fs", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(r.URL.Path)))

	w.Write(blob)
}
