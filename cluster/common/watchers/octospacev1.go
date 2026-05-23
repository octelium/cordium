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

package watchers

import (
	"context"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/watchers"
	"github.com/octelium/octelium/pkg/apiutils/umetav1"
)

type CordiumV1Watcher struct {
	octeliumC octeliumc.ClientInterface
}

func NewCordiumV1(octeliumC octeliumc.ClientInterface) *CordiumV1Watcher {
	return &CordiumV1Watcher{
		octeliumC: octeliumC,
	}
}

func (c *CordiumV1Watcher) Workspace(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Workspace) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Workspace) error,
	onDelete func(ctx context.Context, item *cordiumv1.Workspace) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindWorkspace, onCreate, onUpdate, onDelete)
}

func (c *CordiumV1Watcher) Secret(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Secret) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Secret) error,
	onDelete func(ctx context.Context, item *cordiumv1.Secret) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindSecret, onCreate, onUpdate, onDelete)
}

/*
func (c *CordiumV1Watcher) Environment(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Environment) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Environment) error,
	onDelete func(ctx context.Context, item *cordiumv1.Environment) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindEnvironment, onCreate, onUpdate, onDelete)
}
*/

func (c *CordiumV1Watcher) Template(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Template) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Template) error,
	onDelete func(ctx context.Context, item *cordiumv1.Template) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindTemplate, onCreate, onUpdate, onDelete)
}

/*
func (c *CordiumV1Watcher) Build(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Build) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Build) error,
	onDelete func(ctx context.Context, item *cordiumv1.Build) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindBuild, onCreate, onUpdate, onDelete)
}
*/

func (c *CordiumV1Watcher) Space(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Space) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Space) error,
	onDelete func(ctx context.Context, item *cordiumv1.Space) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindSpace, onCreate, onUpdate, onDelete)
}

func (c *CordiumV1Watcher) Membership(
	ctx context.Context,
	opts *watchers.Opts,
	onCreate func(ctx context.Context, item *cordiumv1.Membership) error,
	onUpdate func(ctx context.Context, new, old *cordiumv1.Membership) error,
	onDelete func(ctx context.Context, item *cordiumv1.Membership) error,
) error {
	return runWatcherCordiumV1(ctx, c.octeliumC, opts, ucordiumv1.KindMembership, onCreate, onUpdate, onDelete)
}

func runWatcherCordiumV1[T ucordiumv1.ResourceObjectRefG](
	ctx context.Context, octeliumC octeliumc.ClientInterface,
	opts *watchers.Opts,
	kind string,
	onCreate func(ctx context.Context, item T) error,
	onUpdate func(ctx context.Context, new, old T) error,
	onDelete func(ctx context.Context, item T) error,
) error {

	var doOnCreate func(ctx context.Context, itm umetav1.ResourceObjectI) error
	var doOnUpdate func(ctx context.Context, new, old umetav1.ResourceObjectI) error
	var doOnDelete func(ctx context.Context, itm umetav1.ResourceObjectI) error

	if onCreate != nil {
		doOnCreate = func(ctx context.Context, itm umetav1.ResourceObjectI) error {
			return onCreate(ctx, itm.(T))
		}
	}

	if onUpdate != nil {
		doOnUpdate = func(ctx context.Context, new, old umetav1.ResourceObjectI) error {
			return onUpdate(ctx, new.(T), old.(T))
		}
	}

	if onDelete != nil {
		doOnDelete = func(ctx context.Context, itm umetav1.ResourceObjectI) error {
			return onDelete(ctx, itm.(T))
		}
	}

	watcher, err := watchers.NewWatcher(ucordiumv1.API, ucordiumv1.Version, kind,
		doOnCreate, doOnUpdate, doOnDelete,
		octeliumC.CordiumC(), func() (umetav1.ResourceObjectI, error) {
			return ucordiumv1.NewObject(kind)
		},
	)
	if err != nil {
		return err
	}
	return watcher.Run(ctx)
}
