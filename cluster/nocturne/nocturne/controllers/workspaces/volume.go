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
	"fmt"

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/cordium/cluster/common/wsutils"
	"github.com/octelium/octelium/apis/cluster/ccordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	utils_types "github.com/octelium/octelium/pkg/utils/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) OnAddVolume(ctx context.Context, vol *cordiumv1.Volume) error {
	return c.reconcileVolume(ctx, vol)
}

func (c *Controller) OnUpdateVolume(ctx context.Context, new, old *cordiumv1.Volume) error {
	return c.reconcileVolume(ctx, new)
}

func (c *Controller) OnDeleteVolume(ctx context.Context, vol *cordiumv1.Volume) error {
	if !c.isMyRegionVolume(vol) {
		return nil
	}

	zap.L().Debug("Removing the PVC of the Volume", zap.String("name", vol.Metadata.Name))

	if err := c.k8sC.CoreV1().PersistentVolumeClaims(ns).
		Delete(ctx, getVolumePVCName(vol), k8smetav1.DeleteOptions{}); err != nil {
		if !k8serr.IsNotFound(err) {
			return err
		}
		zap.L().Debug("The PVC of the Volume is already deleted. Nothing to be done",
			zap.String("name", vol.Metadata.Name))
	}

	return nil
}

func (c *Controller) ReconcileVolumes(ctx context.Context) error {

	var page uint32
	for {
		itmList, err := c.octeliumC.CordiumC().ListVolume(ctx, &rmetav1.ListOptions{
			Paginate:     true,
			ItemsPerPage: 500,
			Page:         page,
		})
		if err != nil {
			return err
		}

		for _, itm := range itmList.Items {
			if err := c.reconcileVolume(ctx, itm); err != nil {
				zap.L().Warn("Could not reconcile Volume",
					zap.String("name", itm.Metadata.Name), zap.Error(err))
			}
		}

		if itmList.ListResponseMeta == nil || !itmList.ListResponseMeta.HasMore {
			return nil
		}

		page = page + 1
	}
}

func (c *Controller) isMyRegionVolume(vol *cordiumv1.Volume) bool {
	if vol.Status == nil || vol.Status.RegionRef == nil {
		return false
	}

	return vol.Status.RegionRef.Uid == c.regionRef.Uid
}

func (c *Controller) reconcileVolume(ctx context.Context, vol *cordiumv1.Volume) error {

	if !c.isMyRegionVolume(vol) {
		return nil
	}

	pvc, err := c.setVolumePersistentVolumeClaim(ctx, vol)
	if err != nil {
		return err
	}

	if pvc == nil {
		return nil
	}

	return c.setVolumeStatus(ctx, vol, pvc)
}

func volumeWasProvisioned(vol *cordiumv1.Volume) bool {
	return vol.Status.ReadyAt.IsValid() ||
		vol.Status.State == cordiumv1.Volume_Status_STATE_READY
}

func (c *Controller) setVolumeFailure(ctx context.Context,
	vol *cordiumv1.Volume, failure *cordiumv1.Volume_Status_Failure) error {

	if vol.Status.State == cordiumv1.Volume_Status_STATE_FAILED &&
		pbutils.IsEqual(vol.Status.Failure, failure) {
		return nil
	}

	zap.L().Warn("Setting the Volume as failed",
		zap.String("name", vol.Metadata.Name), zap.Any("failure", failure))

	vol.Status.State = cordiumv1.Volume_Status_STATE_FAILED
	vol.Status.Failure = failure

	if _, err := c.octeliumC.CordiumC().UpdateVolume(ctx, vol); err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (c *Controller) setVolumePersistentVolumeClaim(ctx context.Context,
	vol *cordiumv1.Volume) (*corev1.PersistentVolumeClaim, error) {

	pvc, err := c.k8sC.CoreV1().PersistentVolumeClaims(ns).
		Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
	if err == nil {
		if pvc.Labels["octelium.com/volume-uid"] != vol.Metadata.Uid {
			return nil, errors.Errorf(
				"The PVC: %s does not belong to the Volume: %s", pvc.Name, vol.Metadata.Name)
		}

		return c.setVolumePersistentVolumeClaimSize(ctx, vol, pvc)
	}

	if !k8serr.IsNotFound(err) {
		return nil, err
	}

	if volumeWasProvisioned(vol) {
		return nil, c.setVolumeFailure(ctx, vol, &cordiumv1.Volume_Status_Failure{
			Message: "The underlying storage of the Volume does not exist anymore",
			Type: &cordiumv1.Volume_Status_Failure_Storage_{
				Storage: &cordiumv1.Volume_Status_Failure_Storage{},
			},
		})
	}

	storageClassName, err := c.getVolumeStorageClassName(ctx, vol)
	if err != nil {
		return nil, err
	}

	zap.L().Debug("Creating the PVC of the Volume",
		zap.String("name", vol.Metadata.Name),
		zap.Uint32("sizeMB", vol.Spec.Size.GetMegabytes()))

	pvc, err = c.k8sC.CoreV1().PersistentVolumeClaims(ns).Create(ctx, &corev1.PersistentVolumeClaim{
		ObjectMeta: k8smetav1.ObjectMeta{
			Name:      getVolumePVCName(vol),
			Namespace: ns,
			Labels:    getVolumeLabels(vol),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: getVolumeAccessModes(vol),
			VolumeMode: func() *corev1.PersistentVolumeMode {
				ret := corev1.PersistentVolumeFilesystem
				return &ret
			}(),
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": *getVolumeStorageQuantity(vol),
				},
			},
			StorageClassName: storageClassName,
		},
	}, k8smetav1.CreateOptions{})
	if err != nil {
		if !k8serr.IsAlreadyExists(err) {
			return nil, err
		}

		return c.k8sC.CoreV1().PersistentVolumeClaims(ns).
			Get(ctx, getVolumePVCName(vol), k8smetav1.GetOptions{})
	}

	return pvc, nil
}

func (c *Controller) setVolumePersistentVolumeClaimSize(ctx context.Context,
	vol *cordiumv1.Volume, pvc *corev1.PersistentVolumeClaim) (*corev1.PersistentVolumeClaim, error) {

	desired := getVolumeStorageQuantity(vol)

	cur, ok := pvc.Spec.Resources.Requests["storage"]
	if ok && cur.Cmp(*desired) >= 0 {
		return pvc, nil
	}

	zap.L().Debug("Growing the PVC of the Volume",
		zap.String("name", vol.Metadata.Name),
		zap.Uint32("sizeMB", vol.Spec.Size.GetMegabytes()))

	if pvc.Spec.Resources.Requests == nil {
		pvc.Spec.Resources.Requests = corev1.ResourceList{}
	}
	pvc.Spec.Resources.Requests["storage"] = *desired

	ret, err := c.k8sC.CoreV1().PersistentVolumeClaims(ns).Update(ctx, pvc, k8smetav1.UpdateOptions{})
	if err != nil {
		zap.L().Warn("Could not grow the PVC of the Volume. The storage backend most probably does not support the expansion of the already provisioned volumes",
			zap.String("name", vol.Metadata.Name), zap.Error(err))
		return pvc, nil
	}

	return ret, nil
}

func (c *Controller) setVolumeStatus(ctx context.Context,
	vol *cordiumv1.Volume, pvc *corev1.PersistentVolumeClaim) error {

	old := pbutils.Clone(vol).(*cordiumv1.Volume)

	if capacity, ok := pvc.Status.Capacity["storage"]; ok {
		if megabytes := capacity.Value() / (1000 * 1000); megabytes > 0 {
			vol.Status.Capacity = &cordiumv1.Volume_Spec_Size{
				Megabytes: uint32(megabytes),
			}
		}
	}

	switch pvc.Status.Phase {
	case corev1.ClaimBound:
		if vol.Status.State != cordiumv1.Volume_Status_STATE_READY {
			zap.L().Debug("The Volume is now ready", zap.String("name", vol.Metadata.Name))
			vol.Status.ReadyAt = pbutils.Now()
		}
		vol.Status.State = cordiumv1.Volume_Status_STATE_READY
		vol.Status.Failure = nil
	case corev1.ClaimLost:
		zap.L().Warn("The PVC of the Volume is lost", zap.String("name", vol.Metadata.Name))
		vol.Status.State = cordiumv1.Volume_Status_STATE_FAILED
		vol.Status.Failure = &cordiumv1.Volume_Status_Failure{
			Message: "The underlying storage of the Volume is lost",
			Type: &cordiumv1.Volume_Status_Failure_Storage_{
				Storage: &cordiumv1.Volume_Status_Failure_Storage{},
			},
		}
	default:
		vol.Status.State = cordiumv1.Volume_Status_STATE_PENDING
	}

	if pbutils.IsEqual(old, vol) {
		return nil
	}

	if _, err := c.octeliumC.CordiumC().UpdateVolume(ctx, vol); err != nil {
		if grpcerr.IsNotFound(err) {
			return nil
		}
		return err
	}

	return nil
}

func (c *Controller) getVolumeStorageClassName(ctx context.Context,
	vol *cordiumv1.Volume) (*string, error) {

	cc, err := c.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	if cc.Spec.Volume == nil || cc.Spec.Volume.Storage == nil ||
		cc.Spec.Volume.Storage.StorageClass == nil ||
		len(cc.Spec.Volume.Storage.StorageClass.Rules) == 0 {
		return nil, nil
	}

	reqCtxMap := map[string]any{
		"ctx": map[string]any{
			"volume": pbutils.MustConvertToMap(vol),
		},
	}

	for _, rule := range cc.Spec.Volume.Storage.StorageClass.Rules {
		if rule.StorageClass == "" {
			continue
		}

		cond, err := ovutils.ToCoreCondition(rule.Condition)
		if err != nil {
			return nil, errors.Errorf(
				"Could not read the storageClass rules of the Cluster: %+v", err)
		}

		isMatched, err := c.celEngine.EvalCondition(ctx, cond, reqCtxMap)
		if err != nil {
			return nil, errors.Errorf(
				"Could not evaluate the storageClass rules of the Cluster: %+v", err)
		}

		if isMatched {
			return utils_types.StrToPtr(rule.StorageClass), nil
		}
	}

	return nil, nil
}

func getVolumeAccessModes(vol *cordiumv1.Volume) []corev1.PersistentVolumeAccessMode {
	switch vol.Spec.AccessMode {
	case cordiumv1.Volume_ACCESS_MODE_SHARED:
		return []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}
	default:
		return []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
	}
}

func getVolumeStorageQuantity(vol *cordiumv1.Volume) *resource.Quantity {
	return getResourceQuantity(fmt.Sprintf("%dM", vol.Spec.Size.GetMegabytes()))
}

func getVolumePVCName(vol *cordiumv1.Volume) string {
	return getVolumePVCNameByUID(vol.Metadata.Uid)
}

func getVolumePVCNameByUID(uid string) string {
	return fmt.Sprintf("vol-%s", uid)
}

func getVolumeLabels(vol *cordiumv1.Volume) map[string]string {
	return map[string]string{
		"app":                         "octelium",
		"octelium.com/component-type": "user",
		"octelium.com/component":      "volume",
		"octelium.com/volume-uid":     vol.Metadata.Uid,
	}
}

func getK8sVolumeName(mount *ccordiumv1.ResolvedVolumeMount) string {
	return fmt.Sprintf("vol-%s", mount.VolumeRef.Uid)
}

func getK8sVolumeMountPath(mount *ccordiumv1.ResolvedVolumeMount) string {
	return workspacecommon.GetVolumePathByUID(mount.VolumeRef.Uid)
}

func (c *Controller) resolveVolumeMounts(ctx context.Context,
	ws *cordiumv1.Workspace) ([]*ccordiumv1.ResolvedVolumeMount, error) {

	var tmpl *cordiumv1.Template
	if ws.Status.TemplateRef != nil {
		var err error
		tmpl, err = c.octeliumC.CordiumC().GetTemplate(ctx, &rmetav1.GetOptions{
			Uid: ws.Status.TemplateRef.Uid,
		})
		if err != nil {
			return nil, err
		}
	}

	resolved, err := wsutils.ResolveVolumeMounts(ctx, c.octeliumC, &wsutils.ResolveVolumeMountsReq{
		Workspace: ws,
		Template:  tmpl,
		RegionRef: ws.Status.RegionRef,
	})
	if err != nil {
		return nil, err
	}

	return resolved.Mounts, nil
}
