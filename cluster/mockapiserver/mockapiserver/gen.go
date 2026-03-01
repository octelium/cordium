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

package apiserver

import (
	"context"
	"fmt"
	"time"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/cluster/cclusterv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/sessionc"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	utils_cert "github.com/octelium/octelium/pkg/utils/cert"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/protobuf/types/known/structpb"
)

func genResources(ctx context.Context, octeliumC octeliumc.ClientInterface) error {

	clusterCfg, err := octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	cc, err := octeliumC.CoreV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return err
	}

	namespaces := []string{
		"production",
		"db",
		"ssh",
		"aws",
	}

	{
		cc := clusterCfg

		if cc.Spec.Space == nil {
			cc.Spec.Space = &cordiumv1.ClusterConfig_Spec_Space{}
		}

		cc.Spec.Space = &cordiumv1.ClusterConfig_Spec_Space{
			Ownership: &cordiumv1.ClusterConfig_Spec_Space_Ownership{
				Rules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
					{
						Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,

						Condition: &cordiumv1.Condition{
							Type: &cordiumv1.Condition_MatchAny{
								MatchAny: true,
							},
						},
					},
				},
				/*
					OrganizationRules: []*cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule{
						{
							Effect: cordiumv1.ClusterConfig_Spec_Space_Ownership_Rule_ALLOW,
							Conditions: []*metav1.Condition{
								{
									Type: &metav1.Condition_MatchAny{
										MatchAny: true,
									},
								},
							},
						},
					},
				*/
			},
		}

		_, err = octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
		if err != nil {
			return err
		}
	}

	{
		{
			domain := cc.Status.Domain
			sans := []string{
				"localhost",
				fmt.Sprintf("*.%s", domain),
				fmt.Sprintf("*.local.%s", domain),
				fmt.Sprintf("*.s.%s", domain),
			}

			initCrt, err := utils_cert.GenerateSelfSignedCert(domain, sans, 3*time.Hour)
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

			domains := []string{
				domain,
			}
			domains = append(domains, sans...)

			crt := &corev1.Secret{
				Metadata: &metav1.Metadata{
					Name:           "sys:cert-cluster",
					IsSystem:       true,
					IsSystemHidden: true,
					IsUserHidden:   true,
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

			_, err = octeliumC.CoreC().CreateSecret(ctx, crt)
			if err != nil {
				return err
			}
		}

		{

			region := &corev1.Region{
				Metadata: &metav1.Metadata{
					Name: "default",
					SpecLabels: map[string]string{
						"has-workspace": "true",
					},
				},
				Spec:   &corev1.Region_Spec{},
				Status: &corev1.Region_Status{},
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

			_, err = octeliumC.CoreC().CreateRegion(ctx, region)
			if err != nil {
				return err
			}
		}

		{
			key, err := wgtypes.GeneratePrivateKey()
			if err != nil {
				return err
			}

			_, err = octeliumC.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
				Metadata: &metav1.Metadata{
					Name:     "sys:ws-tunnel-wgkey",
					IsSystem: true,
				},
				Spec:   &cordiumv1.Secret_Spec{},
				Status: &cordiumv1.Secret_Status{},
				Data: &cordiumv1.Secret_Data{
					Type: &cordiumv1.Secret_Data_Value{
						Value: key.String(),
					},
				},
			})
			if err != nil {
				return err
			}
		}

		{

			secretVal, err := utilrand.GetRandomBytes(32)
			if err != nil {
				return err
			}
			secret := &corev1.Secret{
				Metadata: &metav1.Metadata{
					Name: fmt.Sprintf("sys:aes256-key-%s", utilrand.GetRandomStringLowercase(8)),
					SystemLabels: map[string]string{
						"aes256-key": "true",
					},
					IsSystem:       true,
					IsSystemHidden: true,
					IsUserHidden:   true,
				},

				Spec:   &corev1.Secret_Spec{},
				Status: &corev1.Secret_Status{},

				Data: &corev1.Secret_Data{
					Type: &corev1.Secret_Data_ValueBytes{
						ValueBytes: secretVal,
					},
				},
			}

			if _, err := octeliumC.CoreC().CreateSecret(ctx, secret); err != nil {
				return err
			}
		}

		/*
			{
				secretVal := utilrand.GetRandomString(24)

				secret := &cordiumv1.Secret{
					Metadata: &metav1.Metadata{
						Name: "sys:internal-registry-password",

						IsSystem:       true,
						IsSystemHidden: true,
						IsUserHidden:   true,
					},

					Spec:   &cordiumv1.Secret_Spec{},
					Status: &cordiumv1.Secret_Status{},

					Data: &cordiumv1.Secret_Data{
						Type: &cordiumv1.Secret_Data_Value{
							Value: secretVal,
						},
					},
				}

				if _, err := octeliumC.CordiumC().CreateSecret(ctx, secret); err != nil {
					return err
				}
			}
		*/

		/*
			{
				if _, err := octeliumC.CoreC().CreateAuthenticationMethod(ctx, &corev1.AuthenticationMethod{
					Metadata: &metav1.Metadata{
						Name: "totp-1",
					},
					Spec: &corev1.AuthenticationMethod_Spec{
						Type: &corev1.AuthenticationMethod_Spec_Totp{
							Totp: &corev1.AuthenticationMethod_Spec_TOTP{},
						},
					},
					Status: &corev1.AuthenticationMethod_Status{
						Type: corev1.AuthenticationMethod_Status_TOTP,
					},
				}); err != nil {
					return err
				}
			}
		*/

		for _, ns := range namespaces {
			if _, err := octeliumC.CoreC().CreateNamespace(ctx, &corev1.Namespace{
				Metadata: &metav1.Metadata{
					Name: ns,
				},
				Spec: &corev1.Namespace_Spec{},
			}); err != nil {
				return err
			}
		}

		/*
			{
				if _, err := octeliumC.CoreC().CreateAuthenticationMethod(ctx, &corev1.AuthenticationMethod{
					Metadata: &metav1.Metadata{
						Name: "webauthn-1",
					},
					Spec: &corev1.AuthenticationMethod_Spec{
						Type: &corev1.AuthenticationMethod_Spec_Webauthn_{
							Webauthn: &corev1.AuthenticationMethod_Spec_Webauthn{
								AuthenticatorType: corev1.AuthenticationMethod_Spec_Webauthn_ROAMING,
							},
						},
					},
					Status: &corev1.AuthenticationMethod_Status{
						Type: corev1.AuthenticationMethod_Status_WEBAUTHN,
					},
				}); err != nil {
					return err
				}
			}
		*/

		/*
			{
				if _, err := octeliumC.CordiumC().CreateCloudProvider(ctx, &cordiumv1.CloudProvider{
					Metadata: &metav1.Metadata{
						Name:           "sys:internal-registry",
						IsSystem:       true,
						IsSystemHidden: true,
						IsUserHidden:   true,
					},
					Spec: &cordiumv1.CloudProvider_Spec{
						Type: &cordiumv1.CloudProvider_Spec_ContainerRegistry_{
							ContainerRegistry: &cordiumv1.CloudProvider_Spec_ContainerRegistry{
								Server:   workspacecommon.GetInternalRegistryAddr(),
								NoTLS:    true,
								Username: "octelium",
								Password: &cordiumv1.CloudProvider_Spec_ContainerRegistry_Password{
									Type: &cordiumv1.CloudProvider_Spec_ContainerRegistry_Password_FromSecret{
										FromSecret: "sys:internal-registry-password",
									},
								},
							},
						},
					},
				}); err != nil {
					return err
				}
			}
		*/

		{
			ecdsaKey, err := utils_cert.GenerateECDSA()
			if err != nil {
				return err
			}

			privPEM, err := ecdsaKey.GetPrivateKeyPEM()
			if err != nil {
				return err
			}

			_, err = octeliumC.CoreC().CreateSecret(ctx, &corev1.Secret{
				Metadata: &metav1.Metadata{
					Name:           "sys:ssh-ca",
					IsSystem:       true,
					IsSystemHidden: true,
					IsUserHidden:   true,
				},
				Spec:   &corev1.Secret_Spec{},
				Status: &corev1.Secret_Status{},
				Data: &corev1.Secret_Data{
					Type: &corev1.Secret_Data_ValueBytes{
						ValueBytes: []byte(privPEM),
					},
				},
			})
			if err != nil {
				return err
			}
		}

		_, err = octeliumC.CoreC().CreateUser(ctx, &corev1.User{
			Metadata: &metav1.Metadata{
				Name:     "root",
				IsSystem: true,
			},
			Spec: &corev1.User_Spec{
				Type: corev1.User_Spec_WORKLOAD,
			},
		})
		if err != nil {
			return err
		}

		{

			attrs, err := pbutils.MessageToStruct(&cclusterv1.ClusterConnInfo{})
			if err != nil {
				return err
			}
			_, err = octeliumC.CoreC().CreateConfig(ctx, &corev1.Config{
				Metadata: &metav1.Metadata{
					Name:     "sys:conn-info",
					IsSystem: true,
				},
				Spec:   &corev1.Config_Spec{},
				Status: &corev1.Config_Status{},
				Data: &corev1.Config_Data{
					Type: &corev1.Config_Data_Attrs{
						Attrs: attrs,
					},
				},
			})
			if err != nil {
				return err
			}
		}

		{

			_, err := octeliumC.CoreC().CreateNamespace(ctx, &corev1.Namespace{
				Metadata: &metav1.Metadata{
					Name:     "default",
					IsSystem: true,
				},
				Spec:   &corev1.Namespace_Spec{},
				Status: &corev1.Namespace_Status{},
			})
			if err != nil {
				return err
			}

			octeliumNs, err := octeliumC.CoreC().CreateNamespace(ctx, &corev1.Namespace{
				Metadata: &metav1.Metadata{
					Name:     "octelium",
					IsSystem: true,
				},
				Spec:   &corev1.Namespace_Spec{},
				Status: &corev1.Namespace_Status{},
			})
			if err != nil {
				return err
			}

			_, err = octeliumC.CoreC().CreateNamespace(ctx, &corev1.Namespace{
				Metadata: &metav1.Metadata{
					Name:     "cordium",
					IsSystem: true,
				},
				Spec:   &corev1.Namespace_Spec{},
				Status: &corev1.Namespace_Status{},
			})
			if err != nil {
				return err
			}
			_, err = octeliumC.CoreC().CreateService(ctx, &corev1.Service{
				Metadata: &metav1.Metadata{
					Name:         "dns.octelium",
					IsSystem:     true,
					IsUserHidden: true,
				},
				Spec: &corev1.Service_Spec{
					Port: 53,
					Mode: corev1.Service_Spec_DNS,
					Config: &corev1.Service_Spec_Config{
						Upstream: &corev1.Service_Spec_Config_Upstream{
							Type: &corev1.Service_Spec_Config_Upstream_Url{
								Url: "dns://octelium-dnsserver.octelium.svc",
							},
						},
					},
				},
				Status: &corev1.Service_Status{
					NamespaceRef: umetav1.GetObjectReference(octeliumNs),
					Addresses: []*corev1.Service_Status_Address{
						{
							DualStackIP: &metav1.DualStackIP{
								Ipv4: "1.1.1.1",
							},
						},
					},
				},
			})
			if err != nil {
				return err
			}

			{
				svc := &corev1.Service{
					Metadata: &metav1.Metadata{
						Name:         "default.octelium-api",
						IsSystem:     true,
						IsUserHidden: true,
						SpecLabels: map[string]string{
							"enable-public": "true",
						},
					},
					Spec: &corev1.Service_Spec{
						Port:     8080,
						IsPublic: true,
						Mode:     corev1.Service_Spec_GRPC,
					},
					Status: &corev1.Service_Status{
						NamespaceRef: &metav1.ObjectReference{
							Name: octeliumNs.Metadata.Name,
							Uid:  octeliumNs.Metadata.Uid,
						},
					},
				}

				if _, err := octeliumC.CoreC().CreateService(ctx, svc); err != nil {
					return err
				}
			}
		}
	}

	zap.L().Debug("Starting generating resources")

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  octeliumC,
		IsEmbedded: true,
	})

	displayNames := []string{
		"",
		"Main SSH Server",
		"AWS K8s Cluster",
		"Postgres Database",
		"",
		"My API",
	}

	nSvc := utilrand.GetRandomRangeMath(50, 140)

	namespaces = append(namespaces, "default")
	for i := 0; i < nSvc; i++ {
		req := &corev1.Service{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8),
					namespaces[utilrand.GetRandomRangeMath(0, len(namespaces)-1)]),
				DisplayName: displayNames[utilrand.GetRandomRangeMath(0, len(displayNames)-1)],
			},
			Spec: &corev1.Service_Spec{
				Mode: corev1.Service_Spec_Mode(utilrand.GetRandomRangeMath(1, 11)),
				Port: uint32(utilrand.GetRandomRangeMath(1024, 10000)),
				Config: &corev1.Service_Spec_Config{
					Upstream: &corev1.Service_Spec_Config_Upstream{
						Type: &corev1.Service_Spec_Config_Upstream_Url{
							Url: "https://example.com",
						},
					},
				},
				IsTLS: utilrand.GetRandomRangeMath(0, 1)%2 == 0,
			},
		}

		if utilrand.GetRandomRangeMath(0, 4)%4 == 0 {
			req.Spec.Mode = corev1.Service_Spec_WEB
		}

		if req.Spec.Mode == corev1.Service_Spec_WEB {
			req.Spec.IsPublic = true
		}

		svc, err := adminSrv.CreateService(ctx, req)
		if err != nil {
			return err
		}

		svc.Status.Addresses = []*corev1.Service_Status_Address{
			{
				DualStackIP: &metav1.DualStackIP{
					Ipv4: "1.2.3.4",
					Ipv6: "fd::1",
				},
			},
		}

		_, err = octeliumC.CoreC().UpdateService(ctx, svc)
		if err != nil {
			return err
		}

	}
	for i := 0; i < 20; i++ {
		usr, err := adminSrv.CreateUser(ctx, &corev1.User{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &corev1.User_Spec{
				Type: corev1.User_Spec_HUMAN,
			},
		})
		if err != nil {
			return err
		}

		for x := 0; x < 2; x++ {
			dev, err := octeliumC.CoreC().CreateDevice(ctx, &corev1.Device{
				Metadata: &metav1.Metadata{
					Name: utilrand.GetRandomStringCanonical(6),
				},
				Spec: &corev1.Device_Spec{
					State: corev1.Device_Spec_ACTIVE,
				},
				Status: &corev1.Device_Status{
					UserRef: umetav1.GetObjectReference(usr),
					OsType:  corev1.Device_Status_LINUX,
				},
			})
			if err != nil {
				return err
			}

			for j := 0; j < 2; j++ {
				_, err := sessionc.CreateSession(ctx, &sessionc.CreateSessionOpts{
					Usr:       usr,
					Device:    dev,
					OcteliumC: octeliumC,
					SessType:  corev1.Session_Status_CLIENT,
				})
				if err != nil {
					return err
				}
			}
		}

	}

	zap.L().Debug("Done generating resources")

	return nil
}
