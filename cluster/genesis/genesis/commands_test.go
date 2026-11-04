package genesis

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/octelium/cordium/cluster/common/components"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcorev1"
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
			bootstrap := &corev1k8s.Secret{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-init", Namespace: vutils.K8sNS, UID: "bootstrap-uid"}, Data: map[string][]byte{
				"region": pbutils.MarshalMust(region), "domain": []byte("example.com"), "genesis-input-key": []byte("01234567890123456789012345678901"),
			}}
			cm := &corev1k8s.ConfigMap{ObjectMeta: k8smetav1.ObjectMeta{Name: "octelium-installation", Namespace: vutils.K8sNS}, Data: map[string]string{"custom": "preserved"}}
			k8sC := fake.NewSimpleClientset(&corev1k8s.Namespace{ObjectMeta: k8smetav1.ObjectMeta{Name: vutils.K8sNS, UID: "namespace-uid"}}, bootstrap, cm)
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

type versionTestClient struct {
	octeliumc.ClientInterface
	core *versionTestCore
}

func (c *versionTestClient) CoreC() rcorev1.ResourceServiceClient { return c.core }

type versionTestCore struct {
	rcorev1.ResourceServiceClient
	region  *corev1.Region
	gets    int
	updates int
	get     func(context.Context, *rmetav1.GetOptions) error
	update  func(context.Context, *corev1.Region) error
}

func (c *versionTestCore) GetRegion(ctx context.Context, opts *rmetav1.GetOptions, _ ...grpc.CallOption) (*corev1.Region, error) {
	c.gets++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.get != nil {
		if err := c.get(ctx, opts); err != nil {
			return nil, err
		}
	}
	return proto.Clone(c.region).(*corev1.Region), nil
}

func (c *versionTestCore) UpdateRegion(ctx context.Context, region *corev1.Region, _ ...grpc.CallOption) (*corev1.Region, error) {
	c.updates++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c.update != nil {
		if err := c.update(ctx, region); err != nil {
			return nil, err
		}
	}
	c.region = proto.Clone(region).(*corev1.Region)
	c.region.Metadata.ResourceVersion = fmt.Sprintf("rv-%d", c.updates)
	return proto.Clone(c.region).(*corev1.Region), nil
}

func newVersionTestGenesis(t *testing.T) (*Genesis, *versionTestCore) {
	t.Helper()
	t.Setenv("OCTELIUM_INSTALL_ID", "existing-operation-id")
	c := &versionTestCore{region: &corev1.Region{
		Metadata: &metav1.Metadata{Name: "west", Uid: "region-uid", ResourceVersion: "rv-0", Labels: map[string]string{"custom": "preserved"}},
		Spec:     &corev1.Region_Spec{},
		Status: &corev1.Region_Status{
			Version: "old-core",
			VersionInfoMap: map[string]*corev1.Region_Status_VersionInfo{
				"octelium": {Package: "octelium", Version: "old-core", Id: "core-operation"},
				"other":    {Package: "other", Version: "other-version", Id: "other-operation"},
			},
		},
	}}
	client := &versionTestClient{core: c}
	return &Genesis{octeliumC: client, octeliumCInit: client}, c
}

func TestPackageVersionPublicationPreservesState(t *testing.T) {
	for _, mode := range []string{"existing", "empty-map", "empty-status", "name-only", "legacy-empty-install-id"} {
		t.Run(mode, func(t *testing.T) {
			g, c := newVersionTestGenesis(t)
			switch mode {
			case "empty-map":
				c.region.Status.VersionInfoMap = nil
			case "empty-status":
				c.region.Status = nil
			case "name-only":
				c.region.Metadata.Uid = ""
			case "legacy-empty-install-id":
				t.Setenv("OCTELIUM_INSTALL_ID", "")
			}
			before := proto.Clone(c.region).(*corev1.Region)
			c.get = func(_ context.Context, opts *rmetav1.GetOptions) error {
				if mode == "name-only" {
					assert.Equal(t, before.Metadata.Name, opts.Name)
				} else {
					assert.Equal(t, before.Metadata.Uid, opts.Uid)
				}
				return nil
			}
			require.NoError(t, g.setRegionVersionMap(t.Context(), before))
			assert.Equal(t, 1, c.updates)
			expectedMetadata := proto.Clone(before.Metadata).(*metav1.Metadata)
			expectedMetadata.ResourceVersion = "rv-1"
			assert.True(t, proto.Equal(expectedMetadata, c.region.Metadata))
			assert.Equal(t, before.Metadata.Labels, c.region.Metadata.Labels)
			assert.True(t, proto.Equal(before.Spec, c.region.Spec))
			assert.Equal(t, before.GetStatus().GetVersion(), c.region.Status.Version)
			for key, info := range before.GetStatus().GetVersionInfoMap() {
				assert.True(t, proto.Equal(info, c.region.Status.VersionInfoMap[key]))
			}
			info := c.region.Status.VersionInfoMap["cordium"]
			require.NotNil(t, info)
			assert.Equal(t, "cordium", info.Package)
			assert.Equal(t, ldflags.GetVersion(), info.Version)
			if mode == "legacy-empty-install-id" {
				assert.Empty(t, info.Id)
			} else {
				assert.Equal(t, "existing-operation-id", info.Id)
			}
			assert.NotNil(t, info.SetAt)
		})
	}
}

func TestPackageVersionPublicationRetries(t *testing.T) {
	for _, mode := range []string{"read-outage", "write-outage", "conflict", "response-lost"} {
		t.Run(mode, func(t *testing.T) {
			g, c := newVersionTestGenesis(t)
			input := proto.Clone(c.region).(*corev1.Region)
			var first *corev1.Region_Status_VersionInfo
			c.get = func(context.Context, *rmetav1.GetOptions) error {
				if mode == "read-outage" && c.gets == 1 {
					return status.Error(codes.Unavailable, "temporary read failure")
				}
				return nil
			}
			c.update = func(ctx context.Context, proposed *corev1.Region) error {
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				assert.LessOrEqual(t, time.Until(deadline), 30*time.Second)
				if first == nil {
					first = proto.Clone(proposed.Status.VersionInfoMap["cordium"]).(*corev1.Region_Status_VersionInfo)
				} else {
					assert.True(t, proto.Equal(first, proposed.Status.VersionInfoMap["cordium"]))
				}
				if c.updates != 1 {
					return nil
				}
				switch mode {
				case "write-outage":
					return status.Error(codes.Unavailable, "temporary write failure")
				case "conflict":
					c.region.Metadata.Labels["concurrent"] = "preserved"
					c.region.Status.VersionInfoMap["other"].Version = "new-peer-version"
					c.region.Metadata.ResourceVersion = "concurrent-rv"
					return status.Error(codes.OutOfRange, "resource changed")
				case "response-lost":
					c.region = proto.Clone(proposed).(*corev1.Region)
					return status.Error(codes.Unavailable, "committed response lost")
				}
				return nil
			}
			require.NoError(t, g.setRegionVersionMap(t.Context(), input))
			assert.Equal(t, 2, c.gets)
			if mode == "conflict" {
				assert.Equal(t, "preserved", c.region.Metadata.Labels["concurrent"])
				assert.Equal(t, "new-peer-version", c.region.Status.VersionInfoMap["other"].Version)
			}
			if mode == "response-lost" || mode == "read-outage" {
				assert.Equal(t, 1, c.updates)
			} else {
				assert.Equal(t, 2, c.updates)
			}
			assert.Equal(t, "old-core", input.Status.Version)
			assert.NotContains(t, input.Status.VersionInfoMap, "cordium")
		})
	}
}

func TestPackageVersionPublicationFailurePolicy(t *testing.T) {
	for _, tc := range []struct {
		code     codes.Code
		attempts int
	}{
		{codes.Unavailable, 5}, {codes.OutOfRange, 5}, {codes.Aborted, 5},
		{codes.ResourceExhausted, 5}, {codes.DeadlineExceeded, 5},
		{codes.PermissionDenied, 1}, {codes.InvalidArgument, 1}, {codes.NotFound, 1}, {codes.Canceled, 1},
	} {
		for _, point := range []string{"read", "write"} {
			t.Run(point+"/"+tc.code.String(), func(t *testing.T) {
				g, c := newVersionTestGenesis(t)
				before := proto.Clone(c.region)
				failure := status.Error(tc.code, "injected failure")
				if point == "read" {
					c.get = func(context.Context, *rmetav1.GetOptions) error { return failure }
				} else {
					c.update = func(context.Context, *corev1.Region) error { return failure }
				}
				require.ErrorIs(t, g.setRegionVersionMap(t.Context(), proto.Clone(c.region).(*corev1.Region)), failure)
				assert.Equal(t, tc.attempts, c.gets)
				if point == "write" {
					assert.Equal(t, tc.attempts, c.updates)
				} else {
					assert.Zero(t, c.updates)
				}
				assert.True(t, proto.Equal(before, c.region))
			})
		}
	}
}

func TestPackageVersionPublicationCancellation(t *testing.T) {
	for _, mode := range []string{"before", "during", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			g, c := newVersionTestGenesis(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "before" {
				cancel()
			} else if mode == "deadline" {
				ctx, cancel = context.WithTimeout(ctx, 20*time.Millisecond)
				defer cancel()
			}
			c.update = func(ctx context.Context, _ *corev1.Region) error {
				if mode == "during" {
					cancel()
				} else {
					<-ctx.Done()
				}
				return ctx.Err()
			}
			err := g.setRegionVersionMap(ctx, proto.Clone(c.region).(*corev1.Region))
			if mode == "deadline" {
				require.ErrorIs(t, err, context.DeadlineExceeded)
			} else {
				require.ErrorIs(t, err, context.Canceled)
			}
			assert.LessOrEqual(t, c.updates, 1)
			if mode == "before" {
				assert.Zero(t, c.gets)
			}
			assert.NotContains(t, c.region.Status.VersionInfoMap, "cordium")
		})
	}
}
