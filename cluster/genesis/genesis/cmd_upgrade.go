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

type UpgradeOpts struct {
	EnableSPIFFECSI         bool
	SPIFFECSIDriver         string
	SPIFFETrustDomain       string
	EnableIngressFrontProxy bool
}

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

	if err := g.installComponents(ctx, &oc.CommonOpts{
		CommonOpts: gc.CommonOpts{
			EnableSPIFFECSI:         o.EnableSPIFFECSI,
			EnableIngressFrontProxy: o.EnableIngressFrontProxy,
			SPIFFECSIDriver:         o.SPIFFECSIDriver,
			SPIFFETrustDomain:       o.SPIFFETrustDomain,
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

	zap.L().Debug("Upgrade successful")

	return nil
}
