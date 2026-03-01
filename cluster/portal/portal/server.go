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
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/suputils"
	"github.com/octelium/cordium/cluster/common/watchers"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/portal/portal/acache"
	wscontroller "github.com/octelium/cordium/cluster/portal/portal/controllers/workspaces"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares/auth"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/commoninit"
	"github.com/octelium/octelium/cluster/common/healthcheck"
	"github.com/octelium/octelium/cluster/common/httputils"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Server struct {
	octeliumC octeliumc.ClientInterface

	svcUID string

	jwkCtl        *jwkctl.Controller
	clusterDomain string
	genCache      *cache.Cache
	tunnelSrv     *tunnelSrv
	aCache        *acache.Cache

	activityCtl *wsutils.ActivityCtl

	dctxMap struct {
		dctxMap map[string]*dctx
		mu      sync.RWMutex
	}

	regionRef *metav1.ObjectReference

	// workspaceMap *workspaceMap

	rootURL      string
	supClientMap *suputils.SupervisorCMap
}

func newServer(ctx context.Context, octeliumC octeliumc.ClientInterface) (*Server, error) {

	var err error
	ret := &Server{
		octeliumC: octeliumC,
		svcUID:    os.Getenv("OCTELIUM_SVC_UID"),
		genCache:  cache.New(cache.NoExpiration, 1*time.Minute),
		/*
			workspaceMap: &workspaceMap{
				wsMap: make(map[string]*workspaceCtx),
			},
		*/
	}

	ret.dctxMap.dctxMap = make(map[string]*dctx)

	cc, err := octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	ret.aCache, err = acache.NewCache()
	if err != nil {
		return nil, err
	}
	ret.activityCtl, err = wsutils.NewActivityCtl(octeliumC)
	if err != nil {
		return nil, err
	}

	ret.clusterDomain = cc.Status.Domain

	ret.jwkCtl, err = jwkctl.NewJWKController(ctx, octeliumC)
	if err != nil {
		return nil, err
	}

	region, err := octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{Name: vutils.GetMyRegionName()})
	if err != nil {
		return nil, errors.Errorf("Could not get my region: %+v", err)
	}

	ret.regionRef = umetav1.GetObjectReference(region)

	ret.supClientMap = suputils.NewSupervisorCtxMap(ret.regionRef)

	ret.tunnelSrv, err = newTunnelSrv(ctx, octeliumC, ret.aCache, cc.Status.Domain, ret.activityCtl, ret.regionRef)
	if err != nil {
		return nil, errors.Errorf("Could not create tunnelSrv: %+v", err)
	}

	svc, err := octeliumC.CoreC().GetService(ctx, &rmetav1.GetOptions{
		Uid: ret.svcUID,
	})
	if err != nil {
		return nil, err
	}

	ret.rootURL = fmt.Sprintf("https://%s", vutils.GetServicePublicFQDN(svc, ret.clusterDomain))

	return ret, nil
}

var rgxGitProviderBegin = regexp.MustCompile(`^\/auth\/v1\/begin\/(?P<ws>[a-z0-9-]{36})$`)

func getForwardedHostPrefix(hdr string, domain string) string {
	return strings.TrimSuffix(hdr, fmt.Sprintf(".%s", domain))
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	switch {
	case rgx.MatchString(getForwardedHostPrefix(r.Header.Get("X-Forwarded-Host"), s.clusterDomain)):
		s.tunnelSrv.ServeHTTP(w, r)
		return
	case r.URL.Path == "/" && r.Method == "GET":
		s.handleIndex(w, r)
		return
	case r.URL.Path == "/connect":
		s.handleConnect(w, r)
		return

		/*
			case r.Method == "GET" && r.URL.Path == "/manifest.octelium.json":
				s.handleManifest(w, r)
				return
		*/
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/assets/"):
		s.handleAsset(w, r)
		return
	case r.Method == "POST" && rgxGitProviderBegin.MatchString(r.URL.Path):
		s.handleAuthGitProviderBegin(w, r)
		return
	case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/auth/v1/callback"):
		s.handleAuthGitProviderCallback(w, r)
		return
	case r.Method == "GET":
		s.handleIndex(w, r)
		return
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.jwkCtl.Run(ctx); err != nil {
		return err
	}

	if err := s.activityCtl.Run(ctx); err != nil {
		return err
	}

	srvErr := make(chan error)

	handler, err := s.getHTTPHandler(ctx)
	if err != nil {
		return err
	}

	go func() {
		srv := &http.Server{
			Handler:           handler,
			Addr:              vutils.ManagedServiceAddr,
			WriteTimeout:      15 * time.Second,
			ReadHeaderTimeout: 15 * time.Second,
			ConnContext: func(ctx context.Context, c net.Conn) context.Context {
				reqCtx := &middlewares.RequestContext{}

				return context.WithValue(ctx, middlewares.CtxRequestContext, reqCtx)
			},
		}

		if err := srv.ListenAndServe(); err != nil {
			srvErr <- err
		}
	}()

	return nil
}

func (s *Server) getHTTPHandler(ctx context.Context) (http.Handler, error) {
	chain := httputils.New()

	chain = chain.Append(func(next http.Handler) (http.Handler, error) {
		return auth.New(ctx, next, s.octeliumC)
	})

	handler, err := chain.Then(s)
	if err != nil {
		return nil, err
	}

	handler = http.AllowQuerySemicolons(handler)

	return handler, nil
}

func Run(ctx context.Context) error {
	if err := commoninit.Run(ctx, nil); err != nil {
		return err
	}

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}

	s, err := newServer(ctx, octeliumC)
	if err != nil {
		return err
	}

	if err := s.Run(ctx); err != nil {
		return err
	}

	ctl, err := wscontroller.NewController(s.aCache, s)
	if err != nil {
		return err
	}

	if err := watchers.NewCordiumV1(s.octeliumC).Workspace(ctx, nil,
		ctl.OnAdd, ctl.OnUpdate, ctl.OnDelete,
	); err != nil {
		return err
	}

	healthcheck.Run(vutils.HealthCheckPortManagedService)
	zap.S().Infof("Workspace Portal is running...")

	<-ctx.Done()

	return nil
}
