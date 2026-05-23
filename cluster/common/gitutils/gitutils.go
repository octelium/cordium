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
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"slices"
	"strconv"
	"strings"
	"syscall"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/octelium/cordium/pkg/apiutils/ucordiumv1"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type CloneOpts struct {
	Dir        string
	Repo       *cordiumv1.Workspace_Spec_Repository
	SecretList *cordiumv1.SecretList
	UserUID    int
}

func Unshallow(ctx context.Context, o *CloneOpts) error {
	if o == nil {
		return errors.Errorf("Could not unhshallow fetch. Nil req")
	}

	if o.Repo == nil || o.Repo.Url == "" {
		return nil
	}

	if _, err := exec.LookPath("git"); err == nil {
		if err := unshallowCmd(ctx, o); err == nil {
			return Checkout(ctx, o)
		} else {
			zap.L().Warn("Could not unshallow via git cmd. Trying with embedded...", zap.Error(err))
		}
	}

	if err := unshallowEmbedded(ctx, o); err != nil {
		return err
	}

	return Checkout(ctx, o)
}

func unshallowCmd(ctx context.Context, o *CloneOpts) error {
	zap.L().Debug("Starting unshallow using git cmd", zap.Any("opts", o))

	{
		cmd, err := getGitCmd(ctx, "git rev-parse --is-shallow-repository", o)
		if err != nil {
			return err
		}

		cmd.Dir = o.Dir
		cmd.Stdout = nil
		cmd.Stderr = nil

		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		if !strings.Contains(string(out), "true") {
			zap.L().Debug("The repo does not need an unshallow fetch. Nothing to be done")
			return nil
		}

		zap.L().Debug("The repo is shallow and it needs a fetch", zap.String("out", string(out)))
	}

	{
		cmd, err := getGitCmd(ctx, "git fetch --unshallow", o)
		if err != nil {
			return err
		}

		cmd.Dir = o.Dir

		if idx := slices.IndexFunc(cmd.Env, func(s string) bool {
			return s == "GIT_CONFIG_COUNT=1"
		}); idx >= 0 {
			cmd.Env[idx] = "GIT_CONFIG_COUNT=2"
			cmd.Env = append(cmd.Env, "GIT_CONFIG_KEY_1=remote.origin.fetch")
			cmd.Env = append(cmd.Env, `GIT_CONFIG_VALUE_1=+refs/heads/*:refs/remotes/origin/*`)
		} else {
			cmd.Env = append(cmd.Env, "GIT_CONFIG_COUNT=1")
			cmd.Env = append(cmd.Env, "GIT_CONFIG_KEY_0=remote.origin.fetch")
			cmd.Env = append(cmd.Env, `GIT_CONFIG_VALUE_0=+refs/heads/*:refs/remotes/origin/*`)
		}

		if err := cmd.Run(); err != nil {
			zap.L().Warn("git fetch --unshallow did not run successfully", zap.Error(err))
			return err
		}
	}

	return nil
}

func unshallowEmbedded(ctx context.Context, o *CloneOpts) error {
	zap.L().Debug("Starting embedded unshallow", zap.Any("opts", o))
	repo, err := git.PlainOpen(o.Dir)
	if err != nil {
		return err
	}

	gopts := &git.FetchOptions{}
	if o.Repo.Authentication != nil {
		usernamePassword, err := getUsernamePassword(o)
		if err != nil {
			return err
		}
		gopts.Auth = &http.BasicAuth{
			Username: usernamePassword.username,
			Password: usernamePassword.password,
		}
	}

	if err := repo.FetchContext(ctx, gopts); err != nil {
		return err
	}

	return nil
}

func Clone(ctx context.Context, o *CloneOpts) error {
	if o == nil {
		return errors.Errorf("Could not clone. Nil req")
	}

	if o.Repo == nil || o.Repo.Url == "" {
		return nil
	}

	if _, err := exec.LookPath("git"); err == nil {
		if err := cloneCmd(ctx, o); err == nil {
			return nil
		} else {
			zap.L().Warn("Could not clone via git cmd. Trying with embedded...", zap.Error(err))
		}
	}

	return cloneEmbedded(ctx, o)
}

func cloneCmd(ctx context.Context, o *CloneOpts) error {

	zap.L().Debug("Starting clone using git cmd", zap.Any("opts", o))
	cmdOpts := []string{}
	opts := o.Repo.CloneOptions
	if opts == nil {
		cmdOpts = append(cmdOpts, "--depth 1")
	} else {

		if !opts.DisableLazyUnshallow {
			cmdOpts = append(cmdOpts, "--depth 1")
		} else {
			if opts.Depth > 0 {
				cmdOpts = append(cmdOpts, fmt.Sprintf("--depth %d", opts.Depth))
			}
		}

		if opts.ShallowSubmodules {
			cmdOpts = append(cmdOpts, "--shallow-submodules")
		}

		if opts.SingleBranch {
			cmdOpts = append(cmdOpts, "--single-branch")
		}
	}

	if opts != nil && opts.Branch != "" {
		cmdOpts = append(cmdOpts, fmt.Sprintf("--branch %s", strings.TrimSpace(opts.Branch)))
	}

	cmdStr := fmt.Sprintf("git clone %s %s %s", strings.Join(cmdOpts, " "), strings.TrimSpace(o.Repo.Url), strings.TrimSpace(o.Dir))

	zap.L().Debug("Executing git clone", zap.String("cmd", cmdStr))

	cmd, err := getGitCmd(ctx, cmdStr, o)
	if err != nil {
		return err
	}

	return cmd.Run()
}

func getGitCmd(ctx context.Context, cmdStr string, o *CloneOpts) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)
	if ldflags.IsDev() {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	cmd.Env = []string{
		os.Getenv("PATH"),
	}

	if o.UserUID != 0 {
		usr, err := user.LookupId(fmt.Sprintf("%d", o.UserUID))
		if err != nil {
			return nil, err
		}

		uid, err := strconv.Atoi(usr.Uid)
		if err != nil {
			return nil, err
		}
		gid, err := strconv.Atoi(usr.Gid)
		if err != nil {
			return nil, err
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)},
		}
	}

	if o.Repo.Authentication != nil {
		usernamePassword, err := getUsernamePassword(o)
		if err != nil {
			return nil, err
		}
		// cmd.Env = append(cmd.Env, "GIT_ASKPASS=cordium-git-cred-helper")
		cmd.Env = append(cmd.Env, "GIT_CONFIG_COUNT=1")
		cmd.Env = append(cmd.Env, "GIT_CONFIG_KEY_0=credential.helper")
		cmd.Env = append(cmd.Env, "GIT_CONFIG_VALUE_0=/bin/cordium-git-cred-helper")
		cmd.Env = append(cmd.Env, "CORDIUM_GIT_INTERNAL=true")
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORDIUM_GIT_USERNAME=%s", usernamePassword.username))
		cmd.Env = append(cmd.Env, fmt.Sprintf("CORDIUM_GIT_PASSWORD=%s", usernamePassword.password))
	}

	return cmd, nil
}

func cloneEmbedded(ctx context.Context, o *CloneOpts) error {

	zap.L().Debug("Starting embedded clone", zap.Any("opts", o))
	gopts := &git.CloneOptions{
		URL: strings.TrimSpace(o.Repo.Url),
	}

	opts := o.Repo.CloneOptions

	if opts != nil && opts.Branch != "" {
		gopts.ReferenceName = plumbing.NewBranchReferenceName(strings.TrimSpace(opts.Branch))
	}

	if opts != nil {
		gopts.Depth = int(opts.Depth)
		gopts.ShallowSubmodules = opts.ShallowSubmodules
		gopts.SingleBranch = opts.SingleBranch
	}

	if opts == nil || !opts.DisableLazyUnshallow {
		gopts.Depth = 1
	}

	var auth transport.AuthMethod

	if o.Repo.Authentication != nil {
		usernamePassword, err := getUsernamePassword(o)
		if err != nil {
			return err
		}
		auth = &http.BasicAuth{
			Username: usernamePassword.username,
			Password: usernamePassword.password,
		}
	}

	gopts.Auth = auth

	_, err := git.PlainCloneContext(ctx, strings.TrimSpace(o.Dir), false, gopts)
	if err != nil {
		return err
	}

	return nil
}

type usernamePassword struct {
	username string
	password string
}

func getUsernamePassword(o *CloneOpts) (*usernamePassword, error) {
	switch o.Repo.Authentication.Type.(type) {
	case *cordiumv1.Workspace_Spec_Repository_Authentication_Http:
		basic := o.Repo.Authentication.GetHttp()
		if basic.Password != nil {
			switch basic.Password.Type.(type) {
			case *cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password_FromSecret:
				if o.SecretList != nil {
					secret, err := ucordiumv1.ToSecretList(o.SecretList).
						GetByName(basic.GetPassword().GetFromSecret())
					if err != nil {
						return nil, err
					}
					return &usernamePassword{
						username: basic.Username,
						password: ucordiumv1.ToSecret(secret).GetValueStr(),
					}, nil
				}
			}
		}

	}
	return nil, errors.Errorf("Could not get Git authentication credentials")
}

func Checkout(ctx context.Context, o *CloneOpts) error {
	if o == nil || o.Repo == nil || o.Repo.CloneOptions == nil || o.Repo.CloneOptions.Checkout == "" {
		return nil
	}

	if _, err := exec.LookPath("git"); err == nil {
		if err := checkoutCmd(ctx, o); err == nil {
			return nil
		} else {
			zap.L().Warn("Could not unshallow via git cmd. Trying with embedded...", zap.Error(err))
		}
	}

	return checkoutEmbedded(ctx, o)
}

func checkoutCmd(ctx context.Context, o *CloneOpts) error {
	zap.L().Debug("Starting checkout using git cmd", zap.Any("opts", o))

	cmd, err := getGitCmd(ctx, fmt.Sprintf("git checkout %s", o.Repo.CloneOptions.Checkout), o)
	if err != nil {
		return err
	}

	cmd.Dir = o.Dir

	if idx := slices.IndexFunc(cmd.Env, func(s string) bool {
		return s == "GIT_CONFIG_COUNT=1"
	}); idx >= 0 {
		cmd.Env[idx] = "GIT_CONFIG_COUNT=2"
		cmd.Env = append(cmd.Env, "GIT_CONFIG_KEY_1=remote.origin.fetch")
		cmd.Env = append(cmd.Env, `GIT_CONFIG_VALUE_1=+refs/heads/*:refs/remotes/origin/*`)
	} else {
		cmd.Env = append(cmd.Env, "GIT_CONFIG_COUNT=1")
		cmd.Env = append(cmd.Env, "GIT_CONFIG_KEY_0=remote.origin.fetch")
		cmd.Env = append(cmd.Env, `GIT_CONFIG_VALUE_0=+refs/heads/*:refs/remotes/origin/*`)
	}

	if err := cmd.Run(); err != nil {
		zap.L().Warn("git checkout did not run successfully", zap.Error(err))
		return err
	}

	return nil
}

func checkoutEmbedded(ctx context.Context, o *CloneOpts) error {
	zap.L().Debug("Starting embedded unshallow", zap.Any("opts", o))
	repo, err := git.PlainOpen(o.Dir)
	if err != nil {
		return err
	}

	gopts := &git.FetchOptions{}
	if o.Repo.Authentication != nil {
		usernamePassword, err := getUsernamePassword(o)
		if err != nil {
			return err
		}
		gopts.Auth = &http.BasicAuth{
			Username: usernamePassword.username,
			Password: usernamePassword.password,
		}
	}

	tree, err := repo.Worktree()
	if err != nil {
		return err
	}

	if err := tree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(o.Repo.CloneOptions.Checkout),
	}); err != nil {
		return err
	}

	return nil
}
