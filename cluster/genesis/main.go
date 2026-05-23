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

		if err := g.RunInit(context.Background(), &genesis.InitOpts{
			EnableSPIFFECSI:         cmdArgs.EnableSPIFFECSIDriver,
			SPIFFECSIDriver:         cmdArgs.SPIFFECSIDriver,
			SPIFFETrustDomain:       cmdArgs.SPIFFETrustDomain,
			EnableIngressFrontProxy: cmdArgs.EnableIngressFrontProxy,
		}); err != nil {
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

		if err := g.RunUpgrade(context.Background(), &genesis.UpgradeOpts{
			EnableSPIFFECSI:         cmdArgs.EnableSPIFFECSIDriver,
			SPIFFECSIDriver:         cmdArgs.SPIFFECSIDriver,
			SPIFFETrustDomain:       cmdArgs.SPIFFETrustDomain,
			EnableIngressFrontProxy: cmdArgs.EnableIngressFrontProxy,
		}); err != nil {
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

func init() {
	initCmd.PersistentFlags().BoolVar(&cmdArgs.EnableSPIFFECSIDriver, "enable-spiffe-csi", false, "Enable SPIFFE CSI Driver")
	initCmd.PersistentFlags().StringVar(&cmdArgs.SPIFFECSIDriver, "spiffe-csi-driver", "", "SPIFFE CSI Driver name")
	initCmd.PersistentFlags().StringVar(&cmdArgs.SPIFFETrustDomain, "spiffe-trust-domain", "", "SPIFFE trust domain")
	initCmd.PersistentFlags().BoolVar(&cmdArgs.EnableIngressFrontProxy, "ingress-front-proxy", false, "Enable Ingress front proxy mode")
}

func init() {
	upgradeCmd.PersistentFlags().BoolVar(&cmdArgs.EnableSPIFFECSIDriver, "enable-spiffe-csi", false, "Enable SPIFFE CSI Driver")
	upgradeCmd.PersistentFlags().StringVar(&cmdArgs.SPIFFECSIDriver, "spiffe-csi-driver", "", "SPIFFE CSI Driver name")
	upgradeCmd.PersistentFlags().StringVar(&cmdArgs.SPIFFETrustDomain, "spiffe-trust-domain", "", "SPIFFE trust domain")
	upgradeCmd.PersistentFlags().BoolVar(&cmdArgs.EnableIngressFrontProxy, "ingress-front-proxy", false, "Enable Ingress front proxy mode")
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
