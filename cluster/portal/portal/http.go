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
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/pkg/errors"
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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(ret)
}

func (s *Server) getCSPConnectSrc() string {
	ret := []string{
		"'self'",
		fmt.Sprintf("https://octelium-api.%s", s.clusterDomain),
		fmt.Sprintf("https://*.octelium-api.%s", s.clusterDomain),
	}

	if host := strings.TrimPrefix(s.rootURL, "https://"); host != "" && host != s.rootURL {
		ret = append(ret, fmt.Sprintf("wss://%s", host))
	}

	return fmt.Sprintf("connect-src %s", strings.Join(ret, " "))
}

func (s *Server) setSecurityHeaders(w http.ResponseWriter, nonce string) {
	csp := strings.Join([]string{
		"default-src 'none'",
		fmt.Sprintf("script-src 'self' 'nonce-%s'", nonce),
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: https:",
		"font-src 'self'",
		s.getCSPConnectSrc(),
		"frame-src 'none'",
		"frame-ancestors 'none'",
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"manifest-src 'self'",
	}, "; ")

	w.Header().Set("Content-Security-Policy", csp)
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
}

func (s *Server) setDomainCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "octelium_domain",
		Value:    s.clusterDomain,
		Secure:   true,
		Domain:   s.clusterDomain,
		Path:     "/",
		SameSite: http.SameSiteNoneMode,
	})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	nonce := utilrand.GetRandomStringCanonical(24)

	blob, err := fs.ReadFile(fsWeb, "web/index.html")
	if err != nil {
		zap.L().Debug("Could not read index.html file from web fs", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	out, err := setIndexNonce(blob, nonce)
	if err != nil {
		zap.L().Error("Could not set the index.html nonce", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.setDomainCookie(w)
	s.setSecurityHeaders(w, nonce)

	w.Write(out)
}

func setIndexNonce(blob []byte, nonce string) ([]byte, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(blob))
	if err != nil {
		return nil, err
	}

	head := doc.Find("head").First()
	if head.Length() == 0 {
		return nil, errors.Errorf("Could not find head in index.html")
	}

	doc.Find("script").Each(func(_ int, sel *goquery.Selection) {
		sel.SetAttr("nonce", nonce)
	})
	doc.Find("link[rel='modulepreload']").Each(func(_ int, sel *goquery.Selection) {
		sel.SetAttr("nonce", nonce)
	})

	var ret bytes.Buffer
	ret.WriteString("<!DOCTYPE html>")
	if err := goquery.Render(&ret, head.Parent()); err != nil {
		return nil, err
	}

	return ret.Bytes(), nil
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {

	zap.S().Debugf("Starting connect")
	ctx := r.Context()

	reqCtx := middlewares.GetCtxRequestContext(ctx)
	if reqCtx == nil || reqCtx.Session == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

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
			return isClusterDomain(u.Hostname(), s.clusterDomain)
		},
	}

	return upgrader.Upgrade(w, r, nil)
}

func isClusterDomain(host, domain string) bool {
	if host == "" || domain == "" {
		return false
	}

	return host == domain || strings.HasSuffix(host, fmt.Sprintf(".%s", domain))
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {

	blob, err := fs.ReadFile(fsWeb, filepath.Join("web", r.URL.Path))
	if err != nil {
		zap.L().Debug("Could not read blob from web fs", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(r.URL.Path)))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

	w.Write(blob)
}
