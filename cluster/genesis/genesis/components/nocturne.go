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

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/cluster/common/k8sretry"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	gc "github.com/octelium/octelium/cluster/genesis/genesis/components"
	utils_types "github.com/octelium/octelium/pkg/utils/types"
	appsv1 "k8s.io/api/apps/v1"
	k8scorev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getNocturneDeployment(o *CommonOpts) *appsv1.Deployment {

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentNocturne),
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: nil,
			Selector: &metav1.LabelSelector{
				MatchLabels: getComponentLabels(componentNocturne),
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Template: k8scorev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      getComponentLabels(componentNocturne),
					Annotations: getAnnotations(),
				},
				Spec: k8scorev1.PodSpec{
					NodeSelector:                  getNodeSelectorControlPlane(o.ClusterConfig),
					ServiceAccountName:            getComponentName(componentNocturne),
					ImagePullSecrets:              getImagePullSecrets(),
					TerminationGracePeriodSeconds: utils_types.Int64ToPtr(800),

					Containers: []k8scorev1.Container{
						{
							Name:            componentNocturne,
							Resources:       getDefaultResourceRequirements(),
							Image:           components.GetImage(components.Nocturne, ""),
							ImagePullPolicy: k8sutils.GetImagePullPolicy(),
							LivenessProbe:   getDefaultLivenessProbe(),

							Env: func() []k8scorev1.EnvVar {
								ret := []k8scorev1.EnvVar{
									{
										Name:  "OCTELIUM_REGION_NAME",
										Value: o.Region.Metadata.Name,
									},
									{
										Name:  "OCTELIUM_REGION_UID",
										Value: o.Region.Metadata.Uid,
									},
								}

								/*
									if ldflags.IsDev() {
										ret = append(ret, k8scorev1.EnvVar{
											Name:  "GRPC_GO_LOG_VERBOSITY_LEVEL",
											Value: "99",
										})

										ret = append(ret, k8scorev1.EnvVar{
											Name:  "GRPC_GO_LOG_SEVERITY_LEVEL",
											Value: "info",
										})
									}
								*/

								return ret
							}(),
						},
					},
				},
			},
		},
	}

	gc.SetDeploymentSPIFFE(deployment, &o.CommonOpts)
	return deployment
}

func getNocturneRole() *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentNocturne),
			Namespace: workspacecommon.K8sNS,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
				Verbs:     []string{"create", "delete"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"configmaps/finalizers"},
				Verbs:     []string{"update"},
			},
			{
				APIGroups: []string{"apps"},
				Resources: []string{"deployments"},
				Verbs:     []string{"create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "create"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"pods"},
				Verbs:     []string{"list"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"persistentvolumeclaims"},
				Verbs:     []string{"get", "create", "update", "delete"},
			},
			{
				APIGroups: []string{"snapshot.storage.k8s.io"},
				Resources: []string{"volumesnapshots"},
				Verbs:     []string{"get", "list", "create", "delete"},
			},
		},
	}
}

func getNocturneServiceAccount() *k8scorev1.ServiceAccount {
	return &k8scorev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentNocturne),
			Namespace: ns,
		},
	}
}

func getNocturneRoleBinding() *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentNocturne),
			Namespace: workspacecommon.K8sNS,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     getComponentName(componentNocturne),
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      getComponentName(componentNocturne),
				Namespace: ns,
			},
		},
	}
}

func getNocturneNetworkPolicy(c *corev1.ClusterConfig) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getComponentName(componentNocturne),
			Namespace: ns,
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: getComponentLabels(componentNocturne),
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
			},
		},
	}
}

func CreateNocturne(ctx context.Context, o *CommonOpts) error {

	if _, err := k8sutils.CreateOrUpdateServiceAccount(ctx, o.K8sC, getNocturneServiceAccount()); err != nil {
		return err
	}

	if _, err := k8sutils.CreateOrUpdateRole(ctx, o.K8sC, getNocturneRole()); err != nil {
		return err
	}

	if _, err := k8sutils.CreateOrUpdateRoleBinding(ctx, o.K8sC, getNocturneRoleBinding()); err != nil {
		return err
	}

	if err := deleteNocturneLegacyClusterRole(ctx, o); err != nil {
		return err
	}

	if _, err := k8sutils.CreateOrUpdateDeployment(ctx, o.K8sC, getNocturneDeployment(o)); err != nil {
		return err
	}

	if _, err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, o.K8sC, getNocturneNetworkPolicy(o.ClusterConfig)); err != nil {
		return err
	}

	return nil
}

func deleteNocturneLegacyClusterRole(ctx context.Context, o *CommonOpts) error {
	name := getComponentName(componentNocturne)

	if err := k8sretry.Do(ctx, func(ctx context.Context) error {
		err := o.K8sC.RbacV1().ClusterRoleBindings().Delete(ctx, name, metav1.DeleteOptions{})
		if k8serr.IsNotFound(err) {
			return nil
		}
		return err
	}); err != nil {
		return err
	}

	return k8sretry.Do(ctx, func(ctx context.Context) error {
		err := o.K8sC.RbacV1().ClusterRoles().Delete(ctx, name, metav1.DeleteOptions{})
		if k8serr.IsNotFound(err) {
			return nil
		}
		return err
	})
}
