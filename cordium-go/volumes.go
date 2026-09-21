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

package cordium

import (
	"context"
	"iter"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
)

// VolumeState is the state of a Volume. It is an alias of the generated enum,
// so the generated constants and a VolumeState are interchangeable.
type VolumeState = cordiumv1.Volume_Status_State

// The states of a Volume.
const (
	VolumeStateUnknown = cordiumv1.Volume_Status_STATE_UNKNOWN
	VolumeStatePending = cordiumv1.Volume_Status_STATE_PENDING
	VolumeStateReady   = cordiumv1.Volume_Status_STATE_READY
	VolumeStateFailed  = cordiumv1.Volume_Status_STATE_FAILED
)

// VolumeAccessMode is the concurrency guarantee of a Volume.
type VolumeAccessMode = cordiumv1.Volume_AccessMode

// The access modes of a Volume.
const (
	VolumeAccessModeExclusive = cordiumv1.Volume_ACCESS_MODE_EXCLUSIVE
	VolumeAccessModeShared    = cordiumv1.Volume_ACCESS_MODE_SHARED
)

// VolumeOption configures a Volume at creation time.
type VolumeOption func(*cordiumv1.Volume) error

// WithVolumeSize sets the requested capacity of the Volume in megabytes. It
// defaults to the Cluster's default Volume size.
func WithVolumeSize(megabytes uint32) VolumeOption {
	return func(vol *cordiumv1.Volume) error {
		if megabytes == 0 {
			return invalidArgumentf("empty Volume size")
		}
		vol.Spec.Size = &cordiumv1.Volume_Spec_Size{Megabytes: megabytes}
		return nil
	}
}

// SharedVolume allows the Volume to be mounted by several running Workspaces
// at the same time. It requires the Cluster to be configured with a shared
// filesystem storage backend. Volumes are EXCLUSIVE by default, which means
// that a single running Workspace mounts them at a time.
func SharedVolume() VolumeOption {
	return func(vol *cordiumv1.Volume) error {
		vol.Spec.AccessMode = cordiumv1.Volume_ACCESS_MODE_SHARED
		return nil
	}
}

// InVolumeRegion pins the Volume to a specific Region. Only the Workspaces
// that run in that Region can mount it.
func InVolumeRegion(name string) VolumeOption {
	return func(vol *cordiumv1.Volume) error {
		if name == "" {
			return invalidArgumentf("empty Region name")
		}
		vol.Status.RegionRef = &metav1.ObjectReference{Name: name}
		return nil
	}
}

// VolumeClient is the Space scoped Volume API. A Volume is a persistent
// storage device that the Workspaces of a Space mount at arbitrary paths.
// Unlike a Workspace's own storage, it has a lifecycle of its own, it outlives
// the Workspaces that mount it and it is never included in their
// WorkspaceSnapshots.
//
// It is obtained from [Client.Volumes].
type VolumeClient struct {
	c *Client
}

// Create creates a Volume inside a Space. The name can be short (e.g.
// "datasets") or qualified with its Space (e.g. "datasets.my-project"). The
// caller must be at least an ADMIN Member of the Space.
//
//	vol, err := c.Volumes().Create(ctx, "datasets", cordium.WithVolumeSize(50000))
//
// The Volume is returned in the PENDING state. It can already be mounted by
// the Workspaces while it is PENDING since the storage backends commonly defer
// the provisioning itself until the first Workspace that mounts it is
// scheduled.
func (vc *VolumeClient) Create(ctx context.Context,
	name string, opts ...VolumeOption) (*cordiumv1.Volume, error) {
	if err := vc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Volume name")
	}

	vol := &cordiumv1.Volume{
		Metadata: &metav1.Metadata{Name: name},
		Spec:     &cordiumv1.Volume_Spec{},
		Status:   &cordiumv1.Volume_Status{},
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(vol); err != nil {
			return nil, err
		}
	}

	return vc.c.MainService().CreateVolume(ctx, vol)
}

// Get retrieves a Volume by name.
func (vc *VolumeClient) Get(ctx context.Context, name string) (*cordiumv1.Volume, error) {
	if err := vc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Volume name")
	}

	return vc.c.MainService().GetVolume(ctx, getOptionsFor(name))
}

// Grow grows a Volume to a new size. A Volume can never be shrunk and growing
// one additionally requires the storage backend to support the expansion of
// the already provisioned volumes.
func (vc *VolumeClient) Grow(ctx context.Context,
	name string, megabytes uint32) (*cordiumv1.Volume, error) {
	if err := vc.c.ensureOpen(); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, invalidArgumentf("empty Volume name")
	}
	if megabytes == 0 {
		return nil, invalidArgumentf("empty Volume size")
	}

	vol, err := vc.Get(ctx, name)
	if err != nil {
		return nil, err
	}

	vol.Spec.Size = &cordiumv1.Volume_Spec_Size{Megabytes: megabytes}

	return vc.c.MainService().UpdateVolume(ctx, vol)
}

// VolumeList is a single page of Volumes.
type VolumeList struct {
	// Items is the page's Volumes.
	Items []*cordiumv1.Volume
	// Page is the pagination information of the response.
	Page PageInfo
}

// List returns one page of the Volumes of a Space, which is chosen with
// [InSpace].
func (vc *VolumeClient) List(ctx context.Context, opts ...ListOption) (*VolumeList, error) {
	if err := vc.c.ensureOpen(); err != nil {
		return nil, err
	}

	cfg, err := newListConfig(opts...)
	if err != nil {
		return nil, err
	}

	list, err := vc.c.MainService().ListVolume(ctx, &cordiumv1.ListVolumeOptions{
		Common:   cfg.common(),
		SpaceRef: cfg.spaceRef,
	})
	if err != nil {
		return nil, err
	}

	return &VolumeList{
		Items: list.GetItems(),
		Page:  pageInfoFrom(list.GetListResponseMeta()),
	}, nil
}

// All iterates over every Volume of a Space, fetching the pages as it goes.
// The iteration stops at the first error, which is yielded with a nil Volume.
func (vc *VolumeClient) All(ctx context.Context,
	opts ...ListOption) iter.Seq2[*cordiumv1.Volume, error] {
	return allPages(ctx, opts,
		func(ctx context.Context, opts ...ListOption) ([]*cordiumv1.Volume, PageInfo, error) {
			list, err := vc.List(ctx, opts...)
			if err != nil {
				return nil, PageInfo{}, err
			}
			return list.Items, list.Page, nil
		})
}

// Delete deletes a Volume together with its underlying storage. It is rejected
// while the Volume is still mounted by a Workspace or a Template of the Space
// regardless of whether those Workspaces are running.
func (vc *VolumeClient) Delete(ctx context.Context, name string) error {
	if err := vc.c.ensureOpen(); err != nil {
		return err
	}
	if name == "" {
		return invalidArgumentf("empty Volume name")
	}

	_, err := vc.c.MainService().DeleteVolume(ctx, deleteOptionsFor(name))
	return err
}

// WaitUntilReady polls a Volume until its underlying storage is provisioned.
// It returns a [*VolumeFailureError] once the Volume fails.
//
// A Volume does not have to be READY in order to be mounted, so waiting is only
// useful when the caller needs the storage to actually exist (e.g. in order to
// report its real capacity). A Volume whose StorageClass defers the
// provisioning until its first consumer stays PENDING until a Workspace that
// mounts it is scheduled, in which case this never returns.
func (vc *VolumeClient) WaitUntilReady(ctx context.Context, name string) (*cordiumv1.Volume, error) {

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		vol, err := vc.Get(ctx, name)
		if err != nil {
			return nil, err
		}

		switch vol.GetStatus().GetState() {
		case VolumeStateReady:
			return vol, nil
		case VolumeStateFailed:
			return nil, &VolumeFailureError{Volume: vol}
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
