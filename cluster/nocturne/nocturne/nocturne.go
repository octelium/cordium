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

package nocturne

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	wswatchers "github.com/octelium/cordium/cluster/common/watchers"
	tmplcontroller "github.com/octelium/cordium/cluster/nocturne/nocturne/controllers/templates"
	usrcontroller "github.com/octelium/cordium/cluster/nocturne/nocturne/controllers/users"
	wscontroller "github.com/octelium/cordium/cluster/nocturne/nocturne/controllers/workspaces"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/commoninit"
	"github.com/octelium/octelium/cluster/common/healthcheck"
	"github.com/octelium/octelium/cluster/common/jwkctl"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/cluster/common/watchers"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
)

func Run(ctx context.Context) error {

	if err := commoninit.Run(ctx, nil); err != nil {
		return err
	}

	cfg, err := k8sutils.GetInClusterConfig()
	if err != nil {
		return err
	}

	k8sC, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}

	jwkCtl, err := jwkctl.NewJWKController(ctx, octeliumC, nil)
	if err != nil {
		return err
	}

	if err := jwkCtl.Run(ctx); err != nil {
		return err
	}

	region, err := octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{
		Name: vutils.GetMyRegionName(),
	})
	if err != nil {
		return err
	}

	ctl, err := wscontroller.NewController(ctx, ctx, octeliumC, k8sC, jwkCtl, umetav1.GetObjectReference(region))
	if err != nil {
		return err
	}

	usrCtl, err := usrcontroller.NewController(ctx, octeliumC)
	if err != nil {
		return err
	}

	tmplCtl, err := tmplcontroller.NewController(ctx, octeliumC)
	if err != nil {
		return err
	}

	if err := wswatchers.NewCordiumV1(octeliumC).Workspace(ctx, nil, ctl.OnAdd, ctl.OnUpdate, ctl.OnDelete); err != nil {
		return err
	}

	if err := wswatchers.NewCordiumV1(octeliumC).Template(ctx,
		nil, tmplCtl.OnAdd, tmplCtl.OnUpdate, tmplCtl.OnDelete); err != nil {
		return err
	}

	if err := watchers.NewCoreV1(octeliumC).User(ctx, nil, usrCtl.OnAdd, usrCtl.OnUpdate, usrCtl.OnDelete); err != nil {
		return err
	}

	watcher := newWatcher(octeliumC, k8sC, umetav1.GetObjectReference(region))
	if err := watcher.run(ctx); err != nil {
		return err
	}

	zap.L().Debug("Workspace controller is running...")

	healthcheck.Run(vutils.HealthCheckPortMain)
	<-ctx.Done()

	zap.L().Debug("Received a TERM signal. Shutting down the Workspace controller")

	ctl.WaitUntilAllWatchersClosed()

	return nil
}
