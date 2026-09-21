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

package cordium

import (
	"os"
	"sort"
	"strings"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// WorkspaceOption is the shared building block of the Workspace and the
// Template specs. Since a Template is a reusable Workspace configuration, the
// same options describe both, and [TemplateClient.Create] rejects the few
// options that only a Workspace can carry.
type WorkspaceOption func(*specBuilder) error

type specBuilder struct {
	spec        *cordiumv1.Workspace_Spec
	displayName string
	templateRef *metav1.ObjectReference
	snapshotRef *metav1.ObjectReference
	gitProvider string
}

func newSpecBuilder(opts ...WorkspaceOption) (*specBuilder, error) {
	ret := &specBuilder{
		spec: &cordiumv1.Workspace_Spec{},
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(ret); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func (b *specBuilder) runtime() *cordiumv1.Workspace_Spec_Runtime {
	if b.spec.Runtime == nil {
		b.spec.Runtime = &cordiumv1.Workspace_Spec_Runtime{}
	}
	return b.spec.Runtime
}

func (b *specBuilder) repository() *cordiumv1.Workspace_Spec_Repository {
	if b.spec.Repository == nil {
		b.spec.Repository = &cordiumv1.Workspace_Spec_Repository{}
	}
	return b.spec.Repository
}

func (b *specBuilder) limit() *cordiumv1.Workspace_Spec_Limit {
	if b.spec.Limit == nil {
		b.spec.Limit = &cordiumv1.Workspace_Spec_Limit{}
	}
	return b.spec.Limit
}

func (b *specBuilder) capabilities() *cordiumv1.Workspace_Spec_Runtime_Capabilities {
	rt := b.runtime()
	if rt.Capabilities == nil {
		rt.Capabilities = &cordiumv1.Workspace_Spec_Runtime_Capabilities{}
	}
	return rt.Capabilities
}

func (b *specBuilder) egress() *cordiumv1.Workspace_Spec_Runtime_Network_Egress {
	rt := b.runtime()
	if rt.Network == nil {
		rt.Network = &cordiumv1.Workspace_Spec_Runtime_Network{}
	}
	if rt.Network.Egress == nil {
		rt.Network.Egress = &cordiumv1.Workspace_Spec_Runtime_Network_Egress{}
	}
	return rt.Network.Egress
}

func (b *specBuilder) octeliumRuntime() *cordiumv1.Workspace_Spec_Runtime_Octelium {
	rt := b.runtime()
	if rt.Octelium == nil {
		rt.Octelium = &cordiumv1.Workspace_Spec_Runtime_Octelium{}
	}
	return rt.Octelium
}

/*
 * Base spec
 */

// FromSpec starts from an existing Workspace spec, which the later options then
// refine. It is the escape hatch for the specs that are built by hand, loaded
// from a configuration store or copied from another Workspace. The spec is
// cloned, so the caller's copy is never mutated.
func FromSpec(spec *cordiumv1.Workspace_Spec) WorkspaceOption {
	return func(b *specBuilder) error {
		if spec == nil {
			return invalidArgumentf("nil Workspace spec")
		}
		b.spec = proto.Clone(spec).(*cordiumv1.Workspace_Spec)
		return nil
	}
}

// FromSpecJSON starts from a Workspace spec that is encoded as protobuf JSON.
func FromSpecJSON(data []byte) WorkspaceOption {
	return func(b *specBuilder) error {
		spec := &cordiumv1.Workspace_Spec{}
		if err := protojson.Unmarshal(data, spec); err != nil {
			return invalidArgumentf("could not decode the Workspace spec: %v", err)
		}
		b.spec = spec
		return nil
	}
}

// WithSpecFunc mutates the spec that the previous options produced. It is the
// escape hatch for the fields that this SDK does not expose as options yet.
func WithSpecFunc(fn func(*cordiumv1.Workspace_Spec) error) WorkspaceOption {
	return func(b *specBuilder) error {
		if fn == nil {
			return invalidArgumentf("nil spec function")
		}
		return fn(b.spec)
	}
}

// WithDisplayName sets a human readable name. Workspaces are always given a
// short randomly generated name by the Cluster, so the display name is the only
// naming that an application controls.
func WithDisplayName(displayName string) WorkspaceOption {
	return func(b *specBuilder) error {
		b.displayName = displayName
		return nil
	}
}

// WithTemplate creates the Workspace from a specific Template, whose own spec
// it inherits. The name can be short (e.g. "ci-runner") or qualified with its
// Space (e.g. "ci-runner.my-project"). Without it, the User's default Template
// is used.
func WithTemplate(name string) WorkspaceOption {
	return func(b *specBuilder) error {
		if name == "" {
			return invalidArgumentf("empty Template name")
		}
		b.templateRef = &metav1.ObjectReference{Name: name}
		return nil
	}
}

// FromSnapshot restores the Workspace's persistent storage from a
// WorkspaceSnapshot instead of initializing it from scratch. The snapshot must
// be READY. Unless [WithTemplate] or [WithSpace] is also given, the Workspace
// is created from the Template of the snapshot's source Workspace, and an
// explicit Template must belong to the Space of that source Workspace. The
// restored Workspace runs in the Region that holds the snapshot.
func FromSnapshot(name string) WorkspaceOption {
	return func(b *specBuilder) error {
		if name == "" {
			return invalidArgumentf("empty WorkspaceSnapshot name")
		}
		b.snapshotRef = &metav1.ObjectReference{Name: name}
		return nil
	}
}

// WithVolume mounts a Volume of the Workspace's Space at an absolute path
// inside the Workspace. The Volume must already exist and it must belong to
// the same Space as the Workspace.
//
//	ws, err := c.Workspaces().Create(ctx,
//		cordium.WithSpace("my-project"),
//		cordium.WithVolume("datasets", "/data"))
//
// The mount path cannot be the root directory, it cannot overlap with another
// mount and it cannot cover the paths that are reserved by the Cluster. Use
// [WithReadOnlyVolume] in order to mount the Volume read-only.
func WithVolume(name, mountPath string) WorkspaceOption {
	return withVolume(name, mountPath, false)
}

// WithReadOnlyVolume mounts a Volume read-only inside the Workspace. It is set
// per mount, so the same Volume can simultaneously be mounted read-write by a
// Workspace and read-only by another one.
func WithReadOnlyVolume(name, mountPath string) WorkspaceOption {
	return withVolume(name, mountPath, true)
}

func withVolume(name, mountPath string, readOnly bool) WorkspaceOption {
	return func(b *specBuilder) error {
		if name == "" {
			return invalidArgumentf("empty Volume name")
		}
		if mountPath == "" {
			return invalidArgumentf("empty Volume mountPath")
		}
		if !strings.HasPrefix(mountPath, "/") {
			return invalidArgumentf("the Volume mountPath %q must be absolute", mountPath)
		}

		runtime := b.runtime()
		runtime.VolumeMounts = append(runtime.VolumeMounts,
			&cordiumv1.Workspace_Spec_Runtime_VolumeMount{
				VolumeRef: &metav1.ObjectReference{Name: name},
				MountPath: mountPath,
				ReadOnly:  readOnly,
			})
		return nil
	}
}

// WithSpace creates the Workspace from the default Template of a Space. It is
// shorthand for WithTemplate("default." + name).
func WithSpace(name string) WorkspaceOption {
	return func(b *specBuilder) error {
		if name == "" {
			return invalidArgumentf("empty Space name")
		}
		b.templateRef = &metav1.ObjectReference{Name: "default." + name}
		return nil
	}
}

/*
 * Image
 */

// WithImage pulls a pre-built image from a container registry (e.g.
// "ubuntu:24.04" or "registry.example.com/dev/base:latest").
func WithImage(ref string) WorkspaceOption {
	return func(b *specBuilder) error {
		if ref == "" {
			return invalidArgumentf("empty image reference")
		}
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Registry_{
				Registry: &cordiumv1.Workspace_Spec_Image_Registry{Url: ref},
			},
		}
		return nil
	}
}

// WithPrivateImage pulls a pre-built image from a registry that requires
// authentication. The password is read by the Cluster from a Secret of the
// Workspace's Space at initialization time, so no credential ever travels
// through the application.
func WithPrivateImage(ref, username, passwordSecret string) WorkspaceOption {
	return func(b *specBuilder) error {
		if ref == "" {
			return invalidArgumentf("empty image reference")
		}
		if passwordSecret == "" {
			return invalidArgumentf("empty registry password Secret name")
		}
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Registry_{
				Registry: &cordiumv1.Workspace_Spec_Image_Registry{
					Url: ref,
					Authentication: &cordiumv1.Workspace_Spec_Image_Registry_Authentication{
						Username: username,
						Password: &cordiumv1.Workspace_Spec_Image_Registry_Authentication_Password{
							Type: &cordiumv1.Workspace_Spec_Image_Registry_Authentication_Password_FromSecret{
								FromSecret: passwordSecret,
							},
						},
					},
				},
			},
		}
		return nil
	}
}

// WithDockerfile builds the image from the content of a Dockerfile that is
// provided inline. COPY and ADD of local paths are not supported since no build
// context is uploaded.
func WithDockerfile(content string) WorkspaceOption {
	return func(b *specBuilder) error {
		if content == "" {
			return invalidArgumentf("empty Dockerfile content")
		}
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_{
				Dockerfile: &cordiumv1.Workspace_Spec_Image_Dockerfile{
					Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_Inline{Inline: content},
				},
			},
		}
		return nil
	}
}

// WithDockerfileFile reads a local Dockerfile and embeds its content inline.
func WithDockerfileFile(path string) WorkspaceOption {
	return func(b *specBuilder) error {
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return WithDockerfile(string(content))(b)
	}
}

// WithDockerfileURL builds the image from a Dockerfile that the Cluster
// downloads from a URL.
func WithDockerfileURL(url string) WorkspaceOption {
	return func(b *specBuilder) error {
		if url == "" {
			return invalidArgumentf("empty Dockerfile URL")
		}
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_{
				Dockerfile: &cordiumv1.Workspace_Spec_Image_Dockerfile{
					Type: &cordiumv1.Workspace_Spec_Image_Dockerfile_Url{Url: url},
				},
			},
		}
		return nil
	}
}

// ImageGitOption configures [WithImageFromGit].
type ImageGitOption func(*cordiumv1.Workspace_Spec_Image_Git) error

// ImageGitCheckout checks out a branch, a tag or a commit of the image
// repository after cloning it.
func ImageGitCheckout(ref string) ImageGitOption {
	return func(g *cordiumv1.Workspace_Spec_Image_Git) error {
		g.Checkout = ref
		return nil
	}
}

// ImageGitDockerfile is the path of the Dockerfile inside the image repository.
// Without it, Cordium looks for a devcontainer spec in the repository instead.
func ImageGitDockerfile(path string) ImageGitOption {
	return func(g *cordiumv1.Workspace_Spec_Image_Git) error {
		g.Dockerfile = path
		return nil
	}
}

// ImageGitContext is the build context directory inside the image repository.
// It defaults to the repository root.
func ImageGitContext(dir string) ImageGitOption {
	return func(g *cordiumv1.Workspace_Spec_Image_Git) error {
		g.Context = dir
		return nil
	}
}

// WithImageFromGit builds the image out of a dedicated git repository that is
// separate from the Workspace's own repository.
func WithImageFromGit(url string, opts ...ImageGitOption) WorkspaceOption {
	return func(b *specBuilder) error {
		if url == "" {
			return invalidArgumentf("empty image repository URL")
		}
		git := &cordiumv1.Workspace_Spec_Image_Git{Url: url}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			if err := opt(git); err != nil {
				return err
			}
		}
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Git_{Git: git},
		}
		return nil
	}
}

// WithImageFromRepoDockerfile builds the image from a Dockerfile that lives in
// the Workspace's own primary repository. An empty context defaults to the
// repository root.
func WithImageFromRepoDockerfile(path, contextDir string) WorkspaceOption {
	return func(b *specBuilder) error {
		if path == "" {
			return invalidArgumentf("empty Dockerfile path")
		}
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Repository_{
				Repository: &cordiumv1.Workspace_Spec_Image_Repository{
					Type: &cordiumv1.Workspace_Spec_Image_Repository_Dockerfile_{
						Dockerfile: &cordiumv1.Workspace_Spec_Image_Repository_Dockerfile{
							Path:    path,
							Context: contextDir,
						},
					},
				},
			},
		}
		return nil
	}
}

// WithImageFromRepoDevcontainer builds the image from the Development Container
// spec that the Workspace's own primary repository contains. An empty dirPath
// lets Cordium look the spec up on its own.
func WithImageFromRepoDevcontainer(dirPath string) WorkspaceOption {
	return func(b *specBuilder) error {
		b.spec.Image = &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Repository_{
				Repository: &cordiumv1.Workspace_Spec_Image_Repository{
					Type: &cordiumv1.Workspace_Spec_Image_Repository_Devcontainer_{
						Devcontainer: &cordiumv1.Workspace_Spec_Image_Repository_Devcontainer{
							DirPath: dirPath,
						},
					},
				},
			},
		}
		return nil
	}
}

/*
 * Repositories
 */

// RepoOption configures how a git repository is cloned into a Workspace.
type RepoOption func(*cordiumv1.Workspace_Spec_Repository) error

func (o RepoOption) apply(repo *cordiumv1.Workspace_Spec_Repository) error {
	if o == nil {
		return nil
	}
	return o(repo)
}

func cloneOptions(repo *cordiumv1.Workspace_Spec_Repository) *cordiumv1.Workspace_Spec_Repository_CloneOptions {
	if repo.CloneOptions == nil {
		repo.CloneOptions = &cordiumv1.Workspace_Spec_Repository_CloneOptions{}
	}
	return repo.CloneOptions
}

// WithBranch clones a specific branch instead of the repository's default one.
func WithBranch(branch string) RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		cloneOptions(repo).Branch = branch
		return nil
	}
}

// WithDepth limits the number of the fetched commits. It is only effective
// together with [WithoutLazyUnshallow].
func WithDepth(depth uint32) RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		cloneOptions(repo).Depth = depth
		return nil
	}
}

// WithCheckout checks out a commit or a tag after cloning.
func WithCheckout(ref string) RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		cloneOptions(repo).Checkout = ref
		return nil
	}
}

// WithSingleBranch fetches only the chosen branch instead of every branch.
func WithSingleBranch() RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		cloneOptions(repo).SingleBranch = true
		return nil
	}
}

// WithShallowSubmodules clones the repository's submodules with a depth of one.
func WithShallowSubmodules() RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		cloneOptions(repo).ShallowSubmodules = true
		return nil
	}
}

// WithoutLazyUnshallow disables Cordium's default behavior of performing a
// shallow clone for a faster startup and then fetching the full history
// asynchronously in the background.
func WithoutLazyUnshallow() RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		cloneOptions(repo).DisableLazyUnshallow = true
		return nil
	}
}

// WithRepoAuth authenticates the clone of a private repository with HTTP basic
// authentication whose password the Cluster reads from a Secret of the
// Workspace's Space. It is not needed when the Template is associated with a
// GitProvider, since the User's OAuth2 token is then injected automatically.
func WithRepoAuth(username, passwordSecret string) RepoOption {
	return func(repo *cordiumv1.Workspace_Spec_Repository) error {
		if passwordSecret == "" {
			return invalidArgumentf("empty repository password Secret name")
		}
		repo.Authentication = &cordiumv1.Workspace_Spec_Repository_Authentication{
			Type: &cordiumv1.Workspace_Spec_Repository_Authentication_Http{
				Http: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP{
					Username: username,
					Password: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password{
						Type: &cordiumv1.Workspace_Spec_Repository_Authentication_HTTP_Password_FromSecret{
							FromSecret: passwordSecret,
						},
					},
				},
			},
		}
		return nil
	}
}

// WithRepo clones a primary git repository into /workspace/repo. Only the https
// scheme is supported.
func WithRepo(url string, opts ...RepoOption) WorkspaceOption {
	return func(b *specBuilder) error {
		if url == "" {
			return invalidArgumentf("empty repository URL")
		}
		repo := b.repository()
		repo.Url = url
		for _, opt := range opts {
			if err := opt.apply(repo); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithRepository is an alias for [WithRepo].
func WithRepository(url string, opts ...RepoOption) WorkspaceOption {
	return WithRepo(url, opts...)
}

// WithAdditionalRepo clones a secondary repository alongside the primary one.
// The clonePath is the absolute path inside the Workspace at which it is
// cloned (e.g. "/workspace/additional-repos/shared-libs").
func WithAdditionalRepo(name, clonePath, url string, opts ...RepoOption) WorkspaceOption {
	return func(b *specBuilder) error {
		switch {
		case name == "":
			return invalidArgumentf("empty additional repository name")
		case url == "":
			return invalidArgumentf("empty additional repository URL")
		}

		repo := &cordiumv1.Workspace_Spec_Repository{Url: url}
		for _, opt := range opts {
			if err := opt.apply(repo); err != nil {
				return err
			}
		}

		b.spec.AdditionalRepositories = append(b.spec.AdditionalRepositories,
			&cordiumv1.Workspace_Spec_AdditionalRepository{
				Name:       name,
				ClonePath:  clonePath,
				Repository: repo,
			})
		return nil
	}
}

// WithGitProvider associates a Template with a GitProvider of its Space so that
// the User's stored OAuth2 token is injected into the Template's Workspaces,
// which enables authenticated git operations with no credential configuration
// at all. It only applies to Templates.
func WithGitProvider(name string) WorkspaceOption {
	return func(b *specBuilder) error {
		if name == "" {
			return invalidArgumentf("empty GitProvider name")
		}
		b.gitProvider = name
		return nil
	}
}

/*
 * Runtime
 */

// WithEnv injects an environment variable into the Workspace container and into
// all of its lifecycle Tasks.
func WithEnv(key, value string) WorkspaceOption {
	return func(b *specBuilder) error {
		if key == "" {
			return invalidArgumentf("empty environment variable name")
		}
		rt := b.runtime()
		rt.EnvVars = append(rt.EnvVars, &cordiumv1.Workspace_Spec_Runtime_EnvVar{
			Key:  key,
			Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{Value: value},
		})
		return nil
	}
}

// WithEnvs injects a set of environment variables. They are added in a stable
// order so that two identical maps always produce an identical spec.
func WithEnvs(envs map[string]string) WorkspaceOption {
	return func(b *specBuilder) error {
		for _, key := range sortedKeys(envs) {
			if err := WithEnv(key, envs[key])(b); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithEnvFromSecret injects an environment variable whose value the Cluster
// reads from a Secret of the Workspace's Space at initialization time. The
// value never travels through the application that creates the Workspace.
func WithEnvFromSecret(key, secretName string) WorkspaceOption {
	return func(b *specBuilder) error {
		switch {
		case key == "":
			return invalidArgumentf("empty environment variable name")
		case secretName == "":
			return invalidArgumentf("empty Secret name")
		}
		rt := b.runtime()
		rt.EnvVars = append(rt.EnvVars, &cordiumv1.Workspace_Spec_Runtime_EnvVar{
			Key:  key,
			Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_FromSecret{FromSecret: secretName},
		})
		return nil
	}
}

// TaskOption configures a lifecycle Task.
type TaskOption func(*cordiumv1.Workspace_Spec_Runtime_Task) error

// TaskWorkingDir sets the working directory of the Task. It defaults to the
// Workspace User's home directory.
func TaskWorkingDir(dir string) TaskOption {
	return func(t *cordiumv1.Workspace_Spec_Runtime_Task) error {
		t.WorkingDir = dir
		return nil
	}
}

// TaskEnv sets a Task specific environment variable, which is merged with the
// Workspace level ones.
func TaskEnv(key, value string) TaskOption {
	return func(t *cordiumv1.Workspace_Spec_Runtime_Task) error {
		if key == "" {
			return invalidArgumentf("empty Task environment variable name")
		}
		t.EnvVars = append(t.EnvVars, &cordiumv1.Workspace_Spec_Runtime_Task_EnvVar{
			Key:   key,
			Value: value,
		})
		return nil
	}
}

// TaskInBackground starts the Task and lets the initialization of the Workspace
// proceed without waiting for the Task to complete. It is what dev servers and
// the other long running processes need.
func TaskInBackground() TaskOption {
	return func(t *cordiumv1.Workspace_Spec_Runtime_Task) error {
		t.IsBackground = true
		return nil
	}
}

// TaskAsRoot runs the Task as root instead of as the Workspace User.
func TaskAsRoot() TaskOption {
	return func(t *cordiumv1.Workspace_Spec_Runtime_Task) error {
		t.RunAsRoot = true
		return nil
	}
}

// TaskAbortOnFailure aborts the initialization of the Workspace once the Task
// fails.
func TaskAbortOnFailure() TaskOption {
	return func(t *cordiumv1.Workspace_Spec_Runtime_Task) error {
		t.OnFailure = cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT
		return nil
	}
}

// TaskContinueOnFailure logs the failure of the Task and carries on with the
// initialization of the Workspace.
func TaskContinueOnFailure() TaskOption {
	return func(t *cordiumv1.Workspace_Spec_Runtime_Task) error {
		t.OnFailure = cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_CONTINUE
		return nil
	}
}

func withTask(typ cordiumv1.Workspace_Spec_Runtime_Task_Type,
	name, run string, opts ...TaskOption) WorkspaceOption {
	return func(b *specBuilder) error {
		switch {
		case name == "":
			return invalidArgumentf("empty Task name")
		case run == "":
			return invalidArgumentf("empty Task command")
		}

		task := &cordiumv1.Workspace_Spec_Runtime_Task{
			Name: name,
			Run:  run,
			Type: typ,
		}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			if err := opt(task); err != nil {
				return err
			}
		}

		rt := b.runtime()
		rt.Tasks = append(rt.Tasks, task)
		return nil
	}
}

// WithTask adds an ON_CREATE lifecycle Task, which only runs on a fresh run:
// the first start of a persistent Workspace and every start of an ephemeral
// one. It is where dependency installation and the other one-time setup belong.
//
// [WithPostStartTask] and [WithPreStopTask] cover the other two points of the
// lifecycle.
func WithTask(name, run string, opts ...TaskOption) WorkspaceOption {
	return withTask(cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE, name, run, opts...)
}

// WithOnCreateTask is an explicit alias for [WithTask].
func WithOnCreateTask(name, run string, opts ...TaskOption) WorkspaceOption {
	return withTask(cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE, name, run, opts...)
}

// WithPostStartTask adds a Task that runs on every start of the Workspace. It
// is where background services and dev servers belong, normally together with
// [TaskInBackground].
func WithPostStartTask(name, run string, opts ...TaskOption) WorkspaceOption {
	return withTask(cordiumv1.Workspace_Spec_Runtime_Task_POST_START, name, run, opts...)
}

// WithPreStopTask adds a Task that runs right before the Workspace's container
// is stopped. It is where graceful shutdown and cleanup belong.
func WithPreStopTask(name, run string, opts ...TaskOption) WorkspaceOption {
	return withTask(cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP, name, run, opts...)
}

// WithCmd overrides the container image's default command.
func WithCmd(cmd string) WorkspaceOption {
	return func(b *specBuilder) error {
		b.runtime().Cmd = cmd
		return nil
	}
}

// WithEntrypoint overrides the container image's default entrypoint.
func WithEntrypoint(entrypoint string) WorkspaceOption {
	return func(b *specBuilder) error {
		b.runtime().Entrypoint = entrypoint
		return nil
	}
}

// WithoutInit disables the minimal init process that Cordium runs as PID 1 in
// order to reap the zombie processes. It should only be used when the image
// already contains its own init system.
func WithoutInit() WorkspaceOption {
	return func(b *specBuilder) error {
		b.runtime().DisableInit = true
		return nil
	}
}

// WithDevcontainerFeature installs a Development Container Feature inside the
// Workspace (e.g. "ghcr.io/devcontainers/features/docker-in-docker:2"). The
// Features are merged with the ones that the repository's own devcontainer spec
// declares.
func WithDevcontainerFeature(reference string, options map[string]string) WorkspaceOption {
	return func(b *specBuilder) error {
		if reference == "" {
			return invalidArgumentf("empty devcontainer Feature reference")
		}

		feature := &cordiumv1.Workspace_Spec_Runtime_Devcontainers_Feature{
			Reference: reference,
		}
		for _, key := range sortedKeys(options) {
			feature.Options = append(feature.Options,
				&cordiumv1.Workspace_Spec_Runtime_Devcontainers_Feature_Option{
					Key:   key,
					Value: options[key],
				})
		}

		rt := b.runtime()
		if rt.Devcontainers == nil {
			rt.Devcontainers = &cordiumv1.Workspace_Spec_Runtime_Devcontainers{}
		}
		rt.Devcontainers.Features = append(rt.Devcontainers.Features, feature)
		return nil
	}
}

// WithOcteliumServices serves the named Octelium Services, among the ones that
// are assigned to the owner User, inside the Workspace. It is what gives the
// Workspace secretless access to the Cluster's Services (e.g. databases and
// internal APIs) with no credential of its own.
func WithOcteliumServices(names ...string) WorkspaceOption {
	return func(b *specBuilder) error {
		oct := b.octeliumRuntime()
		oct.ServeServices = append(oct.ServeServices, names...)
		return nil
	}
}

// WithAllOcteliumServices serves every Octelium Service that is assigned to the
// owner User inside the Workspace.
func WithAllOcteliumServices() WorkspaceOption {
	return func(b *specBuilder) error {
		b.octeliumRuntime().ServeAll = true
		return nil
	}
}

// WithReadOnlyRootFilesystem makes the container's root filesystem read-only.
func WithReadOnlyRootFilesystem() WorkspaceOption {
	return func(b *specBuilder) error {
		rt := b.runtime()
		if rt.Filesystem == nil {
			rt.Filesystem = &cordiumv1.Workspace_Spec_Runtime_Filesystem{}
		}
		rt.Filesystem.ReadOnly = true
		return nil
	}
}

// WithAddedCapabilities adds Linux capabilities to the Workspace container
// (e.g. "NET_ADMIN"). They are merged with the Space level and the Cluster
// level capabilities.
func WithAddedCapabilities(capabilities ...string) WorkspaceOption {
	return func(b *specBuilder) error {
		caps := b.capabilities()
		caps.Add = append(caps.Add, capabilities...)
		return nil
	}
}

// WithDroppedCapabilities drops Linux capabilities from the Workspace container
// (e.g. "NET_RAW").
func WithDroppedCapabilities(capabilities ...string) WorkspaceOption {
	return func(b *specBuilder) error {
		caps := b.capabilities()
		caps.Drop = append(caps.Drop, capabilities...)
		return nil
	}
}

// WithoutTimeout disables the inactivity timeout after which the Cluster
// automatically stops a running Workspace. It is only honored when the
// ClusterConfig allows Workspaces to have no timeout.
func WithoutTimeout() WorkspaceOption {
	return func(b *specBuilder) error {
		b.runtime().Timeout = &cordiumv1.Workspace_Spec_Runtime_Timeout{
			Mode: cordiumv1.Workspace_Spec_Runtime_Timeout_DISABLED,
		}
		return nil
	}
}

// WithDefaultTimeout applies the Cluster's inactivity timeout, which is also
// what an unset timeout does.
func WithDefaultTimeout() WorkspaceOption {
	return func(b *specBuilder) error {
		b.runtime().Timeout = &cordiumv1.Workspace_Spec_Runtime_Timeout{
			Mode: cordiumv1.Workspace_Spec_Runtime_Timeout_DEFAULT,
		}
		return nil
	}
}

// WithAutoStop stops the Workspace automatically once all of its non-background
// lifecycle Tasks complete. It is what turns a Workspace into a one-shot job,
// which is how CI/CD runs and unattended agent tasks are normally modeled.
func WithAutoStop() WorkspaceOption {
	return func(b *specBuilder) error {
		b.runtime().AutoStop = true
		return nil
	}
}

/*
 * Network
 */

// WithEgressDefaultDeny denies every outbound destination that no rule added
// with [WithEgressAllow] matches.
func WithEgressDefaultDeny() WorkspaceOption {
	return func(b *specBuilder) error {
		b.egress().DefaultAction = cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY
		return nil
	}
}

// WithEgressDefaultAllowPublic allows the outbound traffic to the publicly
// routable networks and denies it to the private ones. It is the default.
func WithEgressDefaultAllowPublic() WorkspaceOption {
	return func(b *specBuilder) error {
		b.egress().DefaultAction = cordiumv1.Workspace_Spec_Runtime_Network_Egress_ALLOW_PUBLIC
		return nil
	}
}

// WithEgressAllow allows the outbound traffic to a set of network ranges, in
// CIDR notation, and optionally only to a set of destination ports. An empty
// port list matches every port.
//
// The egress rules are unordered and a DENY always wins over an ALLOW.
func WithEgressAllow(cidrs []string, ports ...uint32) WorkspaceOption {
	return egressRule(cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW, cidrs, ports)
}

// WithEgressDeny denies the outbound traffic to a set of network ranges, in
// CIDR notation, and optionally only to a set of destination ports. A
// destination that a DENY rule matches is denied even when an ALLOW rule
// matches it too.
func WithEgressDeny(cidrs []string, ports ...uint32) WorkspaceOption {
	return egressRule(cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY, cidrs, ports)
}

func egressRule(action cordiumv1.Workspace_Spec_Runtime_Network_Rule_Action,
	cidrs []string, ports []uint32) WorkspaceOption {
	return func(b *specBuilder) error {
		if len(cidrs) == 0 {
			return invalidArgumentf("an egress rule needs at least one CIDR")
		}
		egress := b.egress()
		egress.Rules = append(egress.Rules, &cordiumv1.Workspace_Spec_Runtime_Network_Rule{
			Cidrs:  append([]string(nil), cidrs...),
			Action: action,
			Ports:  append([]uint32(nil), ports...),
		})
		return nil
	}
}

/*
 * Applications
 */

// AppOption configures an Application, which is a named port of the Workspace.
type AppOption func(*cordiumv1.Workspace_Spec_Application) error

// AppDisplayName sets a human readable name for the Application.
func AppDisplayName(displayName string) AppOption {
	return func(a *cordiumv1.Workspace_Spec_Application) error {
		a.DisplayName = displayName
		return nil
	}
}

// AsDefaultApp serves the Application at the Workspace's root hostname. At most
// one Application can be the default one.
func AsDefaultApp() AppOption {
	return func(a *cordiumv1.Workspace_Spec_Application) error {
		a.IsDefault = true
		return nil
	}
}

// WithApp exposes a named port of the Workspace through the Cordium portal's
// reverse proxy. The name is used as a subdomain prefix of the Workspace's
// hostname and it must be unique within the spec.
func WithApp(name string, port int, opts ...AppOption) WorkspaceOption {
	return func(b *specBuilder) error {
		switch {
		case name == "":
			return invalidArgumentf("empty Application name")
		case port < 1 || port > 65535:
			return invalidArgumentf("Application port must be between 1 and 65535, got %d", port)
		}

		app := &cordiumv1.Workspace_Spec_Application{
			Name: name,
			Port: int32(port),
		}
		for _, opt := range opts {
			if opt == nil {
				continue
			}
			if err := opt(app); err != nil {
				return err
			}
		}

		b.spec.Applications = append(b.spec.Applications, app)
		return nil
	}
}

// WithApplication is an alias for [WithApp].
func WithApplication(name string, port int, opts ...AppOption) WorkspaceOption {
	return WithApp(name, port, opts...)
}

// WithPort exposes a port under an automatically derived name (e.g. "port-3000"
// for the port 3000).
func WithPort(port int) WorkspaceOption {
	return func(b *specBuilder) error {
		if port < 1 || port > 65535 {
			return invalidArgumentf("Application port must be between 1 and 65535, got %d", port)
		}
		return WithApp(portAppName(port), port)(b)
	}
}

/*
 * Limits
 */

// WithCPU allocates CPU in millicores, where one core equals 1000 millicores.
// The effective limits are resolved by precedence (i.e. Workspace, Template,
// Space default and then the Cluster default) and then capped by the Space and
// the Cluster maximums.
func WithCPU(millicores uint32) WorkspaceOption {
	return func(b *specBuilder) error {
		b.limit().Cpu = &cordiumv1.Workspace_Spec_Limit_CPU{Millicores: millicores}
		return nil
	}
}

// WithMemory allocates memory in megabytes.
func WithMemory(megabytes uint32) WorkspaceOption {
	return func(b *specBuilder) error {
		b.limit().Memory = &cordiumv1.Workspace_Spec_Limit_Memory{Megabytes: megabytes}
		return nil
	}
}

// WithStorage allocates disk storage in megabytes.
func WithStorage(megabytes uint32) WorkspaceOption {
	return func(b *specBuilder) error {
		b.limit().Storage = &cordiumv1.Workspace_Spec_Limit_Storage{Megabytes: megabytes}
		return nil
	}
}

// Resources is the compute allocation of a Workspace. A zero field means that
// the dimension is left to the Template, the Space or the Cluster default.
type Resources struct {
	// CPUMillicores is the CPU allocation, where one core equals 1000.
	CPUMillicores uint32
	// MemoryMB is the memory allocation in megabytes.
	MemoryMB uint32
	// StorageMB is the disk storage allocation in megabytes.
	StorageMB uint32
}

// WithResources allocates several compute dimensions at once.
func WithResources(resources Resources) WorkspaceOption {
	return func(b *specBuilder) error {
		if resources.CPUMillicores > 0 {
			if err := WithCPU(resources.CPUMillicores)(b); err != nil {
				return err
			}
		}
		if resources.MemoryMB > 0 {
			if err := WithMemory(resources.MemoryMB)(b); err != nil {
				return err
			}
		}
		if resources.StorageMB > 0 {
			if err := WithStorage(resources.StorageMB)(b); err != nil {
				return err
			}
		}
		return nil
	}
}

/*
 * Variables and storage mode
 */

// WithVar defines a variable that the spec's string fields can reference with
// the "${{ vars.NAME }}" syntax. The substitution happens after all the
// configuration levels are merged, which is what makes a single Template
// serve many parameterized runs.
func WithVar(name, value string) WorkspaceOption {
	return func(b *specBuilder) error {
		if name == "" {
			return invalidArgumentf("empty variable name")
		}
		b.spec.Vars = append(b.spec.Vars, &cordiumv1.Workspace_Spec_Var{
			Name:  name,
			Value: value,
		})
		return nil
	}
}

// WithVars defines a set of variables. They are added in a stable order so that
// two identical maps always produce an identical spec.
func WithVars(vars map[string]string) WorkspaceOption {
	return func(b *specBuilder) error {
		for _, name := range sortedKeys(vars) {
			if err := WithVar(name, vars[name])(b); err != nil {
				return err
			}
		}
		return nil
	}
}

// Ephemeral discards the Workspace's storage once it is stopped, so that every
// start provisions a fresh volume and runs the full initialization from
// scratch. It is what one-shot sandboxes, CI runs and untrusted agent tasks
// normally want.
func Ephemeral() WorkspaceOption {
	return func(b *specBuilder) error {
		b.spec.IsEphemeral = true
		return nil
	}
}

// Persistent preserves the Workspace's filesystem across stops and restarts. It
// is the default.
func Persistent() WorkspaceOption {
	return func(b *specBuilder) error {
		b.spec.IsEphemeral = false
		return nil
	}
}

func sortedKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	ret := make([]string, 0, len(m))
	for key := range m {
		ret = append(ret, key)
	}
	sort.Strings(ret)
	return ret
}
