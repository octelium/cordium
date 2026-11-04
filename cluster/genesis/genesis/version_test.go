package genesis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rcorev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

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
