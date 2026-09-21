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
	"testing"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"google.golang.org/protobuf/encoding/protojson"
)

func buildSpec(t *testing.T, opts ...WorkspaceOption) *specBuilder {
	t.Helper()
	b, err := newSpecBuilder(opts...)
	if err != nil {
		t.Fatalf("newSpecBuilder: %v", err)
	}
	return b
}

func TestSpecImageSources(t *testing.T) {
	b := buildSpec(t, WithImage("ubuntu:24.04"))
	if got := b.spec.GetImage().GetRegistry().GetUrl(); got != "ubuntu:24.04" {
		t.Errorf("registry URL = %q", got)
	}

	b = buildSpec(t, WithPrivateImage("reg.example.com/dev:1", "robot", "reg-password"))
	auth := b.spec.GetImage().GetRegistry().GetAuthentication()
	if auth.GetUsername() != "robot" || auth.GetPassword().GetFromSecret() != "reg-password" {
		t.Errorf("registry authentication = %v", auth)
	}

	b = buildSpec(t, WithDockerfile("FROM ubuntu:24.04\n"))
	if got := b.spec.GetImage().GetDockerfile().GetInline(); got != "FROM ubuntu:24.04\n" {
		t.Errorf("inline Dockerfile = %q", got)
	}

	b = buildSpec(t, WithDockerfileURL("https://example.com/Dockerfile"))
	if got := b.spec.GetImage().GetDockerfile().GetUrl(); got != "https://example.com/Dockerfile" {
		t.Errorf("Dockerfile URL = %q", got)
	}

	b = buildSpec(t, WithImageFromGit("https://github.com/org/images",
		ImageGitCheckout("v1"), ImageGitDockerfile("go/Dockerfile"), ImageGitContext("go")))
	git := b.spec.GetImage().GetGit()
	if git.GetUrl() != "https://github.com/org/images" || git.GetCheckout() != "v1" ||
		git.GetDockerfile() != "go/Dockerfile" || git.GetContext() != "go" {
		t.Errorf("image git = %v", git)
	}

	b = buildSpec(t, WithImageFromRepoDockerfile("docker/Dockerfile.dev", "."))
	repoDockerfile := b.spec.GetImage().GetRepository().GetDockerfile()
	if repoDockerfile.GetPath() != "docker/Dockerfile.dev" || repoDockerfile.GetContext() != "." {
		t.Errorf("repository Dockerfile = %v", repoDockerfile)
	}

	b = buildSpec(t, WithImageFromRepoDevcontainer(".devcontainer"))
	if got := b.spec.GetImage().GetRepository().GetDevcontainer().GetDirPath(); got != ".devcontainer" {
		t.Errorf("devcontainer dirPath = %q", got)
	}

	// The image is a oneof: the last option wins rather than merging.
	b = buildSpec(t, WithDockerfile("FROM scratch"), WithImage("alpine:3"))
	if b.spec.GetImage().GetDockerfile() != nil {
		t.Error("the Dockerfile survived a later WithImage")
	}
	if got := b.spec.GetImage().GetRegistry().GetUrl(); got != "alpine:3" {
		t.Errorf("registry URL = %q", got)
	}
}

func TestSpecRepository(t *testing.T) {
	b := buildSpec(t, WithRepo("https://github.com/org/app",
		WithBranch("develop"),
		WithDepth(1),
		WithCheckout("v2.3.0"),
		WithSingleBranch(),
		WithShallowSubmodules(),
		WithoutLazyUnshallow(),
		WithRepoAuth("oauth2", "git-token")))

	repo := b.spec.GetRepository()
	clone := repo.GetCloneOptions()
	switch {
	case repo.GetUrl() != "https://github.com/org/app":
		t.Errorf("URL = %q", repo.GetUrl())
	case clone.GetBranch() != "develop":
		t.Errorf("branch = %q", clone.GetBranch())
	case clone.GetDepth() != 1:
		t.Errorf("depth = %d", clone.GetDepth())
	case clone.GetCheckout() != "v2.3.0":
		t.Errorf("checkout = %q", clone.GetCheckout())
	case !clone.GetSingleBranch() || !clone.GetShallowSubmodules() || !clone.GetDisableLazyUnshallow():
		t.Errorf("clone flags = %v", clone)
	case repo.GetAuthentication().GetHttp().GetPassword().GetFromSecret() != "git-token":
		t.Errorf("authentication = %v", repo.GetAuthentication())
	}

	b = buildSpec(t,
		WithAdditionalRepo("shared", "/workspace/shared", "https://github.com/org/shared",
			WithBranch("main")))
	additional := b.spec.GetAdditionalRepositories()
	if len(additional) != 1 {
		t.Fatalf("additionalRepositories = %v", additional)
	}
	if additional[0].GetName() != "shared" || additional[0].GetClonePath() != "/workspace/shared" ||
		additional[0].GetRepository().GetCloneOptions().GetBranch() != "main" {
		t.Errorf("additional repository = %v", additional[0])
	}
}

func TestSpecRuntime(t *testing.T) {
	b := buildSpec(t,
		WithEnv("A", "1"),
		WithEnvs(map[string]string{"C": "3", "B": "2"}),
		WithEnvFromSecret("TOKEN", "api-token"),
		WithTask("deps", "npm ci", TaskWorkingDir("/workspace/repo"), TaskEnv("CI", "true"), TaskAbortOnFailure()),
		WithPostStartTask("dev", "npm run dev", TaskInBackground()),
		WithPreStopTask("cleanup", "./cleanup.sh", TaskAsRoot(), TaskContinueOnFailure()),
		WithCmd("/bin/bash"),
		WithEntrypoint("/tini"),
		WithoutInit(),
		WithReadOnlyRootFilesystem(),
		WithAddedCapabilities("NET_ADMIN"),
		WithDroppedCapabilities("NET_RAW"),
		WithoutTimeout(),
		WithAutoStop(),
		WithDevcontainerFeature("ghcr.io/devcontainers/features/docker-in-docker:2",
			map[string]string{"version": "latest"}),
		WithOcteliumServices("postgres", "redis"),
		WithAllOcteliumServices(),
	)

	rt := b.spec.GetRuntime()

	envKeys := make([]string, 0, len(rt.GetEnvVars()))
	for _, env := range rt.GetEnvVars() {
		envKeys = append(envKeys, env.GetKey())
	}
	want := []string{"A", "B", "C", "TOKEN"}
	for i, key := range want {
		if envKeys[i] != key {
			t.Errorf("envVars = %v, want a stable %v", envKeys, want)
			break
		}
	}
	if rt.GetEnvVars()[3].GetFromSecret() != "api-token" {
		t.Errorf("secret-sourced env var = %v", rt.GetEnvVars()[3])
	}

	if len(rt.GetTasks()) != 3 {
		t.Fatalf("tasks = %v", rt.GetTasks())
	}
	onCreate := rt.GetTasks()[0]
	switch {
	case onCreate.GetType() != cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE:
		t.Errorf("task type = %s", onCreate.GetType())
	case onCreate.GetWorkingDir() != "/workspace/repo":
		t.Errorf("task workingDir = %q", onCreate.GetWorkingDir())
	case onCreate.GetOnFailure() != cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_ABORT:
		t.Errorf("task onFailure = %s", onCreate.GetOnFailure())
	case len(onCreate.GetEnvVars()) != 1 || onCreate.GetEnvVars()[0].GetKey() != "CI":
		t.Errorf("task envVars = %v", onCreate.GetEnvVars())
	}
	if rt.GetTasks()[1].GetType() != cordiumv1.Workspace_Spec_Runtime_Task_POST_START ||
		!rt.GetTasks()[1].GetIsBackground() {
		t.Errorf("post start task = %v", rt.GetTasks()[1])
	}
	if rt.GetTasks()[2].GetType() != cordiumv1.Workspace_Spec_Runtime_Task_PRE_STOP ||
		!rt.GetTasks()[2].GetRunAsRoot() ||
		rt.GetTasks()[2].GetOnFailure() != cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_CONTINUE {
		t.Errorf("pre stop task = %v", rt.GetTasks()[2])
	}

	switch {
	case rt.GetCmd() != "/bin/bash" || rt.GetEntrypoint() != "/tini" || !rt.GetDisableInit():
		t.Errorf("container overrides = %v", rt)
	case !rt.GetFilesystem().GetReadOnly():
		t.Error("readOnly filesystem was not set")
	case len(rt.GetCapabilities().GetAdd()) != 1 || len(rt.GetCapabilities().GetDrop()) != 1:
		t.Errorf("capabilities = %v", rt.GetCapabilities())
	case rt.GetTimeout().GetMode() != cordiumv1.Workspace_Spec_Runtime_Timeout_DISABLED:
		t.Errorf("timeout mode = %s", rt.GetTimeout().GetMode())
	case !rt.GetAutoStop():
		t.Error("autoStop was not set")
	}

	features := rt.GetDevcontainers().GetFeatures()
	if len(features) != 1 || features[0].GetReference() == "" ||
		len(features[0].GetOptions()) != 1 || features[0].GetOptions()[0].GetKey() != "version" {
		t.Errorf("devcontainer features = %v", features)
	}

	oct := rt.GetOctelium()
	if len(oct.GetServeServices()) != 2 || !oct.GetServeAll() {
		t.Errorf("octelium runtime = %v", oct)
	}
}

func TestSpecNetworkEgress(t *testing.T) {
	b := buildSpec(t,
		WithEgressDefaultDeny(),
		WithEgressAllow([]string{"10.0.0.0/8"}, 443, 5432),
		WithEgressDeny([]string{"169.254.169.254/32"}))

	egress := b.spec.GetRuntime().GetNetwork().GetEgress()
	if egress.GetDefaultAction() != cordiumv1.Workspace_Spec_Runtime_Network_Egress_DENY {
		t.Errorf("default action = %s", egress.GetDefaultAction())
	}
	if len(egress.GetRules()) != 2 {
		t.Fatalf("rules = %v", egress.GetRules())
	}
	if egress.GetRules()[0].GetAction() != cordiumv1.Workspace_Spec_Runtime_Network_Rule_ALLOW ||
		len(egress.GetRules()[0].GetPorts()) != 2 {
		t.Errorf("allow rule = %v", egress.GetRules()[0])
	}
	if egress.GetRules()[1].GetAction() != cordiumv1.Workspace_Spec_Runtime_Network_Rule_DENY ||
		len(egress.GetRules()[1].GetPorts()) != 0 {
		t.Errorf("deny rule = %v", egress.GetRules()[1])
	}

	if _, err := newSpecBuilder(WithEgressAllow(nil)); !IsInvalidArgument(err) {
		t.Errorf("an egress rule with no CIDR: %v", err)
	}
}

func TestSpecApplicationsAndLimits(t *testing.T) {
	b := buildSpec(t,
		WithApp("web", 3000, AsDefaultApp(), AppDisplayName("Web UI")),
		WithApp("api", 8080),
		WithPort(6006),
		WithResources(Resources{CPUMillicores: 2000, MemoryMB: 4096, StorageMB: 20000}))

	apps := b.spec.GetApplications()
	if len(apps) != 3 {
		t.Fatalf("applications = %v", apps)
	}
	if !apps[0].GetIsDefault() || apps[0].GetDisplayName() != "Web UI" || apps[0].GetPort() != 3000 {
		t.Errorf("default app = %v", apps[0])
	}
	if apps[2].GetName() != "port-6006" || apps[2].GetPort() != 6006 {
		t.Errorf("port app = %v", apps[2])
	}

	limit := b.spec.GetLimit()
	if limit.GetCpu().GetMillicores() != 2000 || limit.GetMemory().GetMegabytes() != 4096 ||
		limit.GetStorage().GetMegabytes() != 20000 {
		t.Errorf("limit = %v", limit)
	}

	for _, port := range []int{0, 70000} {
		if _, err := newSpecBuilder(WithApp("bad", port)); !IsInvalidArgument(err) {
			t.Errorf("port %d was accepted", port)
		}
	}
}

func TestSpecVarsTemplateAndSpace(t *testing.T) {
	b := buildSpec(t,
		WithVar("BRANCH", "main"),
		WithVars(map[string]string{"B": "2", "A": "1"}),
		WithTemplate("ci-runner.my-project"))

	vars := b.spec.GetVars()
	if len(vars) != 3 || vars[0].GetName() != "BRANCH" || vars[1].GetName() != "A" || vars[2].GetName() != "B" {
		t.Errorf("vars = %v", vars)
	}
	if b.templateRef.GetName() != "ci-runner.my-project" {
		t.Errorf("templateRef = %v", b.templateRef)
	}

	b = buildSpec(t, WithSpace("my-project"))
	if got := b.templateRef.GetName(); got != "default.my-project" {
		t.Errorf("WithSpace produced the Template %q", got)
	}
}

func TestSpecVolumeMounts(t *testing.T) {
	b := buildSpec(t,
		WithVolume("datasets", "/data"),
		WithReadOnlyVolume("models", "/opt/models"))

	mounts := b.spec.GetRuntime().GetVolumeMounts()
	if len(mounts) != 2 {
		t.Fatalf("volumeMounts = %v", mounts)
	}

	if mounts[0].GetVolumeRef().GetName() != "datasets" ||
		mounts[0].GetMountPath() != "/data" || mounts[0].GetReadOnly() {
		t.Errorf("mount = %v", mounts[0])
	}

	if mounts[1].GetVolumeRef().GetName() != "models" ||
		mounts[1].GetMountPath() != "/opt/models" || !mounts[1].GetReadOnly() {
		t.Errorf("mount = %v", mounts[1])
	}

	for name, opt := range map[string]WorkspaceOption{
		"emptyName":      WithVolume("", "/data"),
		"emptyPath":      WithVolume("datasets", ""),
		"relativePath":   WithVolume("datasets", "data"),
		"emptyRONname":   WithReadOnlyVolume("", "/data"),
		"relativeROPath": WithReadOnlyVolume("datasets", "data"),
	} {
		if _, err := newSpecBuilder(opt); !IsInvalidArgument(err) {
			t.Errorf("%s was accepted: %v", name, err)
		}
	}
}

func TestTemplateSpecKeepsVolumeMounts(t *testing.T) {
	b := buildSpec(t, WithVolume("datasets", "/data"))

	spec, err := templateSpecFrom(b)
	if err != nil {
		t.Fatalf("templateSpecFrom: %v", err)
	}

	mounts := spec.GetRuntime().GetVolumeMounts()
	if len(mounts) != 1 || mounts[0].GetVolumeRef().GetName() != "datasets" {
		t.Errorf("a Template lost its volumeMounts: %v", mounts)
	}
}

func TestSpecEscapeHatches(t *testing.T) {
	base := &cordiumv1.Workspace_Spec{
		Image: &cordiumv1.Workspace_Spec_Image{
			Type: &cordiumv1.Workspace_Spec_Image_Registry_{
				Registry: &cordiumv1.Workspace_Spec_Image_Registry{Url: "base:1"},
			},
		},
	}

	b := buildSpec(t, FromSpec(base), WithEnv("EXTRA", "yes"))
	if b.spec.GetImage().GetRegistry().GetUrl() != "base:1" {
		t.Error("FromSpec lost the image")
	}
	if base.GetRuntime() != nil {
		t.Error("FromSpec mutated the caller's spec instead of cloning it")
	}

	raw, err := protojson.Marshal(base)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	b = buildSpec(t, FromSpecJSON(raw))
	if b.spec.GetImage().GetRegistry().GetUrl() != "base:1" {
		t.Error("FromSpecJSON did not decode the spec")
	}

	b = buildSpec(t, WithSpecFunc(func(spec *cordiumv1.Workspace_Spec) error {
		spec.IsEphemeral = true
		return nil
	}))
	if !b.spec.GetIsEphemeral() {
		t.Error("WithSpecFunc did not apply")
	}

	if _, err := newSpecBuilder(FromSpecJSON([]byte("{not json"))); !IsInvalidArgument(err) {
		t.Errorf("invalid JSON: %v", err)
	}
}

func TestSpecRejectsEmptyValues(t *testing.T) {
	for name, opt := range map[string]WorkspaceOption{
		"WithImage":     WithImage(""),
		"WithRepo":      WithRepo(""),
		"WithEnv":       WithEnv("", "v"),
		"WithTask":      WithTask("", "run"),
		"WithTaskRun":   WithTask("name", ""),
		"WithVar":       WithVar("", "v"),
		"WithTemplate":  WithTemplate(""),
		"WithSpace":     WithSpace(""),
		"WithApp":       WithApp("", 80),
		"WithDockerfle": WithDockerfile(""),
	} {
		if _, err := newSpecBuilder(opt); !IsInvalidArgument(err) {
			t.Errorf("%s accepted an empty value: %v", name, err)
		}
	}
}

func TestTemplateSpecRejectsWorkspaceOnlyOptions(t *testing.T) {
	if _, err := templateSpecFrom(buildSpec(t, WithApp("web", 3000))); !IsInvalidArgument(err) {
		t.Errorf("a Template with an Application: %v", err)
	}
	if _, err := templateSpecFrom(buildSpec(t, Ephemeral())); !IsInvalidArgument(err) {
		t.Errorf("an ephemeral Template: %v", err)
	}
	if _, err := templateSpecFrom(buildSpec(t, WithTemplate("other"))); !IsInvalidArgument(err) {
		t.Errorf("a Template created from a Template: %v", err)
	}

	spec, err := templateSpecFrom(buildSpec(t,
		WithImage("golang:1.25"),
		WithRepo("https://github.com/org/app"),
		WithGitProvider("github"),
		WithCPU(4000)))
	if err != nil {
		t.Fatalf("templateSpecFrom: %v", err)
	}
	if spec.GetGitProvider() != "github" {
		t.Errorf("gitProvider = %q", spec.GetGitProvider())
	}
	if spec.GetImage().GetRegistry().GetUrl() != "golang:1.25" ||
		spec.GetRepository().GetUrl() != "https://github.com/org/app" ||
		spec.GetLimit().GetCpu().GetMillicores() != 4000 {
		t.Errorf("the Template spec lost fields: %v", spec)
	}
}

func TestEphemeralAndPersistent(t *testing.T) {
	if b := buildSpec(t, Ephemeral()); !b.spec.GetIsEphemeral() {
		t.Error("Ephemeral did not apply")
	}
	if b := buildSpec(t, Ephemeral(), Persistent()); b.spec.GetIsEphemeral() {
		t.Error("Persistent did not override Ephemeral")
	}
}

func TestShortNameAndOrganizationSpaceName(t *testing.T) {
	for in, want := range map[string]string{
		"ml-env.research.jdoe": "ml-env",
		"abc":                  "abc",
		"":                     "",
	} {
		if got := ShortName(in); got != want {
			t.Errorf("ShortName(%q) = %q, want %q", in, got, want)
		}
	}

	for in, want := range map[string]string{
		"my-project":         "my-project.cordium",
		"my-project.cordium": "my-project.cordium",
		"":                   "",
	} {
		if got := OrganizationSpaceName(in); got != want {
			t.Errorf("OrganizationSpaceName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWorkspaceURLs(t *testing.T) {
	ws := &Workspace{ws: &cordiumv1.Workspace{
		Status: &cordiumv1.Workspace_Status{Hostname: "abc.cordium.example.com"},
		Spec: &cordiumv1.Workspace_Spec{
			Applications: []*cordiumv1.Workspace_Spec_Application{
				{Name: "web", Port: 3000, IsDefault: true},
				{Name: "api", Port: 8080},
			},
		},
	}}

	if got, want := ws.URL(), "https://abc.cordium.example.com"; got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
	if got, want := ws.AppURL("web"), "https://abc.cordium.example.com"; got != want {
		t.Errorf("AppURL(default) = %q, want %q", got, want)
	}
	if got, want := ws.AppURL("api"), "https://api_abc.cordium.example.com"; got != want {
		t.Errorf("AppURL = %q, want %q", got, want)
	}
	if got, want := ws.PortURL(3000), "https://port_3000_abc.cordium.example.com"; got != want {
		t.Errorf("PortURL = %q, want %q", got, want)
	}

	stopped := &Workspace{ws: &cordiumv1.Workspace{Status: &cordiumv1.Workspace_Status{}}}
	if stopped.URL() != "" || stopped.AppURL("api") != "" || stopped.PortURL(80) != "" {
		t.Error("a stopped Workspace reported URLs")
	}
}

func TestWorkspaceStatePredicates(t *testing.T) {
	for state, checks := range map[State][4]bool{
		//                       running ready  starting stopping
		StateInitRequest:     {false, false, true, false},
		StatePullingImage:    {false, false, true, false},
		StateBuildingImage:   {false, false, true, false},
		StateStartingRuntime: {false, false, true, false},
		StatePreparing:       {false, true, false, false},
		StateRunning:         {true, true, false, false},
		StateStoppingRequest: {false, false, false, true},
		StateStopping:        {false, false, false, true},
		StateStopped:         {false, false, false, false},
	} {
		ws := &Workspace{ws: &cordiumv1.Workspace{
			Status: &cordiumv1.Workspace_Status{State: state},
		}}
		got := [4]bool{ws.IsRunning(), ws.IsReady(), ws.IsStarting(), ws.IsStopping()}
		if got != checks {
			t.Errorf("%s: running/ready/starting/stopping = %v, want %v", state, got, checks)
		}
	}
}
