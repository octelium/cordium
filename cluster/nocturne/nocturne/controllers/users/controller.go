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

	workspacecommon "github.com/octelium/cordium/cluster/common"
	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/common/ourscsrv"
	"github.com/octelium/cordium/cluster/common/ovutils"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/urscsrv"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
	"github.com/octelium/octelium/pkg/common/pbutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"go.uber.org/zap"
)

type Controller struct {
	octeliumC octeliumc.ClientInterface
}

func NewController(
	ctx context.Context,
	octeliumC octeliumc.ClientInterface,
) (*Controller, error) {

	return &Controller{
		octeliumC: octeliumC,
	}, nil
}

func (c *Controller) OnAdd(ctx context.Context, usr *corev1.User) error {

	return nil
}

func (c *Controller) OnUpdate(ctx context.Context, new, old *corev1.User) error {

	if !vutils.IsDefaultRegion() {
		return nil
	}

	if pbutils.IsEqual(workspacecommon.GetMembershipUserInfo(new), workspacecommon.GetMembershipUserInfo(old)) {
		return nil
	}

	zap.L().Debug("Updating Membership UserInfo for User", zap.String("name", new.Metadata.Name))

	memList, err := c.octeliumC.CordiumC().ListMembership(ctx, urscsrv.FilterByUser(new))
	if err != nil {
		return err
	}

	userInfo := workspacecommon.GetMembershipUserInfo(new)

	for _, mem := range memList.Items {
		mem.Status.UserInfo = userInfo
		_, err = c.octeliumC.CordiumC().UpdateMembership(ctx, mem)
		if err != nil {
			zap.L().Error("Could not update Membership", zap.Error(err))
		}
	}

	return nil
}

func (c *Controller) OnDelete(ctx context.Context, usr *corev1.User) error {
	if !vutils.IsDefaultRegion() {
		return nil
	}

	return c.doDeleteResources(ctx, usr)
}

func (c *Controller) doDeleteResources(ctx context.Context, usr *corev1.User) error {

	zap.L().Debug("Starting deleting resources of User", zap.String("userName", usr.Metadata.Name))
	usrCfg, err := c.octeliumC.CordiumC().GetUserConfig(ctx, &rmetav1.GetOptions{
		Name: ovutils.GetUserConfigName(umetav1.GetObjectReference(usr)),
	})
	if err == nil {
		c.octeliumC.CordiumC().DeleteUserConfig(ctx, &rmetav1.DeleteOptions{
			Uid: usrCfg.Metadata.Uid,
		})
	} else if err != nil && !grpcerr.IsNotFound(err) {
		return err
	}

	usrSecretList, err := c.octeliumC.CordiumC().ListUserSecret(ctx, urscsrv.FilterByUser(usr))
	if err != nil {
		return err
	}

	for _, sec := range usrSecretList.Items {
		c.octeliumC.CordiumC().DeleteUserSecret(ctx, &rmetav1.DeleteOptions{
			Uid: sec.Metadata.Uid,
		})
	}

	spaceList, err := c.octeliumC.CordiumC().ListSpace(ctx, urscsrv.FilterByUser(usr))
	if err != nil {
		return err
	}

	for _, spc := range spaceList.Items {
		c.doDeleteSpaceResources(ctx, spc)
	}

	zap.L().Debug("Done deleting resources of User", zap.String("userName", usr.Metadata.Name))

	return nil
}

func (s *Controller) doDeleteSpaceResources(ctx context.Context, itm *cordiumv1.Space) error {

	{
		memList, err := s.octeliumC.CordiumC().ListMembership(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteMembership(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return err
				}
			}
		}
	}

	/*
		{
			memList, err := s.octeliumC.CordiumC().ListBuild(ctx, ourscsrv.FilterBySpace(itm))
			if err != nil {
				return err
			}

			for _, item := range memList.Items {
				_, err := s.octeliumC.CordiumC().DeleteBuild(ctx, &rmetav1.DeleteOptions{
					Uid: item.Metadata.Uid,
				})
				if err != nil {
					if !grpcerr.IsNotFound(err) {
						return err
					}
				}
			}
		}
	*/

	{
		memList, err := s.octeliumC.CordiumC().ListTemplate(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteTemplate(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return err
				}
			}
		}
	}

	{
		memList, err := s.octeliumC.CordiumC().ListSecret(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteSecret(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return err
				}
			}
		}
	}

	{
		memList, err := s.octeliumC.CordiumC().ListGitProvider(ctx, ourscsrv.FilterBySpace(itm))
		if err != nil {
			return err
		}

		for _, item := range memList.Items {
			_, err := s.octeliumC.CordiumC().DeleteGitProvider(ctx, &rmetav1.DeleteOptions{
				Uid: item.Metadata.Uid,
			})
			if err != nil {
				if !grpcerr.IsNotFound(err) {
					return err
				}
			}
		}
	}

	if _, err := s.octeliumC.CordiumC().DeleteSpace(ctx, &rmetav1.DeleteOptions{Uid: itm.Metadata.Uid}); err != nil {
		return err
	}

	return nil
}
