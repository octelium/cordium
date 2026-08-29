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

package main

import (
	"context"

	wsc "github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/genesis/genesis"
	"github.com/octelium/octelium/cluster/common/commoninit"
	"github.com/octelium/octelium/cluster/common/components"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:  "genesis",
	Long: `genesis`,
}

var initCmd = &cobra.Command{
	Use: "init",
	RunE: func(cmd *cobra.Command, args []string) error {

		g, err := genesis.NewGenesis()
		if err != nil {
			return err
		}

		if err := g.RunInit(context.Background(), &genesis.InitOpts{}); err != nil {
			return err
		}

		return nil
	},
}

var upgradeCmd = &cobra.Command{
	Use: "upgrade",
	RunE: func(cmd *cobra.Command, args []string) error {
		g, err := genesis.NewGenesis()
		if err != nil {
			return err
		}

		if err := g.RunUpgrade(context.Background(), &genesis.UpgradeOpts{}); err != nil {
			return err
		}

		return nil
	},
}

var cmdArgs args

type args struct {
	EnableSPIFFECSIDriver   bool
	SPIFFECSIDriver         string
	SPIFFETrustDomain       string
	EnableIngressFrontProxy bool
}

func setDeprecatedFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()

	f.BoolVar(&cmdArgs.EnableSPIFFECSIDriver, "enable-spiffe-csi", false, "Deprecated. Set via the bootstrap Config instead")
	f.StringVar(&cmdArgs.SPIFFECSIDriver, "spiffe-csi-driver", "", "Deprecated. Set via the bootstrap Config instead")
	f.StringVar(&cmdArgs.SPIFFETrustDomain, "spiffe-trust-domain", "", "Deprecated. Set via the bootstrap Config instead")
	f.BoolVar(&cmdArgs.EnableIngressFrontProxy, "ingress-front-proxy", false, "Deprecated. Set via the bootstrap Config instead")

	for _, name := range []string{"enable-spiffe-csi", "spiffe-csi-driver", "spiffe-trust-domain",
		"ingress-front-proxy"} {
		f.MarkHidden(name)
	}
}

func init() {
	setDeprecatedFlags(initCmd)
	setDeprecatedFlags(upgradeCmd)
}

func init() {
	components.SetComponentNamespace(wsc.ComponentNamespaceCordium)
	components.SetComponentType(wsc.Genesis)
}

func main() {
	components.RunComponent(func(ctx context.Context) error {
		rootCmd.AddCommand(initCmd)
		rootCmd.AddCommand(upgradeCmd)

		if err := commoninit.Run(ctx, nil); err != nil {
			return err
		}

		if err := rootCmd.Execute(); err != nil {
			return err
		}

		return nil
	}, nil)
}
