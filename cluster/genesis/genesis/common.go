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

package genesis

import (
	"context"
	"os"

	wsc "github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/genesis/genesis/components"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"go.uber.org/zap"
)

func (g *Genesis) installComponents(ctx context.Context, o *components.CommonOpts) error {
	zap.S().Debugf("Installing components...")
	clusterCfg, err := g.octeliumCInit.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	if o == nil {
		o = &components.CommonOpts{}
	}
	o.K8sC = g.k8sC
	o.ClusterConfig = clusterCfg
	if o.OcteliumC == nil {
		if g.octeliumC != nil {
			o.OcteliumC = g.octeliumC
		} else if g.octeliumCInit != nil {
			o.OcteliumC = g.octeliumCInit
		}
	}
	if o.Region == nil {
		regionName := func() string {
			if os.Getenv("OCTELIUM_REGION_NAME") != "" {
				return os.Getenv("OCTELIUM_REGION_NAME")
			}
			return "default"
		}()

		region, err := g.octeliumCInit.CoreC().GetRegion(ctx, &rmetav1.GetOptions{Name: regionName})
		if err != nil {
			return err
		}
		o.Region = region
	}

	if err := components.CreateRscServer(ctx, o); err != nil {
		return err
	}

	if err := k8sutils.WaitReadinessDeployment(ctx, g.k8sC, wsc.CordiumComponent(wsc.RscServer)); err != nil {
		return err
	}

	if err := components.CreateNocturne(ctx, o); err != nil {
		return err
	}

	return nil
}

func (g *Genesis) setRegionVersionMap(ctx context.Context, rgn *corev1.Region) error {
	region, err := g.octeliumC.CoreC().GetRegion(ctx, apivalidation.ObjectToRGetOptions(rgn))
	if err != nil {
		return err
	}

	if region.Status.VersionInfoMap == nil {
		region.Status.VersionInfoMap = make(map[string]*corev1.Region_Status_VersionInfo)
	}

	region.Status.VersionInfoMap["cordium"] = &corev1.Region_Status_VersionInfo{
		Package: "cordium",
		SetAt:   pbutils.Now(),
		Version: ldflags.GetVersion(),
		Id:      os.Getenv("OCTELIUM_INSTALL_ID"),
	}

	_, err = g.octeliumC.CoreC().UpdateRegion(ctx, region)
	if err != nil {
		return err
	}

	return nil
}
