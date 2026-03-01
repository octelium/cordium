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

package gitutils

import (
	"os"
	"testing"
	"time"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/stretchr/testify/assert"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func TestGitCmd(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		opts := &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
					Checkout: "fix_pango",
				},
			},
		}
		err = cloneCmd(ctx, opts)
		assert.Nil(t, err)

		{
			repo, err := git.PlainOpen(tmpDir)
			assert.Nil(t, err)
			iter, err := repo.CommitObjects()
			assert.Nil(t, err)

			i := 0
			iter.ForEach(func(c *object.Commit) error {
				i = i + 1
				return nil
			})
			assert.Equal(t, 1, i)
		}

		err = unshallowCmd(ctx, opts)
		assert.Nil(t, err)
		{
			repo, err := git.PlainOpen(tmpDir)
			assert.Nil(t, err)
			iter, err := repo.CommitObjects()
			assert.Nil(t, err)

			i := 0
			iter.ForEach(func(c *object.Commit) error {
				i = i + 1
				return nil
			})
			assert.Greater(t, i, 100)
		}
		{
			err = checkoutCmd(ctx, opts)
			assert.Nil(t, err)

		}

	}

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err = cloneCmd(ctx, &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url:          "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{},
			},
		})
		assert.Nil(t, err)

		repo, err := git.PlainOpen(tmpDir)
		assert.Nil(t, err)
		iter, err := repo.CommitObjects()
		assert.Nil(t, err)

		i := 0
		iter.ForEach(func(c *object.Commit) error {
			i = i + 1
			return nil
		})
		assert.Equal(t, 1, i)
	}

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err = cloneCmd(ctx, &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
					Depth:                2,
					DisableLazyUnshallow: true,
				},
			},
		})
		assert.Nil(t, err)

		repo, err := git.PlainOpen(tmpDir)
		assert.Nil(t, err)
		iter, err := repo.CommitObjects()
		assert.Nil(t, err)

		i := 0
		iter.ForEach(func(c *object.Commit) error {
			i = i + 1
			return nil
		})
		assert.Equal(t, 2, i)
	}

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err = cloneCmd(ctx, &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
					Branch: "fix_pango",
				},
			},
		})
		assert.Nil(t, err)

		repo, err := git.PlainOpen(tmpDir)
		assert.Nil(t, err)
		iter, err := repo.CommitObjects()
		assert.Nil(t, err)
		_, err = repo.Branch("fix_pango")
		assert.Nil(t, err)

		i := 0
		iter.ForEach(func(c *object.Commit) error {
			i = i + 1
			return nil
		})
		assert.Equal(t, 1, i)
	}
}

func TestGitEmbedded(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		opts := &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/geommer/yabar",
			},
		}
		err = cloneEmbedded(ctx, opts)
		assert.Nil(t, err)

		err = unshallowEmbedded(ctx, opts)
		assert.Nil(t, err)

	}

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err = cloneEmbedded(ctx, &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url:          "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{},
			},
		})
		assert.Nil(t, err)

	}

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err = cloneEmbedded(ctx, &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
					Depth:                2,
					DisableLazyUnshallow: true,
				},
			},
		})
		assert.Nil(t, err)

	}

	{
		tmpDir, err := os.MkdirTemp("/tmp", "")
		assert.Nil(t, err)

		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err = cloneEmbedded(ctx, &CloneOpts{
			Dir: tmpDir,
			Repo: &cordiumv1.Workspace_Spec_Repository{
				Url: "https://github.com/geommer/yabar",
				CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{
					Branch: "fix_pango",
				},
			},
		})
		assert.Nil(t, err)

		repo, err := git.PlainOpen(tmpDir)
		assert.Nil(t, err)

		_, err = repo.Branch("fix_pango")
		assert.Nil(t, err)

	}
}
