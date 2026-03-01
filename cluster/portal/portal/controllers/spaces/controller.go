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
	c *acache.Cache
}

func NewController(
	c *acache.Cache,
) (*Controller, error) {

	return &Controller{
		c: c,
	}, nil
}

func (c *Controller) OnAdd(ctx context.Context, itm *cordiumv1.Space) error {

	if err := c.c.SetSpace(itm); err != nil {
		return err
	}

	return nil
}

func (c *Controller) OnUpdate(ctx context.Context, new, old *cordiumv1.Space) error {

	if err := c.c.SetSpace(new); err != nil {
		return err
	}

	return nil
}

func (c *Controller) OnDelete(ctx context.Context, itm *cordiumv1.Space) error {

	if err := c.c.DeleteSpace(itm); err != nil {
		return err
	}

	return nil
}
