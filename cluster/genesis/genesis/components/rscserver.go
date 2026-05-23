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

package components

import (
	"context"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"github.com/octelium/octelium/cluster/common/vutils"
	gc "github.com/octelium/octelium/cluster/genesis/genesis/components"
	utils_types "github.com/octelium/octelium/pkg/utils/types"
	"github.com/octelium/cordium/cluster/common/components"
	appsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func getRscServerService() *k8scorev1.Service {

	ret := &k8scorev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentRscServer),
			Namespace: ns,
			Labels:    getComponentLabels(componentRscServer),
		},
		Spec: k8scorev1.ServiceSpec{
			Type:     k8scorev1.ServiceTypeClusterIP,
			Selector: getComponentLabels(componentRscServer),
			Ports: []k8scorev1.ServicePort{
				{
					Protocol: k8scorev1.ProtocolTCP,
					Port:     8080,
					TargetPort: intstr.IntOrString{
						Type:   intstr.Int,
						IntVal: 8080,
					},
				},
			},
		},
	}

	return ret
}

func getRscServerDeployment(o *CommonOpts) *appsv1.Deployment {

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentRscServer),
			Namespace: ns,
			Labels:    getComponentLabels(componentRscServer),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: nil,
			Selector: &metav1.LabelSelector{
				MatchLabels: getComponentLabels(componentRscServer),
			},
			Template: k8scorev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      getComponentLabels(componentRscServer),
					Annotations: getAnnotations(),
				},
				Spec: k8scorev1.PodSpec{
					AutomountServiceAccountToken: utils_types.BoolToPtr(false),
					NodeSelector:                 getNodeSelectorControlPlane(o.ClusterConfig),
					ImagePullSecrets:             getImagePullSecrets(),

					Containers: []k8scorev1.Container{
						{
							Name: componentRscServer,
							Env: func() []k8scorev1.EnvVar {
								var ret []k8scorev1.EnvVar
								ret = append(ret, gc.GetPostgresEnv()...)
								ret = append(ret, gc.GetRedisEnv()...)

								return ret
							}(),
							ReadinessProbe: &k8scorev1.Probe{
								InitialDelaySeconds: 5,
								TimeoutSeconds:      4,
								PeriodSeconds:       20,
								FailureThreshold:    3,
								ProbeHandler: k8scorev1.ProbeHandler{
									GRPC: &k8scorev1.GRPCAction{
										Port: int32(vutils.HealthCheckPortMain),
									},
								},
							},

							LivenessProbe: getDefaultLivenessProbe(),

							Image:           components.GetImage(components.RscServer, ""),
							ImagePullPolicy: k8sutils.GetImagePullPolicy(),

							Resources: k8scorev1.ResourceRequirements{
								Requests: getDefaultRequests(),
								Limits: k8scorev1.ResourceList{
									k8scorev1.ResourceMemory: resource.MustParse("1200Mi"),
									k8scorev1.ResourceCPU:    resource.MustParse("1500m"),
								},
							},
						},
					},
				},
			},
		},
	}

	gc.SetDeploymentSPIFFE(deployment, &o.CommonOpts)
	return deployment
}

func getRscServerNetworkPolicy(c *corev1.ClusterConfig) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentRscServer),
			Namespace: ns,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: getComponentLabels(componentRscServer),
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
					},
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":                         "octelium",
									"octelium.com/component-type": "cluster",
								},
							},
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"kubernetes.io/metadata.name": ns,
								},
							},
						},
						{
							// Genesis could run in default or octelium namespaces
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"app":                    "octelium",
									"octelium.com/component": "genesis",
								},
							},
							NamespaceSelector: &metav1.LabelSelector{},
						},
					},
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}
}

func CreateRscServer(ctx context.Context, o *CommonOpts) error {

	if _, err := k8sutils.CreateOrUpdateDeployment(ctx, o.K8sC, getRscServerDeployment(o)); err != nil {
		return err
	}

	if _, err := k8sutils.CreateOrUpdateService(ctx, o.K8sC, getRscServerService()); err != nil {
		return err
	}

	if _, err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, o.K8sC, getRscServerNetworkPolicy(o.ClusterConfig)); err != nil {
		return err
	}

	return nil
}
