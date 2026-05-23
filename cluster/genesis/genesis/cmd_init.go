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
	"fmt"
	"os"
	"time"

	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/ovutils"
	oc "github.com/octelium/cordium/cluster/genesis/genesis/components"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	octeliumcinit "github.com/octelium/octelium/cluster/common/octeliumc"
	"github.com/octelium/octelium/cluster/common/vutils"
	gc "github.com/octelium/octelium/cluster/genesis/genesis/components"
	"github.com/octelium/octelium/cluster/genesis/genesis/genesisutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	utils_cert "github.com/octelium/octelium/pkg/utils/cert"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/structpb"
)

type InitOpts struct {
	EnableSPIFFECSI         bool
	SPIFFECSIDriver         string
	SPIFFETrustDomain       string
	EnableIngressFrontProxy bool
}

func (g *Genesis) RunInit(ctx context.Context, o *InitOpts) error {
	zap.L().Info("Starting initializing the Cluster")

	octeliumCInit, err := octeliumcinit.NewClient(ctx)
	if err != nil {
		return err
	}

	g.octeliumCInit = octeliumCInit

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

	octeliumC, err := octeliumc.NewClient(ctx, nil)
	if err != nil {
		return err
	}
	g.octeliumC = octeliumC

	if err := g.installSystemResources(ctx, region); err != nil {
		return errors.Errorf("Could not install system resources: %+v", err)
	}

	if err := g.doInit(ctx); err != nil {
		return errors.Errorf("Could not init Cordium specific components: %+v", err)
	}

	if err := g.setInitClusterCertificate(ctx, region); err != nil {
		return err
	}

	region, err = g.octeliumCInit.CoreC().GetRegion(ctx, &rmetav1.GetOptions{Name: regionName})
	if err != nil {
		return err
	}
	extInfo := &cordiumv1.RegionExtInfo{
		IsEnabled: true,
	}

	extInfoStruct, err := pbutils.MessageToStruct(extInfo)
	if err != nil {
		return err
	}
	if region.Status.Ext == nil {
		region.Status.Ext = make(map[string]*structpb.Struct)
	}
	region.Status.Ext[ovutils.ExtInfoLabel] = extInfoStruct

	if region.Metadata.SpecLabels == nil {
		region.Metadata.SpecLabels = make(map[string]string)
	}
	region.Metadata.SpecLabels["has-workspace"] = "true"
	region, err = g.octeliumCInit.CoreC().UpdateRegion(ctx, region)
	if err != nil {
		return err
	}

	if err := g.setRegionVersionMap(ctx, region); err != nil {
		zap.L().Warn("Could not setRegionVersionMap", zap.Error(err))
	}

	zap.L().Info("Successfully initialized the Cluster")

	return nil
}

func (g *Genesis) getOrCreateCordiumNamespace(ctx context.Context) (*corev1.Namespace, error) {
	ns, err := g.octeliumCInit.CoreC().GetNamespace(ctx, &rmetav1.GetOptions{Name: "cordium"})
	if err == nil {
		return ns, nil
	}
	if !grpcerr.IsNotFound(err) {
		return nil, err
	}

	zap.L().Debug("Creating Cordium Namespace")

	ns, err = g.octeliumCInit.CoreC().CreateNamespace(ctx, &corev1.Namespace{
		Metadata: &metav1.Metadata{
			Name:     "cordium",
			IsSystem: true,
		},
		Spec:   &corev1.Namespace_Spec{},
		Status: &corev1.Namespace_Status{},
	})
	if err != nil {
		return nil, err
	}
	return ns, nil
}

func (g *Genesis) installSystemResources(ctx context.Context, region *corev1.Region) error {
	zap.S().Debugf("Creating system resources")

	_, err := g.getOrCreateCordiumNamespace(ctx)
	if err != nil {
		return err
	}

	{
		svc := &corev1.Service{
			Metadata: &metav1.Metadata{
				Name:         fmt.Sprintf("%s-cordium.octelium-api", region.Metadata.Name),
				DisplayName:  "Cordium API Server",
				IsSystem:     true,
				IsUserHidden: true,
				SpecLabels: map[string]string{
					"enable-public": "true",
				},
				SystemLabels: map[string]string{
					"octelium-apiserver": "true",
					"apiserver-path":     "/octelium.api.main.cordium",
				},
			},
			Spec: &corev1.Service_Spec{
				Port:     8080,
				IsPublic: true,
				Mode:     corev1.Service_Spec_GRPC,

				Authorization: &corev1.Service_Spec_Authorization{
					InlinePolicies: []*corev1.InlinePolicy{
						{
							Spec: &corev1.Policy_Spec{
								Rules: []*corev1.Policy_Spec_Rule{
									{
										Effect: corev1.Policy_Spec_Rule_ALLOW,
										Condition: &corev1.Condition{
											Type: &corev1.Condition_Match{
												Match: `ctx.request.grpc.serviceFullName == "octelium.api.main.cordium.v1.MainService"`,
											},
										},
									},
								},
							},
						},
					},
				},
				Region: region.Metadata.Name,
			},
			Status: &corev1.Service_Status{
				ManagedService: &corev1.Service_Status_ManagedService{
					Image: ldflags.GetImage(components.CordiumComponent(components.APIServer), ""),
					Type:  "apiserver",
					// ImagePullSecret: "octelium-regcred",
					HealthCheck: &corev1.Service_Status_ManagedService_HealthCheck{
						Type: &corev1.Service_Status_ManagedService_HealthCheck_Grpc{
							Grpc: &corev1.Service_Status_ManagedService_HealthCheck_GRPC{
								Port: vutils.HealthCheckPortManagedService,
							},
						},
					},
				},
			},
		}

		if err := genesisutils.CreateOrUpdateService(ctx, g.octeliumCInit, svc); err != nil {
			return err
		}
	}

	{
		svc := &corev1.Service{
			Metadata: &metav1.Metadata{
				Name:        fmt.Sprintf("%s.cordium", region.Metadata.Name),
				IsSystem:    true,
				DisplayName: "Cordium Web Console",
			},
			Spec: &corev1.Service_Spec{
				Port:     8080,
				IsPublic: true,
				Mode:     corev1.Service_Spec_WEB,

				Region: region.Metadata.Name,
			},
			Status: &corev1.Service_Status{
				ManagedService: &corev1.Service_Status_ManagedService{
					Image:        ldflags.GetImage(components.CordiumComponent(components.Portal), ""),
					HasSubdomain: true,
					ForwardHost:  true,
					// ImagePullSecret: "octelium-regcred",
					HealthCheck: &corev1.Service_Status_ManagedService_HealthCheck{
						Type: &corev1.Service_Status_ManagedService_HealthCheck_Grpc{
							Grpc: &corev1.Service_Status_ManagedService_HealthCheck_GRPC{
								Port: vutils.HealthCheckPortManagedService,
							},
						},
					},
				},
			},
		}

		if err := genesisutils.CreateOrUpdateService(ctx, g.octeliumCInit, svc); err != nil {
			return err
		}
	}

	{
		svc := &corev1.Service{
			Metadata: &metav1.Metadata{
				Name:        fmt.Sprintf("%s-ssh.cordium", region.Metadata.Name),
				DisplayName: fmt.Sprintf(`Cordium SSH for Region "%s"`, region.Metadata.Name),
				IsSystem:    true,
			},
			Spec: &corev1.Service_Spec{
				Port: 22,
				Mode: corev1.Service_Spec_SSH,
				Config: &corev1.Service_Spec_Config{
					Type: &corev1.Service_Spec_Config_Ssh{
						Ssh: &corev1.Service_Spec_Config_SSH{
							EnableLocalPortForwarding: true,
							EnableSubsystem:           true,
						},
					},
				},
				Authorization: &corev1.Service_Spec_Authorization{
					InlinePolicies: []*corev1.InlinePolicy{
						{
							Spec: &corev1.Policy_Spec{
								Rules: []*corev1.Policy_Spec_Rule{
									{
										Effect: corev1.Policy_Spec_Rule_ALLOW,
										Condition: &corev1.Condition{
											Type: &corev1.Condition_MatchAny{
												MatchAny: true,
											},
										},
									},
								},
							},
						},
					},
				},

				Region: region.Metadata.Name,
			},
			Status: &corev1.Service_Status{
				ManagedService: &corev1.Service_Status_ManagedService{
					Type:  "vigil",
					Image: ldflags.GetImage(components.CordiumComponent(components.Vigil), ""),
					// ImagePullSecret: "octelium-regcred",
				},
			},
		}

		if err := genesisutils.CreateOrUpdateService(ctx, g.octeliumCInit, svc); err != nil {
			return err
		}
	}

	return nil
}

/*
func (g *Genesis) copyRegcred(ctx context.Context) error {
	if !ovutils.IsPrivateRegistry() {
		return nil
	}

	zap.S().Debugf("Copying regcred")

	regcred, err := g.k8sC.CoreV1().Secrets("octelium").Get(ctx, "octelium-regcred", k8smetav1.GetOptions{})
	if err != nil {
		return err
	}

	sec := &k8scorev1.Secret{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      regcred.Name,
			Namespace: workspacecommon.K8sNS,
		},
		Data:       regcred.Data,
		StringData: regcred.StringData,
		Type:       regcred.Type,
	}

	_, err = g.k8sC.CoreV1().Secrets(workspacecommon.K8sNS).Create(ctx, sec, k8smetav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}
*/

func (g *Genesis) setInitClusterCertificate(ctx context.Context, region *corev1.Region) error {

	zap.S().Debugf("Setting initial Cluster Certificate")
	clusterCfg, err := g.octeliumCInit.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	domain := fmt.Sprintf("*.%s.cordium.%s", region.Metadata.Name, clusterCfg.Status.Domain)
	var sans []string

	if region.Metadata.Name == "default" {
		sans = append(sans,
			fmt.Sprintf("*.cordium.%s", clusterCfg.Status.Domain))
	}

	initCrt, err := utils_cert.GenerateSelfSignedCert(domain, sans, 4*12*30*24*time.Hour)
	if err != nil {
		return err
	}

	crtPEM, err := initCrt.GetCertPEM()
	if err != nil {
		return err
	}

	privPEM, err := initCrt.GetPrivateKeyPEM()
	if err != nil {
		return err
	}

	crt := &corev1.Secret{
		Metadata: &metav1.Metadata{
			Name:           "crt-svc-default-cordium",
			IsSystem:       true,
			IsSystemHidden: true,
			IsUserHidden:   true,
			SystemLabels: map[string]string{
				"octelium-cert": "true",
			},
		},

		Spec: &corev1.Secret_Spec{
			Data: &corev1.Secret_Spec_Data{
				Type: &corev1.Secret_Spec_Data_Value{
					Value: crtPEM,
				},
			},
		},
		Status: &corev1.Secret_Status{},
		Data: &corev1.Secret_Data{
			Type: &corev1.Secret_Data_Value{
				Value: privPEM,
			},
		},
	}

	_, err = g.octeliumC.CoreC().CreateSecret(ctx, crt)
	if err != nil {
		if grpcerr.AlreadyExists(err) {
			return nil
		}
		return err
	}

	return nil
}
