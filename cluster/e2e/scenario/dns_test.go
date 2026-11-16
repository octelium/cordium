package scenario

import (
	"testing"

	"github.com/octelium/octelium/cluster/e2e/scenario"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestConfigureClusterDNS(t *testing.T) {
	for _, name := range []string{"Create", "Update"} {
		t.Run(name, func(t *testing.T) {
			k8sC := k8sfake.NewSimpleClientset()
			if name == "Update" {
				_, err := k8sC.CoreV1().ConfigMaps("kube-system").Create(t.Context(), &k8scorev1.ConfigMap{
					ObjectMeta: k8smetav1.ObjectMeta{
						Name:      "coredns-custom",
						Namespace: "kube-system",
						Labels:    map[string]string{"existing": "label"},
					},
					Data: map[string]string{"other.server": "example.test:53 {\n    whoami\n}\n"},
				}, k8smetav1.CreateOptions{})
				require.Nil(t, err)
			}

			require.Nil(t, configureClusterDNS(t.Context(), k8sC, "192.0.2.10"))

			cm, err := k8sC.CoreV1().ConfigMaps("kube-system").Get(t.Context(), "coredns-custom", k8smetav1.GetOptions{})
			require.Nil(t, err)
			assert.Contains(t, cm.Data["cordium.server"], "localhost:53 {")
			assert.Contains(t, cm.Data["cordium.server"], `answer "{{ .Name }} 5 IN A 192.0.2.10"`)
			assert.Contains(t, cm.Data["cordium.server"], "template IN ANY {\n        rcode NOERROR")
			if name == "Update" {
				assert.Equal(t, "example.test:53 {\n    whoami\n}\n", cm.Data["other.server"])
				assert.Equal(t, "label", cm.Labels["existing"])
			}

			k8sC.ClearActions()
			require.Nil(t, configureClusterDNS(t.Context(), k8sC, "192.0.2.10"))
			require.Len(t, k8sC.Actions(), 1)
			assert.Equal(t, "get", k8sC.Actions()[0].GetVerb())

			require.Nil(t, configureClusterDNS(t.Context(), k8sC, "192.0.2.20"))
			cm, err = k8sC.CoreV1().ConfigMaps("kube-system").Get(t.Context(), "coredns-custom", k8smetav1.GetOptions{})
			require.Nil(t, err)
			assert.Contains(t, cm.Data["cordium.server"], "5 IN A 192.0.2.20")
			assert.NotContains(t, cm.Data["cordium.server"], "192.0.2.10")
		})
	}
}

func TestConfigureClusterDNSEmptyConfigMap(t *testing.T) {
	k8sC := k8sfake.NewSimpleClientset(&k8scorev1.ConfigMap{
		ObjectMeta: k8smetav1.ObjectMeta{Name: "coredns-custom", Namespace: "kube-system"},
	})

	require.Nil(t, configureClusterDNS(t.Context(), k8sC, "192.0.2.10"))
	cm, err := k8sC.CoreV1().ConfigMaps("kube-system").Get(t.Context(), "coredns-custom", k8smetav1.GetOptions{})
	require.Nil(t, err)
	assert.NotEmpty(t, cm.Data["cordium.server"])
}

func TestConfigureClusterDNSInvalidAddress(t *testing.T) {
	for _, address := range []string{"", "invalid", "127.0.0.1", "0.0.0.0", "224.0.0.1", "::1", "2001:db8::1"} {
		t.Run(address, func(t *testing.T) {
			k8sC := k8sfake.NewSimpleClientset()
			assert.NotNil(t, configureClusterDNS(t.Context(), k8sC, address))
			assert.Empty(t, k8sC.Actions())
		})
	}
}

func TestConfigureClusterDNSFailure(t *testing.T) {
	for _, verb := range []string{"get", "create", "update"} {
		t.Run(verb, func(t *testing.T) {
			k8sC := k8sfake.NewSimpleClientset()
			if verb == "update" {
				_, err := k8sC.CoreV1().ConfigMaps("kube-system").Create(t.Context(), &k8scorev1.ConfigMap{
					ObjectMeta: k8smetav1.ObjectMeta{Name: "coredns-custom", Namespace: "kube-system"},
				}, k8smetav1.CreateOptions{})
				require.Nil(t, err)
			}

			wantErr := errors.New("the ConfigMap request failed")
			k8sC.PrependReactor(verb, "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})

			assert.ErrorIs(t, configureClusterDNS(t.Context(), k8sC, "192.0.2.10"), wantErr)
		})
	}
}

func TestClusterDNSPreservesPublicDomain(t *testing.T) {
	r := &scenario.Runner{Scenario: &scenario.Scenario{Domain: "example.com"}}
	assert.Nil(t, stepClusterDNS(t.Context(), r))
}
