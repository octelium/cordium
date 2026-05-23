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

package tests

import (
	"context"

	"github.com/google/uuid"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcordiumv1"
	"github.com/octelium/octelium/apis/rsc/rcorev1"
	ot "github.com/octelium/octelium/cluster/common/tests"
	"github.com/octelium/octelium/cluster/rscserver/rscserver"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
)

type Opts struct {
}

type T struct {
	C     *FakeClient
	inner *ot.T
}

type FakeClient struct {
	K8sC      kubernetes.Interface
	OcteliumC octeliumc.ClientInterface
}

func (t *T) Destroy() error {
	return t.inner.Destroy()
}

func Initialize(o *Opts) (*T, error) {

	ctx := context.Background()

	var rscs []umetav1.ResourceObjectI
	{
		clusterCfg := &cordiumv1.ClusterConfig{
			ApiVersion: "cordium/v1",
			Kind:       "ClusterConfig",
			Metadata: &metav1.Metadata{
				Uid:             uuid.New().String(),
				ResourceVersion: uuid.New().String(),
				Name:            "default",
			},
			Spec:   &cordiumv1.ClusterConfig_Spec{},
			Status: &cordiumv1.ClusterConfig_Status{},
		}

		rscs = append(rscs, clusterCfg)
	}

	inner, err := ot.Initialize(&ot.Opts{
		PreCreatedResources: rscs,
		RscServerOpts: &rscserver.Opts{
			RegisterResourceFn: func(s grpc.ServiceRegistrar) error {
				rcorev1.RegisterResourceServiceServer(s, &struct {
					rcorev1.UnimplementedResourceServiceServer
				}{})
				rcordiumv1.RegisterResourceServiceServer(s, &struct {
					rcordiumv1.UnimplementedResourceServiceServer
				}{})

				return nil
			},

			NewResourceObject:     ovutils.NewResourceObject,
			NewResourceObjectList: ovutils.NewResourceObjectList,
		},
	})
	if err != nil {
		return nil, err
	}

	octeliumC, err := octeliumc.NewClient(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	{
		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, err
		}

		_, err = octeliumC.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
			Metadata: &metav1.Metadata{
				Name:           "sys:ws-tunnel-wgkey",
				IsSystem:       true,
				IsSystemHidden: true,
				IsUserHidden:   true,
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
			return nil, err
		}
	}

	/*
		{
			secretVal := utilrand.GetRandomString(24)

			secret := &cordiumv1.Secret{
				Metadata: &metav1.Metadata{
					Name:           "sys:internal-registry-password",
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
				return nil, err
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
				return nil, err
			}
		}
	*/

	return &T{
		inner: inner,
		C: &FakeClient{
			K8sC:      inner.C.K8sC,
			OcteliumC: octeliumC,
		},
	}, nil
}
