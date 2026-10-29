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

package gitutils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/stretchr/testify/assert"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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

func localGitRepository(t *testing.T) (string, []plumbing.Hash) {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	var commits []plumbing.Hash
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte(fmt.Sprintf("version%d", i)), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := tree.Add("file.txt"); err != nil {
			t.Fatal(err)
		}
		hash, err := tree.Commit(fmt.Sprintf("commit%d", i), &git.CommitOptions{Author: &object.Signature{Name: "Test", Email: "test@example.com", When: time.Unix(int64(i+1), 0)}})
		if err != nil {
			t.Fatal(err)
		}
		commits = append(commits, hash)
	}
	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.NewBranchReferenceName("feature"), commits[1])); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CreateTag("v1.0.0", commits[0], nil); err != nil {
		t.Fatal(err)
	}
	return "file://" + dir, commits
}

func TestLocalGitCloneOptions(t *testing.T) {
	url, commits := localGitRepository(t)
	for _, tc := range []struct {
		name    string
		clone   func(context.Context, *CloneOpts) error
		options *cordiumv1.Workspace_Spec_Repository_CloneOptions
		count   int
		head    plumbing.Hash
	}{
		{name: "command lazy", clone: cloneCmd, count: 1, head: commits[2]},
		{name: "command explicit depth", clone: cloneCmd, options: &cordiumv1.Workspace_Spec_Repository_CloneOptions{DisableLazyUnshallow: true, Depth: 2}, count: 2, head: commits[2]},
		{name: "command full", clone: cloneCmd, options: &cordiumv1.Workspace_Spec_Repository_CloneOptions{DisableLazyUnshallow: true}, count: 3, head: commits[2]},
		{name: "command branch", clone: cloneCmd, options: &cordiumv1.Workspace_Spec_Repository_CloneOptions{Branch: "feature", DisableLazyUnshallow: true}, count: 3, head: commits[1]},
		{name: "embedded full", clone: cloneEmbedded, options: &cordiumv1.Workspace_Spec_Repository_CloneOptions{DisableLazyUnshallow: true}, count: 3, head: commits[2]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := &CloneOpts{Dir: filepath.Join(t.TempDir(), "clone"), Repo: &cordiumv1.Workspace_Spec_Repository{Url: url, CloneOptions: tc.options}}
			if err := tc.clone(t.Context(), opts); err != nil {
				t.Fatal(err)
			}
			repo, err := git.PlainOpen(opts.Dir)
			if err != nil {
				t.Fatal(err)
			}
			head, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tc.head, head.Hash())
			iter, err := repo.CommitObjects()
			if err != nil {
				t.Fatal(err)
			}
			defer iter.Close()
			count := 0
			assert.Nil(t, iter.ForEach(func(*object.Commit) error { count++; return nil }))
			assert.Equal(t, tc.count, count)
		})
	}
}

func TestLocalGitCheckoutReferences(t *testing.T) {
	url, commits := localGitRepository(t)
	for _, tc := range []struct {
		name     string
		checkout func(context.Context, *CloneOpts) error
	}{
		{name: "command", checkout: checkoutCmd},
		{name: "embedded", checkout: checkoutEmbedded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, ref := range []string{commits[0].String(), "v1.0.0"} {
				t.Run(ref, func(t *testing.T) {
					opts := &CloneOpts{Dir: filepath.Join(t.TempDir(), "clone"), Repo: &cordiumv1.Workspace_Spec_Repository{Url: url, CloneOptions: &cordiumv1.Workspace_Spec_Repository_CloneOptions{DisableLazyUnshallow: true, Checkout: ref}}}
					if err := cloneCmd(t.Context(), opts); err != nil {
						t.Fatal(err)
					}
					if err := tc.checkout(t.Context(), opts); !assert.Nil(t, err) {
						return
					}
					repo, err := git.PlainOpen(opts.Dir)
					if err != nil {
						t.Fatal(err)
					}
					head, err := repo.Head()
					if err != nil {
						t.Fatal(err)
					}
					assert.Equal(t, commits[0], head.Hash())
				})
			}
		})
	}
}

func TestLocalGitUnshallow(t *testing.T) {
	url, _ := localGitRepository(t)
	for _, tc := range []struct {
		name      string
		unshallow func(context.Context, *CloneOpts) error
	}{
		{name: "command", unshallow: unshallowCmd},
		{name: "embedded", unshallow: unshallowEmbedded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := &CloneOpts{Dir: filepath.Join(t.TempDir(), "clone"), Repo: &cordiumv1.Workspace_Spec_Repository{Url: url}}
			if err := cloneCmd(t.Context(), opts); err != nil {
				t.Fatal(err)
			}
			if err := tc.unshallow(t.Context(), opts); !assert.Nil(t, err) {
				return
			}
			repo, err := git.PlainOpen(opts.Dir)
			if err != nil {
				t.Fatal(err)
			}
			iter, err := repo.CommitObjects()
			if err != nil {
				t.Fatal(err)
			}
			defer iter.Close()
			count := 0
			assert.Nil(t, iter.ForEach(func(*object.Commit) error { count++; return nil }))
			assert.Equal(t, 3, count)
			assert.Nil(t, tc.unshallow(t.Context(), opts))
		})
	}
}

func TestGitEmptyRequestsAndCancellation(t *testing.T) {
	assert.NotNil(t, Clone(t.Context(), nil))
	assert.NotNil(t, Unshallow(t.Context(), nil))
	assert.Nil(t, Checkout(t.Context(), nil))
	for _, opts := range []*CloneOpts{{}, {Repo: &cordiumv1.Workspace_Spec_Repository{}}} {
		assert.Nil(t, Clone(t.Context(), opts))
		assert.Nil(t, Unshallow(t.Context(), opts))
		assert.Nil(t, Checkout(t.Context(), opts))
	}
	url, _ := localGitRepository(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	assert.NotNil(t, Clone(ctx, &CloneOpts{Dir: filepath.Join(t.TempDir(), "clone"), Repo: &cordiumv1.Workspace_Spec_Repository{Url: url}}))
}
