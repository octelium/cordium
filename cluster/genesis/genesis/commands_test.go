package genesis

import (
	"context"
	"testing"

	"github.com/octelium/cordium/cluster/common/components"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/cluster/genesis/genesis/genesisutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1k8s "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPackageCommandsWithLegacyAndBoundClusters(t *testing.T) {
	for _, mode := range []string{"legacy-default", "legacy-joined", "bound-default", "bound-joined"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("OCTELIUM_DOMAIN", "example.com")
			t.Setenv("OCTELIUM_REGION_NAME", "default")
			for _, key := range []string{"OCTELIUM_GENESIS_CLUSTER_UID", "OCTELIUM_GENESIS_REGION_UID", "OCTELIUM_GENESIS_NAMESPACE_UID"} {
				t.Setenv(key, "")
			}
			tst, err := otests.Initialize(nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, tst.Destroy()) })
			for _, name := range []string{"octelium", "octelium-api"} {
				require.NoError(t, genesisutils.CreateOrUpdateNamespace(t.Context(), tst.C.OcteliumC, &corev1.Namespace{Metadata: &metav1.Metadata{Name: name, IsSystem: true}}))
			}
			cc, err := tst.C.OcteliumC.CoreV1Utils().GetClusterConfig(t.Context())
			require.NoError(t, err)
			region, err := tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Name: "default"})
			require.NoError(t, err)
			if mode == "legacy-joined" || mode == "bound-joined" {
				region, err = tst.C.OcteliumC.CoreC().CreateRegion(t.Context(), &corev1.Region{Metadata: &metav1.Metadata{Name: "west"}, Spec: &corev1.Region_Spec{}, Status: &corev1.Region_Status{Index: 1}})
				require.NoError(t, err)
			}
			bootstrap := &corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-init", Namespace: vutils.K8sNS}, Data: map[string][]byte{"domain": []byte(cc.Status.Domain), "region": pbutils.MarshalMust(region)}}
			cm := &corev1k8s.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS}, Data: map[string]string{"custom": "preserved"}}
			if mode == "bound-default" || mode == "bound-joined" {
				cm.Annotations = packageTargetAnnotations(cc, region)
				t.Setenv("OCTELIUM_GENESIS_CLUSTER_UID", cc.Metadata.Uid)
				t.Setenv("OCTELIUM_GENESIS_REGION_UID", region.Metadata.Uid)
				t.Setenv("OCTELIUM_GENESIS_NAMESPACE_UID", "namespace-uid")
			}
			k8sC := fake.NewSimpleClientset(&corev1k8s.Namespace{ObjectMeta: k8smetav1.ObjectMeta{Name: vutils.K8sNS, UID: "namespace-uid"}}, bootstrap, cm)
			g := &Genesis{k8sC: k8sC}
			key, err := tst.C.OcteliumC.CordiumC().GetSecret(t.Context(), &rmetav1.GetOptions{Name: "sys:ws-tunnel-wgkey"})
			require.NoError(t, err)
			require.NoError(t, g.RunInit(t.Context(), nil))
			certificate, err := tst.C.OcteliumC.CoreC().GetSecret(t.Context(), &rmetav1.GetOptions{Name: "crt-svc-default-cordium"})
			require.NoError(t, err)
			for range 2 {
				require.NoError(t, g.RunUpgrade(t.Context(), nil))
			}
			require.NoError(t, g.RunInit(t.Context(), nil))
			currentKey, err := tst.C.OcteliumC.CordiumC().GetSecret(t.Context(), &rmetav1.GetOptions{Name: key.Metadata.Name})
			require.NoError(t, err)
			assert.Equal(t, key.Metadata.Uid, currentKey.Metadata.Uid)
			assert.True(t, pbutils.IsEqual(key.Data, currentKey.Data))
			currentCert, err := tst.C.OcteliumC.CoreC().GetSecret(t.Context(), &rmetav1.GetOptions{Name: certificate.Metadata.Name})
			require.NoError(t, err)
			assert.Equal(t, certificate.Metadata.Uid, currentCert.Metadata.Uid)
			assert.True(t, pbutils.IsEqual(certificate.Data, currentCert.Data))
			currentRegion, err := tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Name: region.Metadata.Name})
			require.NoError(t, err)
			assert.Equal(t, region.Metadata.Uid, currentRegion.Metadata.Uid)
			assert.Equal(t, ldflags.GetVersion(), currentRegion.Status.VersionInfoMap["cordium"].Version)
			assert.True(t, currentRegion.GetStatus().GetExt()["cordium"].GetFields()["isEnabled"].GetBoolValue())
			for _, name := range []string{components.RscServer, components.Nocturne} {
				deployment, err := k8sC.AppsV1().Deployments(vutils.K8sNS).Get(t.Context(), components.CordiumComponent(name), k8smetav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, components.GetImage(name, ""), deployment.Spec.Template.Spec.Containers[0].Image)
			}
			stored, err := k8sC.CoreV1().Secrets(vutils.K8sNS).Get(t.Context(), bootstrap.Name, k8smetav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, bootstrap.Data, stored.Data)
			storedCM, err := k8sC.CoreV1().ConfigMaps(vutils.K8sNS).Get(t.Context(), cm.Name, k8smetav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, cm, storedCM)
		})
	}
}

func TestPackageCommandsCancelBeforeMutation(t *testing.T) {
	g := &Genesis{k8sC: fake.NewSimpleClientset()}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, g.RunInit(ctx, nil), context.Canceled)
	require.ErrorIs(t, g.RunUpgrade(ctx, nil), context.Canceled)
	assert.Empty(t, g.k8sC.(*fake.Clientset).Actions())
}
