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
	"strconv"
	"sync"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"google.golang.org/protobuf/proto"
)

// State is the state of a Workspace's lifecycle. It is an alias of the
// generated enum, so the generated constants and a State are interchangeable.
type State = cordiumv1.Workspace_Status_State

// The states of a Workspace's lifecycle, from a start request to a full stop.
const (
	StateUnknown         = cordiumv1.Workspace_Status_UNKNOWN
	StateInitRequest     = cordiumv1.Workspace_Status_INIT_REQUEST
	StateInitializing    = cordiumv1.Workspace_Status_INITIALIZING
	StatePullingImage    = cordiumv1.Workspace_Status_PULLING_IMAGE
	StateBuildingImage   = cordiumv1.Workspace_Status_BUILDING_IMAGE
	StateStartingRuntime = cordiumv1.Workspace_Status_STARTING_RUNTIME
	StatePreparing       = cordiumv1.Workspace_Status_PREPARING
	StateRunning         = cordiumv1.Workspace_Status_RUNNING
	StateStoppingRequest = cordiumv1.Workspace_Status_STOPPING_REQUEST
	StateStopping        = cordiumv1.Workspace_Status_STOPPING
	StateStopped         = cordiumv1.Workspace_Status_STOPPED
)

// SharingMode is the audience with which an Application of a Workspace is
// shared.
type SharingMode uint8

const (
	// ShareWithMembers shares the Application with the Members of the
	// Workspace's Space.
	ShareWithMembers SharingMode = iota + 1
	// ShareWithAll shares the Application with all the Cluster's Users.
	ShareWithAll
)

// Workspace is a handle on a Cordium Workspace, which is the platform's
// sandbox: an isolated, rootless, container-based environment.
//
// A handle caches the last state of the Workspace that the Cluster reported.
// The lifecycle methods ([Workspace.Start], [Workspace.Stop], the wait helpers
// and [Workspace.Refresh]) keep that cache up to date, and the accessors read
// it without any network round trip. A handle is safe for concurrent use.
type Workspace struct {
	c *Client

	mu sync.RWMutex
	ws *cordiumv1.Workspace
}

func newWorkspace(c *Client, ws *cordiumv1.Workspace) *Workspace {
	return &Workspace{c: c, ws: ws}
}

func (w *Workspace) snapshot() *cordiumv1.Workspace {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.ws
}

func (w *Workspace) set(ws *cordiumv1.Workspace) {
	if ws == nil {
		return
	}
	w.mu.Lock()
	w.ws = ws
	w.mu.Unlock()
}

// Proto returns a deep copy of the Workspace resource as the Cluster last
// reported it. It is the bridge to the generated API for everything the handle
// does not expose on its own.
func (w *Workspace) Proto() *cordiumv1.Workspace {
	return proto.Clone(w.snapshot()).(*cordiumv1.Workspace)
}

// Name returns the Workspace's Cluster-assigned name (e.g. "abc"). It is what
// every API method and the cordium CLI identify the Workspace by.
func (w *Workspace) Name() string { return w.snapshot().GetMetadata().GetName() }

// UID returns the Workspace's Cluster-wide unique identifier.
func (w *Workspace) UID() string { return w.snapshot().GetMetadata().GetUid() }

// DisplayName returns the human readable name of the Workspace, if any.
func (w *Workspace) DisplayName() string { return w.snapshot().GetMetadata().GetDisplayName() }

// CreatedAt returns the creation time of the Workspace.
func (w *Workspace) CreatedAt() time.Time {
	return w.snapshot().GetMetadata().GetCreatedAt().AsTime()
}

// State returns the last state that the Cluster reported.
func (w *Workspace) State() State { return w.snapshot().GetStatus().GetState() }

// IsRunning reports whether the Workspace is fully initialized and ready to be
// used.
func (w *Workspace) IsRunning() bool { return w.State() == StateRunning }

// IsReady reports whether the Workspace accepts command execution and
// terminals, which is the case from the PREPARING state onwards.
func (w *Workspace) IsReady() bool {
	switch w.State() {
	case StatePreparing, StateRunning:
		return true
	default:
		return false
	}
}

// IsStarting reports whether a run of the Workspace is under way but has not
// reached the PREPARING state yet.
func (w *Workspace) IsStarting() bool {
	switch w.State() {
	case StateInitRequest, StateInitializing, StatePullingImage,
		StateBuildingImage, StateStartingRuntime:
		return true
	default:
		return false
	}
}

// IsStopping reports whether the Workspace is shutting down.
func (w *Workspace) IsStopping() bool {
	switch w.State() {
	case StateStoppingRequest, StateStopping:
		return true
	default:
		return false
	}
}

// IsStopped reports whether the Workspace is not running.
func (w *Workspace) IsStopped() bool { return w.State() == StateStopped }

// Hostname returns the publicly resolvable hostname of the running Workspace
// (e.g. "abc.cordium.example.com"). It is empty while the Workspace is stopped.
func (w *Workspace) Hostname() string { return w.snapshot().GetStatus().GetHostname() }

// URL returns the HTTPS URL of the Workspace's default Application, which is
// the one served at the Workspace's root hostname. It is empty while the
// Workspace is stopped.
func (w *Workspace) URL() string {
	hostname := w.Hostname()
	if hostname == "" {
		return ""
	}
	return "https://" + hostname
}

// AppURL returns the HTTPS URL at which the Cordium portal serves a named
// Application of the Workspace. It is empty while the Workspace is stopped.
//
// Reaching the URL still requires the caller to be authorized: it is either the
// Workspace's owner, or a User the Application was shared with through
// [Workspace.SharePort].
func (w *Workspace) AppURL(name string) string {
	hostname := w.Hostname()
	if hostname == "" || name == "" {
		return ""
	}
	if app := w.Application(name); app != nil && app.GetIsDefault() {
		return "https://" + hostname
	}
	return "https://" + name + "_" + hostname
}

// PortURL returns the HTTPS URL at which the Cordium portal serves a port of
// the Workspace. It is empty while the Workspace is stopped.
func (w *Workspace) PortURL(port int) string {
	hostname := w.Hostname()
	if hostname == "" || port < 1 || port > 65535 {
		return ""
	}
	return "https://port_" + strconv.Itoa(port) + "_" + hostname
}

// Applications returns the named ports that the Workspace's spec declares.
func (w *Workspace) Applications() []*cordiumv1.Workspace_Spec_Application {
	return w.snapshot().GetSpec().GetApplications()
}

// Application returns a named port of the Workspace's spec, or nil.
func (w *Workspace) Application(name string) *cordiumv1.Workspace_Spec_Application {
	for _, app := range w.Applications() {
		if app.GetName() == name {
			return app
		}
	}
	return nil
}

// Spec returns a deep copy of the Workspace's spec.
func (w *Workspace) Spec() *cordiumv1.Workspace_Spec {
	return proto.Clone(w.snapshot().GetSpec()).(*cordiumv1.Workspace_Spec)
}

// Limit returns the effective compute limits that the Cluster resolved for the
// Workspace after merging and capping all the configuration levels.
func (w *Workspace) Limit() *cordiumv1.Workspace_Spec_Limit {
	return w.snapshot().GetStatus().GetLimit()
}

// IsEphemeral reports whether the Workspace's storage is discarded on stop.
func (w *Workspace) IsEphemeral() bool { return w.snapshot().GetSpec().GetIsEphemeral() }

// SpaceName returns the name of the Space that the Workspace belongs to.
func (w *Workspace) SpaceName() string {
	return w.snapshot().GetStatus().GetSpaceRef().GetName()
}

// TemplateName returns the name of the Template that the Workspace was created
// from.
func (w *Workspace) TemplateName() string {
	return w.snapshot().GetStatus().GetTemplateRef().GetName()
}

// RegionName returns the name of the Octelium Region that currently hosts the
// Workspace.
func (w *Workspace) RegionName() string {
	return w.snapshot().GetStatus().GetRegionRef().GetName()
}

// Failure returns the failure of the Workspace's current or latest run, if any.
func (w *Workspace) Failure() *cordiumv1.Workspace_Status_Failure {
	return w.snapshot().GetStatus().GetFailure()
}

// Run returns the current or the latest run of the Workspace.
func (w *Workspace) Run() *cordiumv1.Workspace_Status_Run {
	return w.snapshot().GetStatus().GetRun()
}

// SharedPorts returns the Applications of the Workspace that are currently
// shared with other Users.
func (w *Workspace) SharedPorts() []*cordiumv1.Workspace_Status_SharedPort {
	return w.snapshot().GetStatus().GetSharedPorts()
}

func (w *Workspace) ref() *metav1.ObjectReference {
	ws := w.snapshot()
	return &metav1.ObjectReference{
		Uid:  ws.GetMetadata().GetUid(),
		Name: ws.GetMetadata().GetName(),
	}
}

// Refresh reloads the Workspace from the Cluster.
func (w *Workspace) Refresh(ctx context.Context) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}

	ws, err := w.c.MainService().GetWorkspace(ctx, &metav1.GetOptions{Uid: w.UID()})
	if err != nil {
		return err
	}
	w.set(ws)
	return nil
}

// StartOption configures a single run of a Workspace.
type StartOption func(*cordiumv1.StartWorkspaceRequest_Config) error

// WithRunVar overrides one of the Workspace's variables for this run only.
func WithRunVar(name, value string) StartOption {
	return func(cfg *cordiumv1.StartWorkspaceRequest_Config) error {
		if name == "" {
			return invalidArgumentf("empty variable name")
		}
		cfg.Vars = append(cfg.Vars, &cordiumv1.Workspace_Spec_Var{Name: name, Value: value})
		return nil
	}
}

// WithRunVars overrides a set of the Workspace's variables for this run only.
// They are applied in a stable order.
func WithRunVars(vars map[string]string) StartOption {
	return func(cfg *cordiumv1.StartWorkspaceRequest_Config) error {
		for _, name := range sortedKeys(vars) {
			if err := WithRunVar(name, vars[name])(cfg); err != nil {
				return err
			}
		}
		return nil
	}
}

// WithRegion runs the Workspace in a specific Octelium Region. Without it, the
// Cluster picks one on its own, preferring the User's preferred Region.
func WithRegion(name string) StartOption {
	return func(cfg *cordiumv1.StartWorkspaceRequest_Config) error {
		if name == "" {
			return invalidArgumentf("empty Region name")
		}
		cfg.RegionRef = nameRef(name)
		return nil
	}
}

func startConfig(opts ...StartOption) (*cordiumv1.StartWorkspaceRequest_Config, error) {
	var ret *cordiumv1.StartWorkspaceRequest_Config
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if ret == nil {
			ret = &cordiumv1.StartWorkspaceRequest_Config{}
		}
		if err := opt(ret); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// Start requests the start of a stopped Workspace. The Cluster accepts the
// request and initializes the Workspace asynchronously, so Start returns as
// soon as the request is accepted. Use [Workspace.WaitUntilRunning] to wait for
// the run to be ready, or [WorkspaceClient.Run] to do both in one call.
//
// Starting an already running Workspace is a no-op.
func (w *Workspace) Start(ctx context.Context, opts ...StartOption) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}

	cfg, err := startConfig(opts...)
	if err != nil {
		return err
	}

	_, err = w.c.MainService().StartWorkspace(ctx, &cordiumv1.StartWorkspaceRequest{
		WorkspaceRef: w.ref(),
		Config:       cfg,
	})
	if err != nil {
		if IsAlreadyExists(err) {
			return nil
		}
		return err
	}
	return nil
}

// Stop requests a graceful stop of a running Workspace. The Cluster runs the
// PRE_STOP Tasks and shuts the Workspace down asynchronously, so Stop returns
// as soon as the request is accepted. Use [Workspace.WaitUntilStopped] to wait
// for the Workspace to be fully stopped.
func (w *Workspace) Stop(ctx context.Context) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}

	_, err := w.c.MainService().StopWorkspace(ctx, &cordiumv1.StopWorkspaceRequest{
		WorkspaceRef: w.ref(),
	})
	return err
}

// Delete deletes the Workspace and, with it, its storage.
//
// It is what a caller normally defers right after creating a one-shot sandbox.
// Since a canceled context would leave the Workspace behind, the cleanup path
// should use a context that outlives the work, such as
// context.WithoutCancel(ctx).
func (w *Workspace) Delete(ctx context.Context) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}

	_, err := w.c.MainService().DeleteWorkspace(ctx, &metav1.DeleteOptions{Uid: w.UID()})
	return err
}

// Update applies spec options to the Workspace. Only the display name and the
// spec of a Workspace can be updated, and the changes only take effect on its
// next run.
//
// The options start from the Workspace's current spec, so
//
//	err := ws.Update(ctx, cordium.WithEnv("LOG_LEVEL", "debug"))
//
// adds an environment variable and leaves everything else in place.
func (w *Workspace) Update(ctx context.Context, opts ...WorkspaceOption) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}

	current := w.Proto()
	if current.Spec == nil {
		current.Spec = &cordiumv1.Workspace_Spec{}
	}

	builder, err := newSpecBuilder(append([]WorkspaceOption{FromSpec(current.Spec)}, opts...)...)
	if err != nil {
		return err
	}
	if builder.gitProvider != "" {
		return invalidArgumentf("WithGitProvider only applies to Templates")
	}
	if builder.templateRef != nil {
		return invalidArgumentf("the Template of a Workspace cannot be changed")
	}

	current.Spec = builder.spec
	if builder.displayName != "" {
		if current.Metadata == nil {
			current.Metadata = &metav1.Metadata{}
		}
		current.Metadata.DisplayName = builder.displayName
	}

	updated, err := w.c.MainService().UpdateWorkspace(ctx, current)
	if err != nil {
		return err
	}
	w.set(updated)
	return nil
}

// SharePort shares a named Application of the Workspace with other Users so
// that they can reach it at the Application's public URL.
func (w *Workspace) SharePort(ctx context.Context, applicationName string, mode SharingMode) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}
	if applicationName == "" {
		return invalidArgumentf("empty Application name")
	}

	var pbMode cordiumv1.ShareWorkspacePortRequest_Mode
	switch mode {
	case ShareWithMembers:
		pbMode = cordiumv1.ShareWorkspacePortRequest_MEMBERS
	case ShareWithAll:
		pbMode = cordiumv1.ShareWorkspacePortRequest_ALL
	default:
		return invalidArgumentf("unknown sharing mode %d", mode)
	}

	_, err := w.c.MainService().ShareWorkspacePort(ctx, &cordiumv1.ShareWorkspacePortRequest{
		WorkspaceRef:    w.ref(),
		Mode:            pbMode,
		ApplicationName: applicationName,
	})
	return err
}

// UnsharePort stops sharing a previously shared Application of the Workspace.
func (w *Workspace) UnsharePort(ctx context.Context, applicationName string) error {
	if err := w.c.ensureOpen(); err != nil {
		return err
	}
	if applicationName == "" {
		return invalidArgumentf("empty Application name")
	}

	_, err := w.c.MainService().UnshareWorkspacePort(ctx, &cordiumv1.UnshareWorkspacePortRequest{
		WorkspaceRef:    w.ref(),
		ApplicationName: applicationName,
	})
	return err
}
