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
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestSpaceCreate(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	spc, err := c.Spaces().Create(t.Context(), "my-project",
		AsOrganizationSpace(),
		WithSpaceDisplayName("My Project"),
		WithSpaceDefaultResources(Resources{CPUMillicores: 2000, MemoryMB: 4096}),
		WithSpaceMaxResources(Resources{CPUMillicores: 8000}),
		WithSpaceEnv("REGISTRY", "reg.example.com"),
		WithSpaceEnvFromSecret("NPM_TOKEN", "npm-token"),
		WithSpaceTask("bootstrap", "./bootstrap.sh"),
		WithSpacePostStartTask("agents", "./agents.sh", TaskInBackground()),
		WithSpacePreStopTask("flush", "./flush.sh"),
		WithSpaceAddedCapabilities("NET_ADMIN"),
		WithSpaceDroppedCapabilities("NET_RAW"),
		WithoutSSH(),
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if spc.GetStatus().GetType() != cordiumv1.Space_Status_ORGANIZATION {
		t.Errorf("Space type = %s", spc.GetStatus().GetType())
	}

	fake.mu.Lock()
	sent := fake.lastSpace
	fake.mu.Unlock()

	if got, want := sent.GetMetadata().GetName(), "my-project.cordium"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if sent.GetMetadata().GetDisplayName() != "My Project" {
		t.Errorf("displayName = %q", sent.GetMetadata().GetDisplayName())
	}

	spec := sent.GetSpec()
	switch {
	case spec.GetLimit().GetDefaultLimit().GetCpu().GetMillicores() != 2000:
		t.Errorf("default limit = %v", spec.GetLimit().GetDefaultLimit())
	case spec.GetLimit().GetMaxLimit().GetCpu().GetMillicores() != 8000:
		t.Errorf("max limit = %v", spec.GetLimit().GetMaxLimit())
	case len(spec.GetRuntime().GetEnvVars()) != 2:
		t.Errorf("envVars = %v", spec.GetRuntime().GetEnvVars())
	case spec.GetRuntime().GetEnvVars()[1].GetFromSecret() != "npm-token":
		t.Errorf("secret env var = %v", spec.GetRuntime().GetEnvVars()[1])
	case len(spec.GetRuntime().GetTasks()) != 3:
		t.Errorf("tasks = %v", spec.GetRuntime().GetTasks())
	case len(spec.GetRuntime().GetCapabilities().GetAdd()) != 1:
		t.Errorf("capabilities = %v", spec.GetRuntime().GetCapabilities())
	case !spec.GetAuthorization().GetDisableSSH():
		t.Error("disableSSH was not set")
	}

	if _, err := c.Spaces().Create(t.Context(), ""); !IsInvalidArgument(err) {
		t.Errorf("an empty Space name: %v", err)
	}
}

func TestSpaceListModes(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.Spaces().List(t.Context(),
		MemberSpaces(), OrganizationSpaces(),
		OrderByName(), Descending(), WithPage(2), WithItemsPerPage(50)); err != nil {
		t.Fatalf("List: %v", err)
	}

	fake.mu.Lock()
	req := fake.lastSpaceList
	fake.mu.Unlock()

	switch {
	case req.GetMode() != cordiumv1.ListSpaceOptions_MODE_MEMBER:
		t.Errorf("mode = %s", req.GetMode())
	case req.GetType() != cordiumv1.Space_Status_ORGANIZATION:
		t.Errorf("type = %s", req.GetType())
	case req.GetCommon().GetPage() != 2 || req.GetCommon().GetItemsPerPage() != 50:
		t.Errorf("pagination = %v", req.GetCommon())
	case req.GetCommon().GetOrderBy().GetMode() != metav1.CommonListOptions_OrderBy_DESC:
		t.Errorf("orderBy = %v", req.GetCommon().GetOrderBy())
	}

	if _, err := c.Spaces().List(t.Context(), OwnedSpaces(), UserSpaces(),
		OrderByCreatedAt(), Ascending()); err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := c.Spaces().Leave(t.Context(), "my-project"); err != nil {
		t.Fatalf("Leave: %v", err)
	}
	if err := c.Spaces().Delete(t.Context(), "my-project"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestSpaceUpdateStartsFromTheCurrentSpec(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.Spaces().Update(t.Context(), "my-project.cordium",
		WithSpaceEnv("ADDED", "1")); err != nil {
		t.Fatalf("Update: %v", err)
	}

	fake.mu.Lock()
	sent := fake.lastSpace
	fake.mu.Unlock()

	if len(sent.GetSpec().GetRuntime().GetEnvVars()) != 1 {
		t.Errorf("envVars = %v", sent.GetSpec().GetRuntime().GetEnvVars())
	}
}

func TestTemplateCreateAndUpdate(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.Templates().Create(t.Context(), "ci-runner.my-project",
		WithImage("golang:1.25"),
		WithRepo("https://github.com/org/api"),
		WithGitProvider("github"),
		WithDisplayName("CI runner")); err != nil {
		t.Fatalf("Create: %v", err)
	}

	fake.mu.Lock()
	sent := fake.lastTemplate
	fake.mu.Unlock()

	if sent.GetMetadata().GetName() != "ci-runner.my-project" {
		t.Errorf("name = %q", sent.GetMetadata().GetName())
	}
	if sent.GetSpec().GetGitProvider() != "github" {
		t.Errorf("gitProvider = %q", sent.GetSpec().GetGitProvider())
	}
	if sent.GetMetadata().GetDisplayName() != "CI runner" {
		t.Errorf("displayName = %q", sent.GetMetadata().GetDisplayName())
	}

	// Update starts from the Template's current spec and keeps the GitProvider
	// that the caller did not restate.
	if _, err := c.Templates().Update(t.Context(), "ci-runner.my-project",
		WithEnv("LOG_LEVEL", "debug")); err != nil {
		t.Fatalf("Update: %v", err)
	}

	fake.mu.Lock()
	sent = fake.lastTemplate
	fake.mu.Unlock()

	if sent.GetSpec().GetImage().GetRegistry().GetUrl() != "golang:1.25" {
		t.Error("the update lost the image")
	}
	if sent.GetSpec().GetGitProvider() != "github" {
		t.Error("the update lost the GitProvider")
	}
	if len(sent.GetSpec().GetRuntime().GetEnvVars()) != 1 {
		t.Errorf("envVars = %v", sent.GetSpec().GetRuntime().GetEnvVars())
	}

	if _, err := c.Templates().Create(t.Context(), "bad", WithApp("web", 3000)); !IsInvalidArgument(err) {
		t.Errorf("a Template with an Application: %v", err)
	}
	if _, err := c.Templates().Create(t.Context(), ""); !IsInvalidArgument(err) {
		t.Errorf("an empty Template name: %v", err)
	}
	if err := c.Templates().Delete(t.Context(), "ci-runner.my-project"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestTemplateBuildAndWait(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	tpl, err := c.Templates().Build(t.Context(), "ci-runner.my-project", "v1", "latest")
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if tpl.GetStatus().GetBuildInfo().GetCurrentRunningBuildID() != "build1" {
		t.Errorf("build info = %v", tpl.GetStatus().GetBuildInfo())
	}

	fake.mu.Lock()
	req := fake.lastBuild
	fake.mu.Unlock()
	if len(req.GetTags()) != 2 || req.GetTemplateRef().GetName() != "ci-runner.my-project" {
		t.Errorf("build request = %v", req)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		fake.finishBuild(cordiumv1.Template_Status_BuildInfo_Build_STATE_READY, nil)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	build, err := c.Templates().WaitForBuild(ctx, "ci-runner.my-project")
	if err != nil {
		t.Fatalf("WaitForBuild: %v", err)
	}
	if build.GetState() != cordiumv1.Template_Status_BuildInfo_Build_STATE_READY {
		t.Errorf("build state = %s", build.GetState())
	}

	if _, err := c.Templates().CancelBuild(t.Context(), "ci-runner.my-project"); err != nil {
		t.Fatalf("CancelBuild: %v", err)
	}
}

func TestTemplateWaitForBuildReportsFailure(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.Templates().Build(t.Context(), "ci-runner.my-project"); err != nil {
		t.Fatalf("Build: %v", err)
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		fake.finishBuild(cordiumv1.Template_Status_BuildInfo_Build_STATE_FAILED,
			&cordiumv1.Workspace_Status_Failure{
				Message: "the build ran out of time",
				Type: &cordiumv1.Workspace_Status_Failure_BuildTimeoutExceeded_{
					BuildTimeoutExceeded: &cordiumv1.Workspace_Status_Failure_BuildTimeoutExceeded{},
				},
			})
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()

	_, err := c.Templates().WaitForBuild(ctx, "ci-runner.my-project")

	var failure *WorkspaceFailureError
	if !errors.As(err, &failure) {
		t.Fatalf("err = %v, want a *WorkspaceFailureError", err)
	}
	if FailureReason(failure.Failure) != "BuildTimeoutExceeded" {
		t.Errorf("reason = %q", FailureReason(failure.Failure))
	}
}

func TestSecrets(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.Secrets().CreateString(t.Context(), "db-password", "hunter2"); err != nil {
		t.Fatalf("CreateString: %v", err)
	}
	fake.mu.Lock()
	sent := fake.lastSecret
	fake.mu.Unlock()
	if sent.GetData().GetValue() != "hunter2" || sent.GetMetadata().GetName() != "db-password" {
		t.Errorf("Secret = %v", sent)
	}

	if _, err := c.Secrets().CreateBytes(t.Context(), "cert", []byte{1, 2, 3}); err != nil {
		t.Fatalf("CreateBytes: %v", err)
	}
	fake.mu.Lock()
	sent = fake.lastSecret
	fake.mu.Unlock()
	if len(sent.GetData().GetValueBytes()) != 3 {
		t.Errorf("Secret bytes = %v", sent.GetData())
	}

	if _, err := c.Secrets().CreateAttrs(t.Context(), "creds", map[string]any{
		"username": "app",
		"password": "s3cret",
	}); err != nil {
		t.Fatalf("CreateAttrs: %v", err)
	}
	fake.mu.Lock()
	sent = fake.lastSecret
	fake.mu.Unlock()
	if sent.GetData().GetAttrs().GetFields()["username"].GetStringValue() != "app" {
		t.Errorf("Secret attrs = %v", sent.GetData().GetAttrs())
	}

	if _, err := c.Secrets().CreateAttrs(t.Context(), "bad", map[string]any{
		"chan": make(chan int),
	}); !IsInvalidArgument(err) {
		t.Errorf("unencodable attributes: %v", err)
	}

	if _, err := c.Secrets().Get(t.Context(), "db-password"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if err := c.Secrets().Delete(t.Context(), "db-password"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := c.Secrets().List(t.Context(), InSpace("my-project")); err != nil {
		t.Fatalf("List: %v", err)
	}
	fake.mu.Lock()
	listReq := fake.lastSecretList
	fake.mu.Unlock()
	if listReq.GetSpaceRef().GetName() != "my-project" {
		t.Errorf("list spaceRef = %v", listReq.GetSpaceRef())
	}

	for _, err := range c.Secrets().All(t.Context(), InSpace("my-project")) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
	}
}

func TestUserSecrets(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.UserSecrets().CreateString(t.Context(), "gh-token", "ghp_x"); err != nil {
		t.Fatalf("CreateString: %v", err)
	}

	key, err := c.UserSecrets().CreateSSHKey(t.Context(), "personal-key")
	if err != nil {
		t.Fatalf("CreateSSHKey: %v", err)
	}
	if key.GetStatus().GetSshKey().GetPublicKey() == "" {
		t.Error("the generated key carries no public key")
	}

	fake.mu.Lock()
	sent := fake.lastUserSecret
	fake.mu.Unlock()
	if sent.GetSpec().GetType() != cordiumv1.UserSecret_Spec_SSH_KEY || sent.GetData() != nil {
		t.Errorf("SSH key UserSecret = %v", sent)
	}

	if _, err := c.UserSecrets().UpdateString(t.Context(), "gh-token", "ghp_y"); err != nil {
		t.Fatalf("UpdateString: %v", err)
	}
	fake.mu.Lock()
	sent = fake.lastUserSecret
	fake.mu.Unlock()
	if sent.GetData().GetValue() != "ghp_y" {
		t.Errorf("updated data = %v", sent.GetData())
	}

	if _, err := c.UserSecrets().CreateBytes(t.Context(), "blob", []byte("x")); err != nil {
		t.Fatalf("CreateBytes: %v", err)
	}
	if _, err := c.UserSecrets().CreateAttrs(t.Context(), "attrs", map[string]any{"a": 1}); err != nil {
		t.Fatalf("CreateAttrs: %v", err)
	}
	if _, err := c.UserSecrets().List(t.Context()); err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := c.UserSecrets().Delete(t.Context(), "gh-token"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestGitProviders(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.GitProviders().CreateGitHub(t.Context(), "github.my-project",
		"client-id", "gh-secret", "repo"); err != nil {
		t.Fatalf("CreateGitHub: %v", err)
	}
	fake.mu.Lock()
	sent := fake.lastGitProvider
	fake.mu.Unlock()
	github := sent.GetSpec().GetGithub()
	if github.GetClientID() != "client-id" ||
		github.GetClientSecret().GetFromSecret() != "gh-secret" ||
		len(github.GetScopes()) != 1 {
		t.Errorf("GitHub provider = %v", github)
	}

	if _, err := c.GitProviders().CreateGitLab(t.Context(), "gitlab.my-project",
		"gl-id", "gl-secret", "read_repository"); err != nil {
		t.Fatalf("CreateGitLab: %v", err)
	}
	fake.mu.Lock()
	sent = fake.lastGitProvider
	fake.mu.Unlock()
	if sent.GetSpec().GetGitlab().GetClientID() != "gl-id" {
		t.Errorf("GitLab provider = %v", sent.GetSpec().GetGitlab())
	}

	if _, err := c.GitProviders().CreateOAuth2(t.Context(), "self-hosted.my-project", OAuth2Provider{
		ClientID:         "id",
		ClientSecretName: "secret",
		AuthURL:          "https://git.example.com/oauth/authorize",
		TokenURL:         "https://git.example.com/oauth/token",
		Scopes:           []string{"api"},
	}); err != nil {
		t.Fatalf("CreateOAuth2: %v", err)
	}
	fake.mu.Lock()
	sent = fake.lastGitProvider
	fake.mu.Unlock()
	if sent.GetSpec().GetOauth2().GetTokenURL() == "" {
		t.Errorf("OAuth2 provider = %v", sent.GetSpec().GetOauth2())
	}

	for name, err := range map[string]error{
		"no client ID": func() error {
			_, err := c.GitProviders().CreateGitHub(t.Context(), "x", "", "s")
			return err
		}(),
		"no scopes": func() error {
			_, err := c.GitProviders().CreateOAuth2(t.Context(), "x", OAuth2Provider{
				ClientID: "i", ClientSecretName: "s",
				AuthURL: "https://a", TokenURL: "https://t",
			})
			return err
		}(),
		"no name": func() error {
			_, err := c.GitProviders().CreateGitLab(t.Context(), "", "i", "s")
			return err
		}(),
	} {
		if !IsInvalidArgument(err) {
			t.Errorf("%s: %v, want an invalid argument error", name, err)
		}
	}

	if _, err := c.GitProviders().Get(t.Context(), "github.my-project"); err != nil {
		t.Fatalf("Get: %v", err)
	}
	if _, err := c.GitProviders().List(t.Context(), InSpace("my-project")); err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := c.GitProviders().Delete(t.Context(), "github.my-project"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

func TestMemberships(t *testing.T) {
	fake := newFakeCluster()
	c := startFakeCluster(t, fake)

	if _, err := c.Memberships().Add(t.Context(), "my-project.cordium",
		"jane@example.com", RoleAdmin); err != nil {
		t.Fatalf("Add: %v", err)
	}
	fake.mu.Lock()
	sent := fake.lastMembership
	fake.mu.Unlock()
	if sent.GetEmail() != "jane@example.com" || sent.GetRole() != cordiumv1.CreateMembershipRequest_ADMIN {
		t.Errorf("membership request = %v", sent)
	}

	if _, err := c.Memberships().AddUser(t.Context(), "my-project.cordium", "jane", RoleOwner); err != nil {
		t.Fatalf("AddUser: %v", err)
	}
	fake.mu.Lock()
	sent = fake.lastMembership
	fake.mu.Unlock()
	if sent.GetUserRef().GetName() != "jane" || sent.GetRole() != cordiumv1.CreateMembershipRequest_OWNER {
		t.Errorf("membership request = %v", sent)
	}

	updated, err := c.Memberships().SetRole(t.Context(), "member1", RoleUser)
	if err != nil {
		t.Fatalf("SetRole: %v", err)
	}
	if updated.GetSpec().GetRole() != cordiumv1.Membership_Spec_USER {
		t.Errorf("role = %s", updated.GetSpec().GetRole())
	}

	mine, err := c.Memberships().Mine(t.Context(), "my-project.cordium")
	if err != nil {
		t.Fatalf("Mine: %v", err)
	}
	if mine.GetSpec().GetRole() != cordiumv1.Membership_Spec_ADMIN {
		t.Errorf("own role = %s", mine.GetSpec().GetRole())
	}

	if _, err := c.Memberships().Add(t.Context(), "spc", "a@b.c", Role(99)); !IsInvalidArgument(err) {
		t.Errorf("an unknown Role: %v", err)
	}
	if _, err := c.Memberships().List(t.Context(), InSpace("my-project.cordium")); err != nil {
		t.Fatalf("List: %v", err)
	}
	if err := c.Memberships().Remove(t.Context(), "member1"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	for role, want := range map[Role]string{
		RoleUser: "USER", RoleAdmin: "ADMIN", RoleOwner: "OWNER", Role(0): "UNKNOWN",
	} {
		if got := role.String(); got != want {
			t.Errorf("Role(%d).String() = %q, want %q", role, got, want)
		}
	}
}

func TestRegionsAndUserConfig(t *testing.T) {
	c := startFakeCluster(t, newFakeCluster())

	regions, err := c.Regions().List(t.Context())
	if err != nil {
		t.Fatalf("Regions: %v", err)
	}
	if len(regions) != 1 || regions[0].GetStatus().GetCity() != "Frankfurt" {
		t.Errorf("regions = %v", regions)
	}

	cfg, err := c.UserConfig().SetPreferredRegion(t.Context(), "eu-west")
	if err != nil {
		t.Fatalf("SetPreferredRegion: %v", err)
	}
	if cfg.GetSpec().GetPreferredRegion() != "eu-west" {
		t.Errorf("preferredRegion = %q", cfg.GetSpec().GetPreferredRegion())
	}

	cfg, err = c.UserConfig().SetDotfiles(t.Context(), "https://github.com/jdoe/dotfiles", "main")
	if err != nil {
		t.Fatalf("SetDotfiles: %v", err)
	}
	if cfg.GetSpec().GetDotfiles().GetUrl() != "https://github.com/jdoe/dotfiles" ||
		cfg.GetSpec().GetDotfiles().GetBranch() != "main" {
		t.Errorf("dotfiles = %v", cfg.GetSpec().GetDotfiles())
	}
	// Modify is read-modify-write, so the earlier preferred region survives.
	if cfg.GetSpec().GetPreferredRegion() != "eu-west" {
		t.Error("SetDotfiles discarded the preferred Region")
	}

	if _, err := c.UserConfig().SetDotfiles(t.Context(), "", ""); !IsInvalidArgument(err) {
		t.Errorf("an empty dotfiles URL: %v", err)
	}
	if _, err := c.UserConfig().Modify(t.Context(), nil); !IsInvalidArgument(err) {
		t.Errorf("a nil modify function: %v", err)
	}
	if _, err := c.UserConfig().Update(t.Context(), nil); !IsInvalidArgument(err) {
		t.Errorf("a nil UserConfig: %v", err)
	}

	wantErr := errors.New("boom")
	if _, err := c.UserConfig().Modify(t.Context(),
		func(*cordiumv1.UserConfig_Spec) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Errorf("Modify swallowed the callback error: %v", err)
	}
}

func TestErrorClassifiers(t *testing.T) {
	for _, tc := range []struct {
		code  codes.Code
		check func(error) bool
		name  string
	}{
		{codes.NotFound, IsNotFound, "IsNotFound"},
		{codes.AlreadyExists, IsAlreadyExists, "IsAlreadyExists"},
		{codes.InvalidArgument, IsInvalidArgument, "IsInvalidArgument"},
		{codes.PermissionDenied, IsPermissionDenied, "IsPermissionDenied"},
		{codes.Unauthenticated, IsUnauthenticated, "IsUnauthenticated"},
		{codes.ResourceExhausted, IsResourceExhausted, "IsResourceExhausted"},
		{codes.Unavailable, IsUnavailable, "IsUnavailable"},
		{codes.Canceled, IsCanceled, "IsCanceled"},
		{codes.DeadlineExceeded, IsDeadlineExceeded, "IsDeadlineExceeded"},
	} {
		err := status.Error(tc.code, "boom")
		if !tc.check(err) {
			t.Errorf("%s did not match %s", tc.name, tc.code)
		}
		if Code(err) != tc.code {
			t.Errorf("Code() = %s, want %s", Code(err), tc.code)
		}

		// Wrapped Cluster errors must classify the same way.
		if !tc.check(errors.Join(errors.New("context"), err)) {
			t.Errorf("%s did not match a wrapped %s", tc.name, tc.code)
		}
	}

	if Code(nil) != codes.OK {
		t.Errorf("Code(nil) = %s, want OK", Code(nil))
	}
	if IsNotFound(nil) || IsNotFound(errors.New("plain")) {
		t.Error("IsNotFound matched a non-Cluster error")
	}
	if !IsCanceled(context.Canceled) || !IsDeadlineExceeded(context.DeadlineExceeded) {
		t.Error("the context errors are not classified")
	}
}

func TestErrorMessages(t *testing.T) {
	exitErr := &ExitError{Command: "make test", ExitCode: 2, Stderr: []byte("failed")}
	if got := exitErr.Error(); got == "" ||
		!strings.Contains(got, "make test") || !strings.Contains(got, "failed") {
		t.Errorf("ExitError.Error() = %q", got)
	}

	failure := &WorkspaceFailureError{
		Workspace: "abc",
		Failure: &cordiumv1.Workspace_Status_Failure{
			Message: "the task exited with 1",
			Type: &cordiumv1.Workspace_Status_Failure_Task_{
				Task: &cordiumv1.Workspace_Status_Failure_Task{Name: "deps", ExitCode: 1},
			},
		},
	}
	if got := failure.Error(); !strings.Contains(got, "abc") || !strings.Contains(got, "Task") {
		t.Errorf("WorkspaceFailureError.Error() = %q", got)
	}

	var nilExit *ExitError
	var nilFailure *WorkspaceFailureError
	if nilExit.Error() == "" || nilFailure.Error() == "" {
		t.Error("the nil error types do not render")
	}
	if FailureReason(nil) != "" {
		t.Error("FailureReason(nil) is not empty")
	}
}

func TestStreamAndStageStrings(t *testing.T) {
	if StreamStdout.String() != "stdout" || StreamStderr.String() != "stderr" ||
		Stream(0).String() != "unknown" {
		t.Error("the Stream names are wrong")
	}
	for stage, want := range map[LogStage]string{
		LogStageCloningRepo:   "CLONING_REPO",
		LogStagePullingImage:  "PULLING_IMAGE",
		LogStageBuildingImage: "BUILDING_IMAGE",
		LogStageTask:          "TASK",
		LogStageUnknown:       "UNKNOWN",
	} {
		if got := stage.String(); got != want {
			t.Errorf("LogStage(%d) = %q, want %q", stage, got, want)
		}
	}
	for ev, want := range map[EventType]string{
		EventCreated: "CREATED", EventUpdated: "UPDATED",
		EventDeleted: "DELETED", EventType(0): "UNKNOWN",
	} {
		if got := ev.String(); got != want {
			t.Errorf("EventType(%d) = %q, want %q", ev, got, want)
		}
	}
}
