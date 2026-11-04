package genesis

import (
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func newCompletionProxy(t *testing.T, before, after func(string, []byte) error) {
	t.Helper()
	connection, err := grpc.NewClient(net.JoinHostPort("localhost", os.Getenv("OCTELIUM_TEST_RSCSERVER_PORT")), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer(grpc.UnknownServiceHandler(func(_ any, stream grpc.ServerStream) error {
		method, _ := grpc.MethodFromServerStream(stream)
		request, response := &emptypb.Empty{}, &emptypb.Empty{}
		if err := stream.RecvMsg(request); err != nil {
			return err
		}
		data, err := proto.Marshal(request)
		if err != nil {
			return err
		}
		if err := before(method, data); err != nil {
			return err
		}
		if err := connection.Invoke(stream.Context(), method, request, response); err != nil {
			return err
		}
		if err := after(method, data); err != nil {
			return err
		}
		return stream.SendMsg(response)
	}))
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		<-done
		require.NoError(t, connection.Close())
	})
	t.Setenv("OCTELIUM_TEST_RSCSERVER_PORT", strconv.Itoa(listener.Addr().(*net.TCPAddr).Port))
	t.Setenv("OCTELIUM_TEST_CORDIUM_RSCSERVER_PORT", "")
}

func TestPackageCompletionRecoveryE2E(t *testing.T) {
	for _, point := range []string{"init-version-denied", "upgrade-version-denied", "upgrade-version-conflict", "upgrade-version-response-lost", "init-version-response-lost", "upgrade-resource-denied"} {
		t.Run(point, func(t *testing.T) {
			t.Setenv("OCTELIUM_DOMAIN", "example.com")
			t.Setenv("OCTELIUM_REGION_NAME", "default")
			for _, name := range []string{"OCTELIUM_GENESIS_CLUSTER_UID", "OCTELIUM_GENESIS_REGION_UID", "OCTELIUM_GENESIS_NAMESPACE_UID"} {
				t.Setenv(name, "")
			}
			tst, err := otests.Initialize(nil)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, tst.Destroy()) })
			for _, name := range []string{"octelium", "octelium-api"} {
				require.NoError(t, genesisutils.CreateOrUpdateNamespace(t.Context(), tst.C.OcteliumC, &corev1.Namespace{Metadata: &metav1.Metadata{Name: name, IsSystem: true}}))
			}
			region, err := tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Name: "default"})
			require.NoError(t, err)
			bootstrap := &k8scorev1.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-init", Namespace: vutils.K8sNS, UID: "bootstrap-uid"}, Data: map[string][]byte{
				"region": pbutils.MarshalMust(region), "domain": []byte("example.com"), "genesis-input-key": []byte("01234567890123456789012345678901"),
			}}
			cm := &k8scorev1.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS}, Data: map[string]string{"custom": "preserved"}}
			k8sC := fake.NewSimpleClientset(&k8scorev1.Namespace{ObjectMeta: k8smetav1.ObjectMeta{Name: vutils.K8sNS, UID: "namespace-uid"}}, bootstrap, cm)
			g := &Genesis{k8sC: k8sC}
			require.NoError(t, g.RunInit(t.Context(), nil))
			secrets, err := k8sC.CoreV1().Secrets(vutils.K8sNS).List(t.Context(), k8smetav1.ListOptions{})
			require.NoError(t, err)
			coreSecrets, err := tst.C.OcteliumC.CoreC().ListSecret(t.Context(), &rmetav1.ListOptions{})
			require.NoError(t, err)
			packageKey, err := tst.C.OcteliumC.CordiumC().GetSecret(t.Context(), &rmetav1.GetOptions{Name: "sys:ws-tunnel-wgkey"})
			require.NoError(t, err)
			region, err = tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Name: "default"})
			require.NoError(t, err)
			region.Status.VersionInfoMap["cordium"] = &corev1.Region_Status_VersionInfo{Package: "cordium", Version: "previous", Id: "previous-operation"}
			region.Status.VersionInfoMap["peer"] = &corev1.Region_Status_VersionInfo{Package: "peer", Version: "peer-version", Id: "peer-operation"}
			if region.Metadata.Labels == nil {
				region.Metadata.Labels = make(map[string]string)
			}
			region.Metadata.Labels["custom"] = "preserved"
			region, err = tst.C.OcteliumC.CoreC().UpdateRegion(t.Context(), region)
			require.NoError(t, err)
			previous := proto.Clone(region.Status.VersionInfoMap["cordium"])
			coreVersion := region.Status.Version
			t.Setenv("OCTELIUM_INSTALL_ID", "new-operation")
			var failing atomic.Bool
			failing.Store(true)
			var hits, commits atomic.Int32
			isPublication := func(method string, data []byte) bool {
				if !strings.HasSuffix(method, "/UpdateRegion") {
					return false
				}
				candidate := &corev1.Region{}
				if proto.Unmarshal(data, candidate) != nil {
					return false
				}
				return candidate.GetStatus().GetVersionInfoMap()["cordium"].GetId() == "new-operation"
			}
			newCompletionProxy(t, func(method string, data []byte) error {
				if !failing.Load() {
					return nil
				}
				if point == "upgrade-resource-denied" && strings.HasSuffix(method, "/GetSecret") && strings.Contains(method, "cordium") {
					hits.Add(1)
					return status.Error(codes.PermissionDenied, "required package resource denied")
				}
				if !isPublication(method, data) {
					return nil
				}
				if strings.HasSuffix(point, "version-denied") {
					hits.Add(1)
					return status.Error(codes.PermissionDenied, "version publication denied")
				}
				if strings.HasSuffix(point, "version-conflict") && hits.Add(1) == 1 {
					current, err := tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Uid: region.Metadata.Uid})
					if err != nil {
						return err
					}
					current.Status.VersionInfoMap["peer"].Version = "concurrent-peer-version"
					current.Metadata.Labels["concurrent"] = "preserved"
					if _, err := tst.C.OcteliumC.CoreC().UpdateRegion(t.Context(), current); err != nil {
						return err
					}
					return status.Error(codes.OutOfRange, "concurrent Region change")
				}
				return nil
			}, func(method string, data []byte) error {
				if !isPublication(method, data) {
					return nil
				}
				commits.Add(1)
				if failing.Load() && strings.HasSuffix(point, "version-response-lost") && hits.Add(1) == 1 {
					return status.Error(codes.Unavailable, "committed publication response lost")
				}
				return nil
			})
			run := func() error {
				if strings.HasPrefix(point, "init-") {
					return g.RunInit(t.Context(), nil)
				}
				return g.RunUpgrade(t.Context(), nil)
			}
			err = run()
			current, readErr := tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Uid: region.Metadata.Uid})
			require.NoError(t, readErr)
			if strings.HasSuffix(point, "denied") {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "denied")
				assert.True(t, proto.Equal(previous, current.Status.VersionInfoMap["cordium"]))
				assert.Zero(t, commits.Load())
				failing.Store(false)
				require.NoError(t, run())
			} else {
				require.NoError(t, err)
			}
			assert.Positive(t, hits.Load())
			assert.EqualValues(t, 1, commits.Load())
			current, err = tst.C.OcteliumC.CoreC().GetRegion(t.Context(), &rmetav1.GetOptions{Uid: region.Metadata.Uid})
			require.NoError(t, err)
			assert.Equal(t, region.Metadata.Uid, current.Metadata.Uid)
			assert.Equal(t, "preserved", current.Metadata.Labels["custom"])
			assert.Equal(t, coreVersion, current.Status.Version)
			assert.Equal(t, ldflags.GetVersion(), current.Status.VersionInfoMap["cordium"].Version)
			assert.Equal(t, "new-operation", current.Status.VersionInfoMap["cordium"].Id)
			if strings.HasSuffix(point, "version-conflict") {
				assert.Equal(t, "preserved", current.Metadata.Labels["concurrent"])
				assert.Equal(t, "concurrent-peer-version", current.Status.VersionInfoMap["peer"].Version)
			} else {
				assert.Equal(t, "peer-version", current.Status.VersionInfoMap["peer"].Version)
			}
			for _, secret := range secrets.Items {
				stored, err := k8sC.CoreV1().Secrets(vutils.K8sNS).Get(t.Context(), secret.Name, k8smetav1.GetOptions{})
				require.NoError(t, err)
				assert.Equal(t, secret.UID, stored.UID)
				assert.Equal(t, secret.Data, stored.Data)
			}
			for _, secret := range coreSecrets.Items {
				stored, err := tst.C.OcteliumC.CoreC().GetSecret(t.Context(), &rmetav1.GetOptions{Uid: secret.Metadata.Uid})
				require.NoError(t, err)
				assert.True(t, proto.Equal(secret.Data, stored.Data))
				assert.True(t, proto.Equal(secret.Spec, stored.Spec))
			}
			storedKey, err := tst.C.OcteliumC.CordiumC().GetSecret(t.Context(), &rmetav1.GetOptions{Uid: packageKey.Metadata.Uid})
			require.NoError(t, err)
			assert.True(t, proto.Equal(packageKey.Data, storedKey.Data))
			storedCM, err := k8sC.CoreV1().ConfigMaps(vutils.K8sNS).Get(t.Context(), cm.Name, k8smetav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, cm, storedCM)
		})
	}
}
