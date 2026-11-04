package genesis

import (
	"context"
	"os"
	"time"

	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

func (g *Genesis) setRegionVersionMap(ctx context.Context, rgn *corev1.Region) error {
	info := &corev1.Region_Status_VersionInfo{
		Package: "cordium",
		SetAt:   pbutils.Now(),
		Version: ldflags.GetVersion(),
		Id:      os.Getenv("OCTELIUM_INSTALL_ID"),
	}
	err := retryRegionVersion(ctx, func(ctx context.Context) error {
		region, err := g.octeliumC.CoreC().GetRegion(ctx, apivalidation.ObjectToRGetOptions(rgn))
		if err != nil {
			return err
		}
		updated := proto.Clone(region).(*corev1.Region)
		if updated.Status == nil {
			updated.Status = &corev1.Region_Status{}
		}
		if updated.Status.VersionInfoMap == nil {
			updated.Status.VersionInfoMap = make(map[string]*corev1.Region_Status_VersionInfo)
		}
		updated.Status.VersionInfoMap["cordium"] = info
		if proto.Equal(region, updated) {
			return nil
		}
		_, err = g.octeliumC.CoreC().UpdateRegion(ctx, updated)
		return err
	})
	return errors.Wrap(err, "Could not publish cordium Region version")
}

func retryRegionVersion(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	err := wait.ExponentialBackoffWithContext(ctx, retry.DefaultRetry, func(ctx context.Context) (bool, error) {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		lastErr = fn(ctx)
		if lastErr == nil {
			return true, nil
		}
		switch status.Code(lastErr) {
		case codes.OutOfRange, codes.Unavailable, codes.ResourceExhausted, codes.Aborted, codes.DeadlineExceeded:
			return false, nil
		}
		if errors.Is(lastErr, context.DeadlineExceeded) {
			return false, nil
		}
		return false, lastErr
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil && lastErr != nil {
		return lastErr
	}
	return err
}
