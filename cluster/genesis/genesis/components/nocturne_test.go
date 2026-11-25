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
	"slices"
	"testing"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	gc "github.com/octelium/octelium/cluster/genesis/genesis/components"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8scorev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestNocturneRBAC(t *testing.T) {
	role := getNocturneRole()
	assert.Equal(t, "cordium-nocturne", role.Name)
	assert.Equal(t, "cordium", role.Namespace)
	assert.Equal(t, []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"create", "delete"}},
		{APIGroups: []string{""}, Resources: []string{"configmaps/finalizers"}, Verbs: []string{"update"}},
		{APIGroups: []string{"apps"}, Resources: []string{"deployments"}, Verbs: []string{"create"}},
		{APIGroups: []string{""}, Resources: []string{"services"}, Verbs: []string{"get", "create"}},
		{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"list"}},
		{APIGroups: []string{""}, Resources: []string{"persistentvolumeclaims"}, Verbs: []string{"get", "create", "update", "delete"}},
		{APIGroups: []string{"snapshot.storage.k8s.io"}, Resources: []string{"volumesnapshots"}, Verbs: []string{"get", "list", "create", "delete"}},
	}, role.Rules)
	for _, rule := range role.Rules {
		assert.NotContains(t, rule.APIGroups, "*")
		assert.NotContains(t, rule.Resources, "*")
		assert.NotContains(t, rule.Verbs, "*")
	}

	binding := getNocturneRoleBinding()
	assert.Equal(t, "cordium", binding.Namespace)
	assert.Equal(t, rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "cordium-nocturne"}, binding.RoleRef)
	assert.Equal(t, []rbacv1.Subject{{Kind: "ServiceAccount", Name: "cordium-nocturne", Namespace: "octelium"}}, binding.Subjects)
}

func TestCreateNocturne(t *testing.T) {
	wildcard := []rbacv1.PolicyRule{{APIGroups: []string{"*", "*.*"}, Resources: []string{"*"}, Verbs: []string{"*"}}}
	for _, tc := range []struct {
		name     string
		existing []runtime.Object
	}{
		{name: "Fresh"},
		{name: "Upgrade", existing: []runtime.Object{
			&k8scorev1.ServiceAccount{ObjectMeta: k8smetav1.ObjectMeta{Name: "cordium-nocturne", Namespace: "octelium"}},
			&rbacv1.ClusterRole{ObjectMeta: k8smetav1.ObjectMeta{Name: "cordium-nocturne"}, Rules: wildcard},
			&rbacv1.ClusterRoleBinding{
				ObjectMeta: k8smetav1.ObjectMeta{Name: "cordium-nocturne"},
				RoleRef:    rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: "cordium-nocturne"},
				Subjects:   []rbacv1.Subject{{Kind: "ServiceAccount", Name: "cordium-nocturne", Namespace: "octelium"}},
			},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			client := fake.NewClientset(tc.existing...)
			opts := &CommonOpts{
				CommonOpts: gc.CommonOpts{
					K8sC:          client,
					ClusterConfig: &corev1.ClusterConfig{Metadata: &metav1.Metadata{Name: "default"}, Spec: &corev1.ClusterConfig_Spec{}, Status: &corev1.ClusterConfig_Status{Domain: "example.com"}},
					Region:        &corev1.Region{Metadata: &metav1.Metadata{Name: "default", Uid: "region-uid"}, Spec: &corev1.Region_Spec{}, Status: &corev1.Region_Status{}},
				},
			}

			for range 2 {
				require.NoError(t, CreateNocturne(ctx, opts))
			}

			deployment, err := client.AppsV1().Deployments("octelium").Get(ctx, "cordium-nocturne", k8smetav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, "cordium-nocturne", deployment.Spec.Template.Spec.ServiceAccountName)

			role, err := client.RbacV1().Roles("cordium").Get(ctx, "cordium-nocturne", k8smetav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, getNocturneRole().Rules, role.Rules)

			binding, err := client.RbacV1().RoleBindings("cordium").Get(ctx, "cordium-nocturne", k8smetav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, getNocturneRoleBinding().Subjects, binding.Subjects)

			_, err = client.RbacV1().ClusterRoles().Get(ctx, "cordium-nocturne", k8smetav1.GetOptions{})
			assert.True(t, k8serr.IsNotFound(err))
			_, err = client.RbacV1().ClusterRoleBindings().Get(ctx, "cordium-nocturne", k8smetav1.GetOptions{})
			assert.True(t, k8serr.IsNotFound(err))

			actions := client.Actions()
			roleBindingIdx := slices.IndexFunc(actions, func(action k8stesting.Action) bool {
				return action.GetVerb() == "create" && action.GetResource().Resource == "rolebindings"
			})
			deleteIdx := slices.IndexFunc(actions, func(action k8stesting.Action) bool {
				return action.GetVerb() == "delete" && action.GetResource().Resource == "clusterrolebindings"
			})
			require.GreaterOrEqual(t, roleBindingIdx, 0)
			require.GreaterOrEqual(t, deleteIdx, 0)
			assert.Less(t, roleBindingIdx, deleteIdx)
		})
	}
}
