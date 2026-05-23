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

package templates

import (
	"context"
	"fmt"

	snapshotset "github.com/kubernetes-csi/external-snapshotter/client/v8/clientset/versioned"
	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/k8sutils"
	"go.uber.org/zap"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Controller struct {
	octeliumC octeliumc.ClientInterface
	snapshotC snapshotset.Interface
}

func NewController(
	ctx context.Context,
	octeliumC octeliumc.ClientInterface,
) (*Controller, error) {

	cfg, err := k8sutils.GetInClusterConfig()
	if err != nil {
		return nil, err
	}

	snapshotC, err := snapshotset.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Controller{
		octeliumC: octeliumC,
		snapshotC: snapshotC,
	}, nil
}

func (c *Controller) OnAdd(ctx context.Context, tmpl *cordiumv1.Template) error {

	return nil
}

func (c *Controller) OnUpdate(ctx context.Context, new, old *cordiumv1.Template) error {

	return nil
}

func (c *Controller) OnDelete(ctx context.Context, tmpl *cordiumv1.Template) error {
	tmplList, err := c.snapshotC.SnapshotV1().VolumeSnapshots(workspacecommon.K8sNS).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("octelium.com/template-uid=%s", tmpl.Metadata.Uid),
	})
	if err != nil {
		if k8serr.IsNotFound(err) {
			return nil
		}

		return err
	}

	for _, itm := range tmplList.Items {
		zap.L().Debug("Deleting k8s snapshot", zap.String("name", itm.Name))
		if err := c.snapshotC.SnapshotV1().
			VolumeSnapshots(workspacecommon.K8sNS).Delete(ctx, itm.Name, metav1.DeleteOptions{}); err != nil {
			if !k8serr.IsNotFound(err) {
				zap.L().Warn("Could not delete k8sSnapshot", zap.Error(err))
			}
		}
	}

	return nil
}
