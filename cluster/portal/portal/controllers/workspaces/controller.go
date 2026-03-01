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

package controller

import (
	"context"

	"github.com/octelium/cordium/cluster/portal/portal/acache"
	"github.com/octelium/octelium/apis/main/cordiumv1"
)

type Controller struct {
	c    *acache.Cache
	srvI srvI
}

type srvI interface {
	OnWorkspaceCreate(ctx context.Context, ws *cordiumv1.Workspace) error
	OnWorkspaceUpdate(ctx context.Context, new, old *cordiumv1.Workspace) error
	OnWorkspaceDelete(ctx context.Context, ws *cordiumv1.Workspace) error
}

func NewController(
	c *acache.Cache,
	srvI srvI,
) (*Controller, error) {

	return &Controller{
		c:    c,
		srvI: srvI,
	}, nil
}

func (c *Controller) OnAdd(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Status.IsBuild {
		return nil
	}

	if err := c.c.SetWorkspace(ws); err != nil {
		return err
	}

	if c.srvI != nil {
		if err := c.srvI.OnWorkspaceCreate(ctx, ws); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) OnUpdate(ctx context.Context, new, old *cordiumv1.Workspace) error {
	if new.Status.IsBuild {
		return nil
	}

	if err := c.c.SetWorkspace(new); err != nil {
		return err
	}

	if c.srvI != nil {
		if err := c.srvI.OnWorkspaceUpdate(ctx, new, old); err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) OnDelete(ctx context.Context, ws *cordiumv1.Workspace) error {
	if ws.Status.IsBuild {
		return nil
	}

	if err := c.c.DeleteWorkspace(ws); err != nil {
		return err
	}

	if c.srvI != nil {
		if err := c.srvI.OnWorkspaceDelete(ctx, ws); err != nil {
			return err
		}
	}

	return nil
}
