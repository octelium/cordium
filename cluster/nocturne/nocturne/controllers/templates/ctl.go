/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License version 3,
 * as published by the Free Software Foundation of the License.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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
