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

package vigil

import (
	"context"
	"os"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/watchers"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/cordium/cluster/vigil/vigil/acache"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/commoninit"
	"github.com/octelium/octelium/cluster/common/healthcheck"
	"github.com/octelium/octelium/cluster/common/pprofsrv"
	"github.com/octelium/octelium/cluster/common/vutils"
	cwatechers "github.com/octelium/octelium/cluster/common/watchers"
	"github.com/octelium/octelium/cluster/vigil/vigil"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"go.uber.org/zap"
)

type srv struct {
	s           *vigil.Server
	octeliumC   octeliumc.ClientInterface
	aCache      *acache.Cache
	regionRef   *metav1.ObjectReference
	activityCtl *wsutils.ActivityCtl
}

func newServer(ctx context.Context, octeliumC octeliumc.ClientInterface) (*srv, error) {
	ret := &srv{
		octeliumC: octeliumC,
	}

	svc, err := octeliumC.CoreC().GetService(ctx, &rmetav1.GetOptions{Uid: os.Getenv("OCTELIUM_SVC_UID")})
	if err != nil {
		return nil, err
	}

	region, err := octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
		Name: vutils.GetMyRegionName(),
	})
	if err != nil {
		return nil, err
	}

	ret.regionRef = umetav1.GetObjectReference(region)

	s, err := vigil.NewServer(ctx, &vigil.Opts{
		OcteliumC:     octeliumC,
		Service:       svc,
		PostAuthorize: ret.doPostAuthorize,
		GetUpstream:   ret.doGetUpstream,
	})
	if err != nil {
		return nil, err
	}
	ret.s = s
	ret.aCache, err = acache.NewCache()
	if err != nil {
		return nil, err
	}

	ret.activityCtl, err = wsutils.NewActivityCtl(octeliumC)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (s *srv) Run(ctx context.Context) error {

	if err := watchers.NewCordiumV1(s.octeliumC).Workspace(ctx, nil,
		func(ctx context.Context, item *cordiumv1.Workspace) error {
			return s.aCache.SetWorkspace(item)
		},
		func(ctx context.Context, new, old *cordiumv1.Workspace) error {
			return s.aCache.SetWorkspace(new)
		},
		func(ctx context.Context, item *cordiumv1.Workspace) error {
			return s.aCache.DeleteWorkspace(item)
		},
	); err != nil {
		return err
	}

	if err := watchers.NewCordiumV1(s.octeliumC).Space(ctx, nil,
		func(ctx context.Context, item *cordiumv1.Space) error {
			return s.aCache.SetSpace(item)
		},
		func(ctx context.Context, new, old *cordiumv1.Space) error {
			return s.aCache.SetSpace(new)
		},
		func(ctx context.Context, item *cordiumv1.Space) error {
			return s.aCache.DeleteSpace(item)
		},
	); err != nil {
		return err
	}

	if err := cwatechers.NewCoreV1(s.octeliumC).Session(ctx, nil,
		func(ctx context.Context, item *corev1.Session) error {
			return s.aCache.SetSession(item)
		},
		func(ctx context.Context, new, old *corev1.Session) error {
			return s.aCache.SetSession(new)
		},
		func(ctx context.Context, item *corev1.Session) error {
			return s.aCache.DeleteSession(item)
		}); err != nil {
		return err
	}

	if err := s.s.Run(ctx); err != nil {
		return err
	}

	if err := s.activityCtl.Run(ctx); err != nil {
		return err
	}

	return nil
}

func Run(ctx context.Context) error {
	if err := commoninit.Run(ctx, nil); err != nil {
		return err
	}

	pprofsrv.New().Run(ctx)
	healthcheck.Run(vutils.HealthCheckPortVigil)

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}

	srv, err := newServer(ctx, octeliumC)
	if err != nil {
		return err
	}

	if err := srv.Run(ctx); err != nil {
		return err
	}

	zap.L().Info("Cordium Vigil is running...")

	<-ctx.Done()

	return nil
}
