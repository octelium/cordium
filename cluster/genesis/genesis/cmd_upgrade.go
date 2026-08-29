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

	"github.com/octelium/cordium/cluster/common/octeliumc"
	oc "github.com/octelium/cordium/cluster/genesis/genesis/components"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	gc "github.com/octelium/octelium/cluster/genesis/genesis/components"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type UpgradeOpts struct{}

func (g *Genesis) RunUpgrade(ctx context.Context, o *UpgradeOpts) error {

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}

	g.octeliumC = octeliumC
	g.octeliumCInit = octeliumC

	regionName := os.Getenv("OCTELIUM_REGION_NAME")
	if regionName == "" {
		return errors.Errorf("Could not start upgrade. Empty region name.")
	}

	zap.L().Debug("Starting Cordium upgrade", zap.String("region", regionName))

	regionV, err := g.octeliumC.CoreC().GetRegion(ctx, &rmetav1.GetOptions{Name: regionName})
	if err != nil {
		return err
	}

	clusterCfg, err := g.octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	if err := g.installComponents(ctx, &oc.CommonOpts{
		CommonOpts: gc.CommonOpts{
			K8sC:          g.k8sC,
			ClusterConfig: clusterCfg,
			Region:        regionV,
		},
	}); err != nil {
		return errors.Errorf("Could not install components: %+v", err)
	}

	if err := g.installSystemResources(ctx, regionV); err != nil {
		return errors.Errorf("Could not install system resources: %+v", err)
	}

	if err := g.doInit(ctx); err != nil {
		return errors.Errorf("Could not init Cordium specific components: %+v", err)
	}

	if err := g.setRegionVersionMap(ctx, regionV); err != nil {
		zap.L().Warn("Could not setRegionVersionMap", zap.Error(err))
	}

	zap.L().Debug("Upgrade successful")

	return nil
}
