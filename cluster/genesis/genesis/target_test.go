package genesis

import (
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcorev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/octeliumc"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	corev1k8s "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type targetTestClient struct {
	octeliumc.ClientInterface
	core *targetTestCoreClient
}

func (c *targetTestClient) CoreC() rcorev1.ResourceServiceClient { return c.core }

type targetTestCoreClient struct {
	rcorev1.ResourceServiceClient
	region *corev1.Region
	names  []string
	err    error
}

func (c *targetTestCoreClient) GetRegion(ctx context.Context, opts *rmetav1.GetOptions, _ ...grpc.CallOption) (*corev1.Region, error) {
	c.names = append(c.names, opts.Name)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.region, c.err
}

func newPackageTarget(t *testing.T) (*Genesis, *corev1.ClusterConfig, *corev1.Region, *fake.Clientset) {
	t.Helper()
	for _, key := range []string{"OCTELIUM_DOMAIN", "OCTELIUM_REGION_NAME", "OCTELIUM_GENESIS_CLUSTER_UID", "OCTELIUM_GENESIS_REGION_UID", "OCTELIUM_GENESIS_NAMESPACE_UID"} {
		t.Setenv(key, "")
	}
	cc := &corev1.ClusterConfig{Metadata: &metav1.Metadata{Name: "default", Uid: "cluster-uid"}, Status: &corev1.ClusterConfig_Status{Domain: "example.com"}}
	region := &corev1.Region{Metadata: &metav1.Metadata{Name: "default", Uid: "region-uid"}, Status: &corev1.Region_Status{}}
	c := fake.NewSimpleClientset(
		&corev1k8s.Namespace{ObjectMeta: k8smetav1.ObjectMeta{Name: vutils.K8sNS, UID: "namespace-uid"}},
		&corev1k8s.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS}, Data: map[string]string{"custom": "preserved"}},
		&corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-postgres", Namespace: vutils.K8sNS}, Data: map[string][]byte{"host": []byte("postgres"), "database": []byte("octelium"), "postgres-password": []byte("pg-password")}},
		&corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-redis", Namespace: vutils.K8sNS}, Data: map[string][]byte{"host": []byte("redis"), "database": []byte("1"), "password": []byte("redis-password")}},
	)
	return &Genesis{k8sC: c, octeliumCInit: &targetTestClient{core: &targetTestCoreClient{region: region}}}, cc, region, c
}

func packageTargetAnnotations(cc *corev1.ClusterConfig, region *corev1.Region) map[string]string {
	return map[string]string{targetClusterUID: cc.Metadata.Uid, targetDomain: cc.Status.Domain, targetRegionUID: region.Metadata.Uid, targetRegionName: region.Metadata.Name, targetNamespaceUID: "namespace-uid"}
}

func assertPackageTargetReadOnly(t *testing.T, c *fake.Clientset) {
	t.Helper()
	for _, action := range c.Actions() {
		assert.Contains(t, []string{"get", "list"}, action.GetVerb())
	}
}

func TestPackageRegionCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, requested, stored string
		bound, bootstrap        bool
	}{
		{name: "legacy-implicit-default"},
		{name: "legacy-explicit-default", requested: "default"},
		{name: "legacy-explicit-region", requested: "west", stored: "west"},
		{name: "legacy-bootstrap-region", stored: "west", bootstrap: true},
		{name: "old-default-env-with-bootstrap", requested: "default", stored: "west", bootstrap: true},
		{name: "bound-default", requested: "default", bound: true},
		{name: "bound-region", requested: "west", stored: "west", bound: true},
		{name: "old-default-env-with-binding", requested: "default", stored: "west", bound: true},
		{name: "binding-and-bootstrap", stored: "west", bound: true, bootstrap: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, cc, region, c := newPackageTarget(t)
			t.Setenv("OCTELIUM_REGION_NAME", tc.requested)
			t.Setenv("OCTELIUM_DOMAIN", "EXAMPLE.com.")
			if tc.stored != "" {
				region.Metadata.Name = tc.stored
			}
			if tc.bootstrap {
				require.NoError(t, c.Tracker().Add(&corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-init", Namespace: vutils.K8sNS}, Data: map[string][]byte{"domain": []byte("Example.COM"), "region": pbutils.MarshalMust(region)}}))
			}
			if tc.bound {
				cm := &corev1k8s.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS, Annotations: packageTargetAnnotations(cc, region)}}
				require.NoError(t, c.Tracker().Update(corev1k8s.SchemeGroupVersion.WithResource("configmaps"), cm, vutils.K8sNS))
				t.Setenv("OCTELIUM_GENESIS_CLUSTER_UID", cc.Metadata.Uid)
				t.Setenv("OCTELIUM_GENESIS_REGION_UID", region.Metadata.Uid)
				t.Setenv("OCTELIUM_GENESIS_NAMESPACE_UID", "namespace-uid")
			}
			got, err := g.prepareRegion(t.Context(), cc)
			require.NoError(t, err)
			assert.Same(t, region, got)
			assert.Equal(t, region.Metadata.Name, os.Getenv("OCTELIUM_REGION_NAME"))
			assert.Equal(t, []string{region.Metadata.Name}, g.octeliumCInit.(*targetTestClient).core.names)
			assertPackageTargetReadOnly(t, c)
		})
	}
}

func TestPackageTargetRejectsWrongBootstrapAndRequest(t *testing.T) {
	for _, change := range []string{"domain", "missing-live-domain", "bootstrap-domain", "region", "region-uid", "malformed-region", "live-region-name", "bound-region-name", "cluster-pin", "region-pin", "namespace-pin"} {
		t.Run(change, func(t *testing.T) {
			g, cc, region, c := newPackageTarget(t)
			stored := pbutils.Clone(region).(*corev1.Region)
			bootstrap := &corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-init", Namespace: vutils.K8sNS}, Data: map[string][]byte{"domain": []byte(cc.Status.Domain)}}
			switch change {
			case "domain":
				t.Setenv("OCTELIUM_DOMAIN", "foreign.example")
			case "missing-live-domain":
				cc.Status.Domain = ""
			case "bootstrap-domain":
				bootstrap.Data["domain"] = []byte("foreign.example")
			case "region":
				t.Setenv("OCTELIUM_REGION_NAME", "west")
			case "region-uid":
				stored.Metadata.Uid = "foreign-region"
			case "malformed-region":
				bootstrap.Data["region"] = []byte("invalid")
			case "live-region-name":
				region.Metadata.Name = "west"
			case "bound-region-name":
				cm := &corev1k8s.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS, Annotations: packageTargetAnnotations(cc, region)}}
				cm.Annotations[targetRegionName] = "west"
				require.NoError(t, c.Tracker().Update(corev1k8s.SchemeGroupVersion.WithResource("configmaps"), cm, vutils.K8sNS))
			case "cluster-pin":
				t.Setenv("OCTELIUM_GENESIS_CLUSTER_UID", "foreign")
			case "region-pin":
				t.Setenv("OCTELIUM_GENESIS_REGION_UID", "foreign")
			case "namespace-pin":
				t.Setenv("OCTELIUM_GENESIS_NAMESPACE_UID", "foreign")
			}
			if change != "malformed-region" {
				bootstrap.Data["region"] = pbutils.MarshalMust(stored)
			}
			require.NoError(t, c.Tracker().Add(bootstrap))
			before := os.Getenv("OCTELIUM_REGION_NAME")
			_, err := g.prepareRegion(t.Context(), cc)
			require.Error(t, err)
			assert.Equal(t, before, os.Getenv("OCTELIUM_REGION_NAME"))
			assertPackageTargetReadOnly(t, c)
		})
	}
}

func TestPackageRejectsIncompleteOrForeignStoredIdentity(t *testing.T) {
	for _, name := range []string{"octelium-installation", "octelium-postgres", "octelium-redis"} {
		for _, key := range []string{targetClusterUID, targetDomain, targetRegionUID, targetRegionName, targetNamespaceUID} {
			for _, missing := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/missing=%t", name, key, missing), func(t *testing.T) {
					g, cc, region, c := newPackageTarget(t)
					resource := corev1k8s.SchemeGroupVersion.WithResource("secrets")
					if name == "octelium-installation" {
						resource = corev1k8s.SchemeGroupVersion.WithResource("configmaps")
					}
					object, err := c.Tracker().Get(resource, vutils.K8sNS, name)
					require.NoError(t, err)
					annotations := packageTargetAnnotations(cc, region)
					if missing {
						delete(annotations, key)
					} else {
						annotations[key] = "foreign"
					}
					object.(k8smetav1.Object).SetAnnotations(annotations)
					require.NoError(t, c.Tracker().Update(resource, object, vutils.K8sNS))
					require.Error(t, g.validateRegionTarget(t.Context(), cc, region))
					assertPackageTargetReadOnly(t, c)
				})
			}
		}
	}
}

func TestPackageStorageTargetAndCredentialRotation(t *testing.T) {
	for _, name := range []string{"octelium-postgres", "octelium-redis"} {
		for _, field := range []string{"host", "port", "database", "password", "postgres-password", "username", "no_ssl", "use_tls"} {
			t.Run(name+"/"+field, func(t *testing.T) {
				g, cc, region, c := newPackageTarget(t)
				secret, err := c.CoreV1().Secrets(vutils.K8sNS).Get(t.Context(), name, k8smetav1.GetOptions{})
				require.NoError(t, err)
				secret.Annotations = packageTargetAnnotations(cc, region)
				secret.Annotations[targetStorageHash] = storageTargetHash(secret)
				secret.Data[field] = []byte("changed")
				require.NoError(t, c.Tracker().Update(corev1k8s.SchemeGroupVersion.WithResource("secrets"), secret, vutils.K8sNS))
				c.ClearActions()
				err = g.validateRegionTarget(t.Context(), cc, region)
				if field == "host" || field == "port" || field == "database" {
					require.ErrorContains(t, err, "storage target changed")
				} else {
					require.NoError(t, err)
				}
				assertPackageTargetReadOnly(t, c)
			})
		}
	}
	for _, name := range []string{"octelium-postgres", "octelium-redis"} {
		secret := &corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: name}, Data: map[string][]byte{"host": []byte("EXAMPLE.com."), "port": []byte("0")}}
		before := storageTargetHash(secret)
		secret.Data["host"] = []byte("example.com")
		if name == "octelium-postgres" {
			secret.Data["port"], secret.Data["database"] = []byte("5432"), []byte("postgres")
		} else {
			secret.Data["port"], secret.Data["database"] = []byte("6379"), []byte("0")
		}
		assert.Equal(t, before, storageTargetHash(secret))
	}
}

func TestPackageLegacyMissingMetadata(t *testing.T) {
	g, cc, region, c := newPackageTarget(t)
	require.NoError(t, c.Tracker().Delete(corev1k8s.SchemeGroupVersion.WithResource("configmaps"), vutils.K8sNS, "octelium-installation"))
	for _, name := range []string{"octelium-postgres", "octelium-redis"} {
		require.NoError(t, c.Tracker().Delete(corev1k8s.SchemeGroupVersion.WithResource("secrets"), vutils.K8sNS, name))
	}
	require.NoError(t, c.Tracker().Delete(corev1k8s.SchemeGroupVersion.WithResource("namespaces"), "", vutils.K8sNS))
	require.NoError(t, g.validateRegionTarget(t.Context(), cc, region))
	_, err := g.prepareRegion(t.Context(), cc)
	require.NoError(t, err)
	assertPackageTargetReadOnly(t, c)
}

func TestPackageTargetReadFailures(t *testing.T) {
	for _, resource := range []string{"configmaps", "secrets", "namespaces"} {
		for _, transient := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/transient=%t", resource, transient), func(t *testing.T) {
				g, cc, region, c := newPackageTarget(t)
				t.Setenv("OCTELIUM_GENESIS_NAMESPACE_UID", "namespace-uid")
				calls := 0
				c.PrependReactor("get", resource, func(action k8stesting.Action) (bool, runtime.Object, error) {
					calls++
					if transient && calls > 1 {
						return false, nil, nil
					}
					if transient {
						return true, nil, io.ErrUnexpectedEOF
					}
					return true, nil, k8serr.NewForbidden(action.GetResource().GroupResource(), "target", io.EOF)
				})
				err := g.validateRegionTarget(t.Context(), cc, region)
				if transient {
					require.NoError(t, err)
					assert.GreaterOrEqual(t, calls, 2)
				} else {
					require.True(t, k8serr.IsForbidden(err))
					assert.Equal(t, 1, calls)
				}
				assertPackageTargetReadOnly(t, c)
			})
		}
	}
	g, cc, region, c := newPackageTarget(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, g.validateRegionTarget(ctx, cc, region), context.Canceled)
	assert.Empty(t, c.Actions())
	c.PrependReactor("get", "configmaps", func(k8stesting.Action) (bool, runtime.Object, error) { return true, nil, io.EOF })
	ctx, cancel = context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, g.validateRegionTarget(ctx, cc, region), context.DeadlineExceeded)
	assertPackageTargetReadOnly(t, c)
}

func TestPackageNamespaceIdentity(t *testing.T) {
	for _, state := range []string{"replaced", "deleted", "terminating", "empty"} {
		t.Run(state, func(t *testing.T) {
			g, cc, region, c := newPackageTarget(t)
			cm := &corev1k8s.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS, Annotations: maps.Clone(packageTargetAnnotations(cc, region))}}
			require.NoError(t, c.Tracker().Update(corev1k8s.SchemeGroupVersion.WithResource("configmaps"), cm, vutils.K8sNS))
			namespace := &corev1k8s.Namespace{ObjectMeta: k8smetav1.ObjectMeta{Name: vutils.K8sNS, UID: "namespace-uid"}}
			switch state {
			case "replaced":
				namespace.UID = "replacement"
			case "deleted":
				require.NoError(t, c.Tracker().Delete(corev1k8s.SchemeGroupVersion.WithResource("namespaces"), "", vutils.K8sNS))
			case "terminating":
				namespace.DeletionTimestamp = new(k8smetav1.Now())
			case "empty":
				namespace.UID = ""
			}
			if state != "deleted" {
				require.NoError(t, c.Tracker().Update(corev1k8s.SchemeGroupVersion.WithResource("namespaces"), namespace, ""))
			}
			require.Error(t, g.validateRegionTarget(t.Context(), cc, region))
			assertPackageTargetReadOnly(t, c)
		})
	}
}
