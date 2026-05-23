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

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"

	appsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (s *Genesis) doInit(ctx context.Context) error {

	zap.L().Debug("Starting initializing Cordium/k8s resources")

	if err := s.initSecret(ctx); err != nil {
		return err
	}

	/*
		if err := s.doInitRegistry(ctx); err != nil {
			return err
		}
	*/

	if err := s.setWSNamespace(ctx); err != nil {
		return errors.Errorf("Could not set Workspace k8s namespace: %+v", err)
	}

	/*
		if err := s.setPersistentVolume(ctx); err != nil {
			return errors.Errorf("Could not set default PersistentVolume: %+v", err)
		}
	*/

	if !ldflags.IsDev() {
		zap.L().Warn("Setting the NetworkPolicy for Workspace pods")
		if _, err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, s.k8sC, s.getWSNetworkPolicy()); err != nil {
			return err
		}
	} else {
		zap.L().Warn("Skipping setting the NetworkPolicy for Workspace pods")
	}

	zap.L().Debug("Resource initialization successfully done")

	return nil
}

func (s *Genesis) initSecret(ctx context.Context) error {
	_, err := s.octeliumC.CordiumC().GetSecret(ctx, &rmetav1.GetOptions{
		Name: "sys:ws-tunnel-wgkey",
	})
	if err == nil {
		return nil
	}

	if !grpcerr.IsNotFound(err) {
		return err
	}

	zap.L().Debug("Initializing a new wg tunnel key")

	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return err
	}

	_, err = s.octeliumC.CordiumC().CreateSecret(ctx, &cordiumv1.Secret{
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
		return err
	}

	return nil
}

func getResourceQuantity(arg string) *resource.Quantity {
	ret := resource.MustParse(arg)
	return &ret
}

func (s *Genesis) newDeploymentRegistry() *appsv1.Deployment {

	return &appsv1.Deployment{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      "workspace-registry",
			Namespace: vutils.K8sNS,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &k8smetav1.LabelSelector{
				MatchLabels: s.getRegistryLabels(),
			},
			Template: k8scorev1.PodTemplateSpec{
				ObjectMeta: k8smetav1.ObjectMeta{
					Labels: s.getRegistryLabels(),
				},
				Spec: k8scorev1.PodSpec{
					Volumes: []k8scorev1.Volume{
						{
							Name: "registry-conf",
							VolumeSource: k8scorev1.VolumeSource{
								Secret: &k8scorev1.SecretVolumeSource{
									SecretName: "workspace-registry-conf",
								},
							},
						},
						{
							Name: "registry-creds",
							VolumeSource: k8scorev1.VolumeSource{
								Secret: &k8scorev1.SecretVolumeSource{
									SecretName: "workspace-registry-creds",
								},
							},
						},
						{
							Name: "octelium",
							VolumeSource: k8scorev1.VolumeSource{
								EmptyDir: &k8scorev1.EmptyDirVolumeSource{
									SizeLimit: getResourceQuantity("25Gi"),
								},
							},
						},
					},
					Containers: []k8scorev1.Container{
						{
							Name:  "registry",
							Image: "registry:2",
							Args: []string{
								"serve",
								"/etc/octelium-conf.yaml",
							},
							VolumeMounts: []k8scorev1.VolumeMount{
								{
									Name:      "registry-conf",
									MountPath: "/etc/octelium-conf.yaml",
									ReadOnly:  true,
									SubPath:   "data",
								},

								{
									Name:      "registry-creds",
									MountPath: "/etc/octelium-auth",
									ReadOnly:  true,
									SubPath:   "data",
								},

								{
									Name:      "octelium",
									MountPath: "/octelium",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (s *Genesis) getRegistryLabels() map[string]string {
	return map[string]string{
		"app":                         "octelium",
		"octelium.com/component-type": "cluster",
		"octelium.com/component":      "workspace-registry",
	}
}

func (s *Genesis) setWSNamespace(ctx context.Context) error {
	zap.S().Debugf("Initializing the cordium k8s namespace")

	if _, err := s.k8sC.CoreV1().Namespaces().Get(ctx, workspacecommon.K8sNS, k8smetav1.GetOptions{}); err == nil {
		zap.L().Debug("octelium-ws k8s namespace is already created. Nothing to be done...")
		return nil
	} else if !k8serr.IsNotFound(err) {
		return err
	}

	zap.S().Debugf("Creating octelium-ws k8s namespace")
	_, err := s.k8sC.CoreV1().Namespaces().Create(ctx, &k8scorev1.Namespace{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name: workspacecommon.K8sNS,
			Labels: map[string]string{
				"app": "octelium",
			},
		},
		Spec: k8scorev1.NamespaceSpec{},
	}, k8smetav1.CreateOptions{})
	if err != nil {
		return err
	}

	/*
		if err := s.copyRegcred(ctx); err != nil {
			return err
		}
	*/

	return nil
}

func getCommonWorkspaceLabels() map[string]string {
	return map[string]string{
		"app":                         "octelium",
		"octelium.com/component-type": "user",
		"octelium.com/component":      "workspace",
	}
}

func (s *Genesis) getWSNetworkPolicy() *networkingv1.NetworkPolicy {

	var tcpProtocol = k8scorev1.ProtocolTCP
	var udpProtocol = k8scorev1.ProtocolUDP

	return &networkingv1.NetworkPolicy{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      "ws-main",
			Namespace: workspacecommon.K8sNS,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: k8smetav1.LabelSelector{
				MatchLabels: getCommonWorkspaceLabels(),
			},
			Ingress: []networkingv1.NetworkPolicyIngressRule{
				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcpProtocol,
							Port: &intstr.IntOrString{
								IntVal: 8080,
							},
						},
						{
							Protocol: &tcpProtocol,
							Port: &intstr.IntOrString{
								IntVal: 2022,
							},
						},
						{
							Protocol: &udpProtocol,
							Port: &intstr.IntOrString{
								IntVal: int32(workspacecommon.GetWorkspaceTunnelPort()),
							},
						},
					},
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &k8smetav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":                         "octelium",
									"octelium.com/component-type": "cluster",
								},
							},
							NamespaceSelector: &k8smetav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "octelium",
								},
							},
						},
						{
							// Genesis could run in default or octelium namespaces
							PodSelector: &k8smetav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":                    "octelium",
									"octelium.com/component": "genesis",
								},
							},
							NamespaceSelector: &k8smetav1.LabelSelector{},
						},
					},
				},
			},
			Egress: []networkingv1.NetworkPolicyEgressRule{
				{
					To: []networkingv1.NetworkPolicyPeer{
						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "0.0.0.0/0",
								Except: []string{
									"10.0.0.0/8",
									"172.16.0.0/12",
									"192.168.0.0/16",
									"100.64.0.0/10",
									"169.254.0.0/16",
								},
							},
						},

						{
							IPBlock: &networkingv1.IPBlock{
								CIDR: "::/0",
								Except: []string{
									"fc00::/7",
								},
							},
						},
					},
				},

				{

					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcpProtocol,
							Port: &intstr.IntOrString{
								IntVal: 8080,
							},
						},
					},

					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &k8smetav1.LabelSelector{
								MatchLabels: s.getRegistryLabels(),
							},

							NamespaceSelector: &k8smetav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": "octelium",
								},
							},
						},
					},
				},

				{
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &tcpProtocol,
							Port: &intstr.IntOrString{
								IntVal: 53,
							},
						},
						{
							Protocol: &udpProtocol,
							Port: &intstr.IntOrString{
								IntVal: 53,
							},
						},
					},
					To: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &k8smetav1.LabelSelector{
								MatchLabels: map[string]string{
									"k8s-app": "kube-dns",
								},
							},
							NamespaceSelector: &k8smetav1.LabelSelector{},
						},
						{
							PodSelector: &k8smetav1.LabelSelector{
								MatchLabels: map[string]string{
									"k8s-app": "coredns",
								},
							},
							NamespaceSelector: &k8smetav1.LabelSelector{},
						},
					},
				},
			},

			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}
}
