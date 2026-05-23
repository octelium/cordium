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
