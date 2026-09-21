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

package controller

import (
	"context"
	"time"

	v1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const snapshotCreationTimeout = 2 * time.Hour

func (c *Controller) OnAddWorkspaceSnapshot(ctx context.Context, snapshot *cordiumv1.WorkspaceSnapshot) error {
	return c.reconcileWorkspaceSnapshot(ctx, snapshot)
}

func (c *Controller) OnUpdateWorkspaceSnapshot(ctx context.Context,
	new, old *cordiumv1.WorkspaceSnapshot) error {
	return c.reconcileWorkspaceSnapshot(ctx, new)
}

func (c *Controller) OnDeleteWorkspaceSnapshot(ctx context.Context, snapshot *cordiumv1.WorkspaceSnapshot) error {
	if !c.isMyRegionWorkspaceSnapshot(snapshot) {
		return nil
	}

	zap.L().Debug("Deleting the volume snapshot of the WorkspaceSnapshot",
		zap.String("name", snapshot.Metadata.Name))

	if err := c.snapshotC.SnapshotV1().VolumeSnapshots(ns).
		Delete(ctx, getWorkspaceSnapshotK8sName(snapshot), k8smetav1.DeleteOptions{}); err != nil {
		if !k8serr.IsNotFound(err) {
			return err
		}
		zap.L().Debug("The volume snapshot is already deleted. Nothing to be done",
			zap.String("name", snapshot.Metadata.Name))
	}

	return nil
}

func (c *Controller) ReconcileWorkspaceSnapshots(ctx context.Context) error {

	var page uint32
	for {
		itmList, err := c.octeliumC.CordiumC().ListWorkspaceSnapshot(ctx, &rmetav1.ListOptions{
			Paginate:     true,
			ItemsPerPage: 500,
			Page:         page,
		})
		if err != nil {
			return err
		}

		for _, itm := range itmList.Items {
			if err := c.reconcileWorkspaceSnapshot(ctx, itm); err != nil {
				zap.L().Warn("Could not reconcile WorkspaceSnapshot",
					zap.String("name", itm.Metadata.Name), zap.Error(err))
			}
		}

		if itmList.ListResponseMeta == nil || !itmList.ListResponseMeta.HasMore {
			return nil
		}

		page = page + 1
	}
}

func (c *Controller) isMyRegionWorkspaceSnapshot(snapshot *cordiumv1.WorkspaceSnapshot) bool {
	if snapshot.Status == nil || snapshot.Status.RegionRef == nil {
		return false
	}

	return snapshot.Status.RegionRef.Uid == c.regionRef.Uid
}

func (c *Controller) reconcileWorkspaceSnapshot(ctx context.Context, snapshot *cordiumv1.WorkspaceSnapshot) error {

	if !c.isMyRegionWorkspaceSnapshot(snapshot) {
		return nil
	}

	if !ucordiumv1.ToWorkspaceSnapshot(snapshot).IsCreating() {
		return nil
	}

	if snapshot.Status.WorkspaceRef == nil {
		return c.setWorkspaceSnapshotFailure(ctx, snapshot,
			&cordiumv1.WorkspaceSnapshot_Status_Failure{
				Message: "The WorkspaceSnapshot does not have a source Workspace",
				Type: &cordiumv1.WorkspaceSnapshot_Status_Failure_Unknown_{
					Unknown: &cordiumv1.WorkspaceSnapshot_Status_Failure_Unknown{},
				},
			})
	}

	k8sSnapshot, err := c.snapshotC.SnapshotV1().VolumeSnapshots(ns).
		Get(ctx, getWorkspaceSnapshotK8sName(snapshot), k8smetav1.GetOptions{})
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return err
		}

		k8sSnapshot, err = c.doCreateWorkspaceSnapshot(ctx, snapshot)
		if err != nil {
			return err
		}

		if k8sSnapshot == nil {
			return nil
		}
	}

	return c.setWorkspaceSnapshotStatus(ctx, snapshot, k8sSnapshot)
}

func (c *Controller) doCreateWorkspaceSnapshot(ctx context.Context,
	snapshot *cordiumv1.WorkspaceSnapshot) (*v1.VolumeSnapshot, error) {

	pvcName := getPVCNameByWorkspaceUID(snapshot.Status.WorkspaceRef.Uid)

	if _, err := c.k8sC.CoreV1().PersistentVolumeClaims(ns).
		Get(ctx, pvcName, k8smetav1.GetOptions{}); err != nil {
		if !k8serr.IsNotFound(err) {
			return nil, err
		}

		return nil, c.setWorkspaceSnapshotFailure(ctx, snapshot,
			&cordiumv1.WorkspaceSnapshot_Status_Failure{
				Message: "The persistent storage of the Workspace does not exist",
				Type: &cordiumv1.WorkspaceSnapshot_Status_Failure_SourceNotFound_{
					SourceNotFound: &cordiumv1.WorkspaceSnapshot_Status_Failure_SourceNotFound{},
				},
			})
	}

	ws, tmpl := c.getWorkspaceSnapshotSources(ctx, snapshot)

	zap.L().Debug("Creating the volume snapshot of the WorkspaceSnapshot",
		zap.String("name", snapshot.Metadata.Name), zap.String("pvc", pvcName))

	k8sSnapshot, err := c.createVolumeSnapshot(ctx, &createVolumeSnapshotReq{
		name:    getWorkspaceSnapshotK8sName(snapshot),
		pvcName: pvcName,
		labels: map[string]string{
			"octelium.com/workspace-snapshot-uid": snapshot.Metadata.Uid,
			"octelium.com/workspace-uid":          snapshot.Status.WorkspaceRef.Uid,
		},
		workspace: ws,
		template:  tmpl,
	})
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return nil, err
		}

		zap.L().Warn("Could not create the volume snapshot. The CSI snapshot API is likely unavailable",
			zap.String("name", snapshot.Metadata.Name), zap.Error(err))

		return nil, c.setWorkspaceSnapshotFailure(ctx, snapshot,
			&cordiumv1.WorkspaceSnapshot_Status_Failure{
				Message: "The Cluster does not support snapshotting the Workspaces' storage",
				Type: &cordiumv1.WorkspaceSnapshot_Status_Failure_Unsupported_{
					Unsupported: &cordiumv1.WorkspaceSnapshot_Status_Failure_Unsupported{},
				},
			})
	}

	return k8sSnapshot, nil
}

func (c *Controller) getWorkspaceSnapshotSources(ctx context.Context,
	snapshot *cordiumv1.WorkspaceSnapshot) (*cordiumv1.Workspace, *cordiumv1.Template) {

	ws, err := c.octeliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{
		Uid: snapshot.Status.WorkspaceRef.Uid,
	})
	if err != nil {
		if !grpcerr.IsNotFound(err) {
			zap.L().Warn("Could not get the Workspace of the WorkspaceSnapshot",
				zap.String("name", snapshot.Metadata.Name), zap.Error(err))
		}
		return nil, nil
	}

	if ws.Status.TemplateRef == nil {
		return ws, nil
	}

	tmpl, err := c.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
		Uid: ws.Status.TemplateRef.Uid,
	})
	if err != nil {
		return ws, nil
	}

	return ws, tmpl
}

func (c *Controller) setWorkspaceSnapshotStatus(ctx context.Context,
	snapshot *cordiumv1.WorkspaceSnapshot, k8sSnapshot *v1.VolumeSnapshot) error {

	old := pbutils.Clone(snapshot).(*cordiumv1.WorkspaceSnapshot)

	if k8sSnapshot.Status != nil {
		if k8sSnapshot.Status.CreationTime != nil {
			snapshot.Status.SnapshotAt = pbutils.Timestamp(k8sSnapshot.Status.CreationTime.Time)
		}

		if k8sSnapshot.Status.RestoreSize != nil {
			if restoreSize := k8sSnapshot.Status.RestoreSize.Value(); restoreSize > 0 {
				snapshot.Status.RestoreSizeBytes = uint64(restoreSize)
			}
		}
	}

	switch {
	case isVolumeSnapshotReady(k8sSnapshot):
		zap.L().Debug("The WorkspaceSnapshot is now ready", zap.String("name", snapshot.Metadata.Name))
		snapshot.Status.State = cordiumv1.WorkspaceSnapshot_Status_STATE_READY
		snapshot.Status.ReadyAt = pbutils.Now()
	case c.isWorkspaceSnapshotTimedOut(snapshot):
		errMsg := getVolumeSnapshotErrMsg(k8sSnapshot)
		zap.L().Warn("The WorkspaceSnapshot could not be taken in time",
			zap.String("name", snapshot.Metadata.Name), zap.String("err", errMsg))
		snapshot.Status.State = cordiumv1.WorkspaceSnapshot_Status_STATE_FAILED
		snapshot.Status.Failure = &cordiumv1.WorkspaceSnapshot_Status_Failure{
			Message: errMsg,
			Type: &cordiumv1.WorkspaceSnapshot_Status_Failure_Storage_{
				Storage: &cordiumv1.WorkspaceSnapshot_Status_Failure_Storage{},
			},
		}
	}

	if pbutils.IsEqual(old, snapshot) {
		return nil
	}

	if _, err := c.octeliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot); err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (c *Controller) isWorkspaceSnapshotTimedOut(snapshot *cordiumv1.WorkspaceSnapshot) bool {
	if !snapshot.Metadata.CreatedAt.IsValid() {
		return false
	}

	return time.Now().After(snapshot.Metadata.CreatedAt.AsTime().Add(snapshotCreationTimeout))
}

func (c *Controller) setWorkspaceSnapshotFailure(ctx context.Context,
	snapshot *cordiumv1.WorkspaceSnapshot, failure *cordiumv1.WorkspaceSnapshot_Status_Failure) error {

	zap.L().Warn("Setting the WorkspaceSnapshot as failed",
		zap.String("name", snapshot.Metadata.Name), zap.Any("failure", failure))

	snapshot.Status.State = cordiumv1.WorkspaceSnapshot_Status_STATE_FAILED
	snapshot.Status.Failure = failure

	if _, err := c.octeliumC.CordiumC().UpdateWorkspaceSnapshot(ctx, snapshot); err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}
