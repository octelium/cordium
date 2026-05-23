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
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/portal/portal/acache"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type tunnelSrv struct {
	octeliumC octeliumc.ClientInterface
	cache     *acache.Cache
	domain    string
	tunMap    struct {
		mu     sync.RWMutex
		tunMap map[string]*tunCtx
	}

	tunnelKey struct {
		mu  sync.RWMutex
		key *wgtypes.Key
	}

	activityCtl *wsutils.ActivityCtl
	regionRef   *metav1.ObjectReference
}

func newTunnelSrv(ctx context.Context,
	octeliumC octeliumc.ClientInterface,
	cache *acache.Cache,
	domain string,
	activityCtl *wsutils.ActivityCtl,
	regionRef *metav1.ObjectReference) (*tunnelSrv, error) {

	zap.L().Debug("Creating tunnelSrv")
	ret := &tunnelSrv{
		octeliumC:   octeliumC,
		cache:       cache,
		domain:      domain,
		activityCtl: activityCtl,
		regionRef:   regionRef,
	}
	ret.tunMap.tunMap = make(map[string]*tunCtx)

	zap.L().Debug("Successfully created tunnelSrv")

	return ret, nil
}

func (s *tunnelSrv) getTunnelKey() (*wgtypes.Key, error) {
	s.tunnelKey.mu.RLock()
	if s.tunnelKey.key != nil {
		ret := s.tunnelKey.key
		s.tunnelKey.mu.RUnlock()
		return ret, nil
	}
	s.tunnelKey.mu.RUnlock()

	s.tunnelKey.mu.Lock()
	defer s.tunnelKey.mu.Unlock()

	ret, err := s.fetchTunnelKey()
	if err != nil {
		return nil, err
	}
	s.tunnelKey.key = ret
	return ret, nil
}

func (s *tunnelSrv) fetchTunnelKey() (*wgtypes.Key, error) {
	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()
	keySecret, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
		Name: "sys:ws-tunnel-wgkey",
	})
	if err != nil {
		return nil, err
	}

	privKey, err := wgtypes.ParseKey(ucordiumv1.ToSecret(keySecret).GetValueStr())
	if err != nil {
		return nil, err
	}
	return &privKey, nil
}

type tunCtx struct {
	wsUID  string
	tundev *netTun
	dev    *device.Device

	isClosed bool
	mu       sync.Mutex
}

var rgx = regexp.MustCompile(
	`^((port_(?P<port>[0-9]{1,5})_(?P<workspace>[a-z0-9-]+))|(?P<workspace_def>[a-z0-9-]+)|((?P<application>[a-z0-9-]+)_(?P<workspace_app>[a-z0-9-]+)))\.(((?P<svc>[a-z][a-z0-9-]{0,10}[a-z0-9])(\.|_)(?P<ns>[a-z][a-z0-9-]{0,10}[a-z0-9]))|(?P<svc_default>[a-z][a-z0-9-]{0,10}[a-z0-9]))$`)

type regexResult struct {
	workspace string
	port      int

	svc         string
	ns          string
	application string
}

func parseWorkspaceAppDomain(u string) (*regexResult, error) {

	match := rgx.FindStringSubmatch(u)

	ret := &regexResult{}

	if len(match) == 0 {
		return nil, errors.Errorf("Invalid path")
	}

	var workspace, workspace_def, workspace_app, port, svc, svcDefault, ns, application string

	for i, name := range rgx.SubexpNames() {
		switch name {
		case "port":
			port = match[i]
		case "workspace":
			workspace = match[i]
		case "workspace_def":
			workspace_def = match[i]
		case "workspace_app":
			workspace_app = match[i]
		case "application":
			application = match[i]
		case "svc":
			svc = match[i]
		case "svc_default":
			svcDefault = match[i]
		case "ns":
			ns = match[i]
		}
	}

	ret.application = application

	switch {
	case workspace != "":
		ret.workspace = workspace
	case workspace_def != "":
		ret.workspace = workspace_def
	case workspace_app != "":
		ret.workspace = workspace_app
	}

	if svcDefault != "" {
		ret.svc = svcDefault
		ret.ns = "default"
	} else {
		ret.svc = svc
		ret.ns = ns
	}

	if port != "" {
		portnum, err := strconv.Atoi(port)
		if err != nil {
			return nil, err
		}

		if portnum < 1 || portnum > 65535 {
			return nil, errors.Errorf("Invalid port number")
		}
		ret.port = portnum
	}

	switch {
	case ret.port != 0 && ret.application != "":
		return nil, errors.Errorf("Either port or application must be provided")
	}

	if ret.workspace == "" {
		return nil, errors.Errorf("Empty Workspace")
	}

	if ret.svc == "" {
		return nil, errors.Errorf("Empty service")
	}

	if ret.ns == "" {
		return nil, errors.Errorf("Empty namespace")
	}

	// zap.S().Debugf("got regex res: %+v", ret)

	return ret, nil
}

func (s *tunnelSrv) canAccessSharedApplication(ws *cordiumv1.Workspace, name string, userRef *metav1.ObjectReference) bool {

	for _, sharedPort := range ws.Status.SharedPorts {
		if sharedPort.ApplicationName == name {
			if app := ucordiumv1.ToWorkspace(ws).GetApplicationByName(name); app != nil {

				switch sharedPort.Mode {
				case cordiumv1.Workspace_Status_SharedPort_ALL:
					return true
				case cordiumv1.Workspace_Status_SharedPort_MEMBERS:
					_, err := s.cache.GetMembership(ws.Status.SpaceRef, userRef)
					if err != nil {
						return false
					}
					return true
				}

			}
		}
	}

	return false
}

func (s *tunnelSrv) startActivityCheck(ctx context.Context, wsUID string) {
	s.activityCtl.Set(wsUID)

	tickerCh := time.NewTicker(30 * time.Second)
	defer tickerCh.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tickerCh.C:
			s.activityCtl.Set(wsUID)
		}
	}
}

func (s *tunnelSrv) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	reqCtx := middlewares.GetCtxRequestContext(r.Context())
	sess := reqCtx.Session

	// zap.L().Debug("new tun request", zap.String("host", r.Header.Get("X-Forwarded-Host")))

	reqInfo, err := parseWorkspaceAppDomain(getForwardedHostPrefix(r.Header.Get("X-Forwarded-Host"), s.domain))
	if err != nil {
		zap.L().Debug("Could not get reqInfo", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// zap.L().Debug("Getting workspace",
	// 	zap.String("wsName", reqInfo.workspace), zap.String("sessUID", sess.Metadata.Uid))

	ws, err := s.getWorkspace(reqInfo, sess)
	if err != nil {
		zap.L().Debug("Could not get workspace", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	switch {
	case ws.Status.State == cordiumv1.Workspace_Status_RUNNING:
	case ws.Status.State == cordiumv1.Workspace_Status_PREPARING:
	case ldflags.IsDev() && ucordiumv1.ToWorkspace(ws).IsPreRunning():
	default:
		zap.L().Debug("Workspace is not active")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if ws.Status.RegionRef == nil || ws.Status.RegionRef.Uid != s.regionRef.Uid {
		zap.L().Debug("Not same region")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if ws.Status.SessionRef == nil || ws.Status.IsBuild || ws.Status.UserRef == nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch {
	case ws.Status.UserRef.Uid == sess.Status.UserRef.Uid:
	case reqInfo.application != "" &&
		s.canAccessSharedApplication(ws, reqInfo.application, sess.Status.UserRef):
	default:
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	port, err := s.getPort(reqInfo, ws)
	if err != nil {
		zap.L().Debug("Could not get port", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		return
	}

	tunCtx, err := s.getOrInitTunCtx(ws)
	if err != nil {
		zap.L().Debug("Could not get tunCtx", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	proxy, err := s.getProxy(tunCtx, port)
	if err != nil {
		zap.L().Debug("Could not get proxy", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	go s.startActivityCheck(r.Context(), ws.Metadata.Uid)

	proxy.ServeHTTP(w, r)
}

func (s *tunnelSrv) getPort(i *regexResult, ws *cordiumv1.Workspace) (int, error) {
	if i.port != 0 {
		return i.port, nil
	}
	if i.application != "" {
		app := ucordiumv1.ToWorkspace(ws).GetApplicationByName(i.application)
		if app == nil {
			return 0, errors.Errorf("No application with name: %s", i.application)
		}
		return int(app.Port), nil
	}

	defaultApp := ucordiumv1.ToWorkspace(ws).GetDefaultApplication()
	if defaultApp == nil {
		return 8080, nil
	}
	return int(defaultApp.Port), nil
}

func (s *tunnelSrv) getOrInitTunCtx(ws *cordiumv1.Workspace) (*tunCtx, error) {
	if ret := s.getTunCtx(ws); ret != nil {
		// zap.L().Debug("Found tunCtx", zap.String("wsUID", ws.Metadata.Uid))
		return ret, nil
	}

	return s.initTunCtx(ws)
}

func (s *tunnelSrv) getTunCtx(ws *cordiumv1.Workspace) *tunCtx {
	s.tunMap.mu.RLock()
	defer s.tunMap.mu.RUnlock()

	ret, ok := s.tunMap.tunMap[ws.Metadata.Uid]
	if !ok {
		return nil
	}
	return ret
}

func (s *tunnelSrv) initTunCtx(ws *cordiumv1.Workspace) (*tunCtx, error) {
	s.tunMap.mu.Lock()
	defer s.tunMap.mu.Unlock()

	privKey, err := s.getTunnelKey()
	if err != nil {
		return nil, err
	}
	tunctx, err := newTunCtx(context.Background(), ws, *privKey)
	if err != nil {
		return nil, err
	}

	s.tunMap.tunMap[ws.Metadata.Uid] = tunctx

	return tunctx, nil
}

func (s *tunnelSrv) remove(ws *cordiumv1.Workspace) error {
	wsUID := ws.Metadata.Uid
	s.tunMap.mu.Lock()
	defer s.tunMap.mu.Unlock()
	wsCtx, ok := s.tunMap.tunMap[wsUID]
	if !ok {
		return nil
	}

	err := wsCtx.close()

	delete(s.tunMap.tunMap, wsUID)
	return err
}

func newTunCtx(ctx context.Context, workspace *cordiumv1.Workspace, privKey wgtypes.Key) (*tunCtx, error) {

	// zap.L().Debug("Creating new tunCtx", zap.String("wsUID", workspace.Metadata.Uid))

	ret := &tunCtx{
		wsUID: workspace.Metadata.Uid,
	}

	_, addr, err := net.ParseCIDR("10.100.100.2/32")
	if err != nil {
		return nil, err
	}

	tundev, err := createNetstackTUN(addr)
	if err != nil {
		return nil, err
	}

	logger := device.NewLogger(device.LogLevelSilent, "")

	wsSupC, err := suputils.GetWorkspaceSupClient(workspace, nil)
	if err != nil {
		return nil, err
	}

	defer wsSupC.Close()

	device := device.NewDevice(tundev, conn.NewDefaultBind(), logger)

	if err := device.Up(); err != nil {
		return nil, err
	}

	pubKeyResp, err := wsSupC.C().GetTunnelPublicKey(ctx, &ccordiumv1.GetTunnelPublicKeyRequest{})
	if err != nil {
		return nil, err
	}
	pubKey, err := wgtypes.ParseKey(pubKeyResp.PublicKey)
	if err != nil {
		return nil, err
	}

	ips, err := net.LookupIP(suputils.GetWorkspaceSupHost(workspace))
	if err != nil {
		return nil, err
	}

	if len(ips) < 1 {
		return nil, errors.Errorf("Could not find IP addrs for Workspace")
	}

	tunPort := workspacecommon.GetWorkspaceTunnelPort()

	if err := device.IpcSet(toUAPI(privKey, pubKey, workspace,
		net.JoinHostPort(ips[0].String(), fmt.Sprintf("%d", tunPort)))); err != nil {
		return nil, err
	}

	ret.tundev = tundev
	ret.dev = device

	zap.L().Debug("Successfully created tunCtx", zap.String("wsUID", workspace.Metadata.Uid))

	return ret, nil
}

func (c *tunCtx) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isClosed {
		return nil
	}
	c.isClosed = true

	// zap.L().Debug("Closing tunCtx", zap.String("wsUID", c.wsUID))

	if c.tundev != nil {
		c.tundev.Close()
	}

	if c.dev != nil {
		c.dev.Close()
	}

	// zap.L().Debug("tunCtx successfully closed", zap.String("wsUID", c.wsUID))

	return nil
}

func toUAPI(sk wgtypes.Key, pubK wgtypes.Key, workspace *cordiumv1.Workspace, peerEndpoint string) string {

	var output strings.Builder
	output.WriteString(fmt.Sprintf("private_key=%s\n", wgKeyB64ToHex(sk.String())))

	output.WriteString("replace_peers=true\n")

	output.WriteString(fmt.Sprintf("public_key=%s\n", wgKeyB64ToHex(pubK.String())))
	output.WriteString(fmt.Sprintf("endpoint=%s\n", peerEndpoint))
	output.WriteString(fmt.Sprintf("persistent_keepalive_interval=%d\n", 30))

	output.WriteString("replace_allowed_ips=true\n")

	output.WriteString(fmt.Sprintf("allowed_ip=%s\n", "10.100.100.1/32"))

	return output.String()
}

func wgKeyB64ToHex(arg string) string {
	k, _ := base64.StdEncoding.DecodeString(arg)
	return hex.EncodeToString(k[:])
}

func (s *tunnelSrv) getWorkspace(reqInfo *regexResult, sess *corev1.Session) (*cordiumv1.Workspace, error) {
	return s.cache.GetWorkspace(sess.Status.UserRef.Uid, reqInfo.workspace)
}

func (s *tunnelSrv) getProxy(wsCtx *tunCtx, port int) (http.Handler, error) {

	ret := &httputil.ReverseProxy{
		BufferPool: newBufferPool(),
		Transport: &http.Transport{
			Proxy:       http.ProxyFromEnvironment,
			DialContext: wsCtx.tundev.GetNetstackNet().DialContext,

			ResponseHeaderTimeout: 40 * time.Second,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ReadBufferSize:        64 * 1024,
			WriteBufferSize:       64 * 1024,
		},
		Director: func(outReq *http.Request) {

			hostPort := net.JoinHostPort("10.100.100.1", fmt.Sprintf("%d", port))

			outReq.URL.Scheme = "http"
			outReq.Host = hostPort

			outReq.URL.Host = hostPort

			outReq.RequestURI = ""

			outReq.Proto = "HTTP/1.1"
			outReq.ProtoMajor = 1
			outReq.ProtoMinor = 1
		},

		FlushInterval: time.Duration(100 * time.Millisecond),

		ErrorHandler: func(w http.ResponseWriter, request *http.Request, err error) {
			statusCode := http.StatusInternalServerError
			zap.S().Debugf("Handling response err: %+v", err)
			switch {
			case errors.Is(err, io.EOF):
				statusCode = http.StatusBadGateway
			case errors.Is(err, context.Canceled):
				statusCode = 499
			default:
				var netErr net.Error
				if errors.As(err, &netErr) {
					if netErr.Timeout() {
						statusCode = http.StatusGatewayTimeout
					} else {
						statusCode = http.StatusBadGateway
					}
				}
			}

			zap.L().Debug("Workspace tunnel error",
				zap.String("wsUID", wsCtx.wsUID),
				zap.Error(err),
				zap.Int("statusCode", statusCode))
			w.WriteHeader(statusCode)
			_, werr := w.Write([]byte(http.StatusText(statusCode)))
			if werr != nil {
				zap.S().Debugf("Error while writing status code: %+v", werr)
			}
		},
	}

	return ret, nil
}

const bufferPoolSize = 32 * 1024

func newBufferPool() *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferPoolSize)
			},
		},
	}
}

type bufferPool struct {
	pool sync.Pool
}

func (b *bufferPool) Get() []byte {
	return b.pool.Get().([]byte)
}

func (b *bufferPool) Put(bytes []byte) {
	b.pool.Put(bytes)
}
