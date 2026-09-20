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
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	octelium "github.com/octelium/octelium/octelium-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeCluster is an in-process Cordium Cluster that implements just enough of
// the API for the SDK's own behavior to be exercised end to end.
type fakeCluster struct {
	cordiumv1.UnimplementedMainServiceServer
	cordiumv1.UnimplementedWorkspaceServiceServer

	mu          sync.Mutex
	workspaces  map[string]*cordiumv1.Workspace
	nextName    int
	watchers    map[chan *cordiumv1.WatchWorkspaceResponse]struct{}
	startCalls  int
	listCalls   int
	lastCreate  *cordiumv1.Workspace
	lastExecReq *cordiumv1.ExecRequest_Request

	terminalCols, terminalRows uint32
	terminalWrites             chan []byte
	removedTerminal            string

	lastSpace        *cordiumv1.Space
	lastSpaceList    *cordiumv1.ListSpaceOptions
	lastTemplate     *cordiumv1.Template
	lastTemplateList *cordiumv1.ListTemplateOptions
	lastBuild        *cordiumv1.BuildTemplateRequest
	templateState    *cordiumv1.Template
	lastSecret       *cordiumv1.Secret
	lastSecretList   *cordiumv1.ListSecretOptions
	lastUserSecret   *cordiumv1.UserSecret
	lastGitProvider  *cordiumv1.GitProvider
	lastMembership   *cordiumv1.CreateMembershipRequest
	userConfig       *cordiumv1.UserConfig

	// execScript decides what an Exec stream produces.
	execScript func(req *cordiumv1.ExecRequest_Request, srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error
}

func newFakeCluster() *fakeCluster {
	return &fakeCluster{
		workspaces: map[string]*cordiumv1.Workspace{},
		watchers:   map[chan *cordiumv1.WatchWorkspaceResponse]struct{}{},
	}
}

func startFakeCluster(t *testing.T, fake *fakeCluster) *Client {
	t.Helper()

	lis := bufconn.Listen(1024 * 1024)
	srv := grpc.NewServer()
	cordiumv1.RegisterMainServiceServer(srv, fake)
	cordiumv1.RegisterWorkspaceServiceServer(srv, fake)

	go func() {
		_ = srv.Serve(lis)
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	c, err := New(ctx,
		WithDomain("example.com"),
		WithAccessToken("test-token"),
		WithAPIEndpoint("passthrough:///bufnet"),
		WithOcteliumOptions(
			octelium.WithGRPCTransportCredentials(insecure.NewCredentials()),
			octelium.WithGRPCDialOptions(grpc.WithContextDialer(
				func(ctx context.Context, _ string) (net.Conn, error) {
					return lis.DialContext(ctx)
				})),
		),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Cleanup(func() {
		_ = c.Close()
		srv.Stop()
		_ = lis.Close()
	})

	return c
}

func (f *fakeCluster) CreateWorkspace(ctx context.Context, req *cordiumv1.Workspace) (*cordiumv1.Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastCreate = proto.Clone(req).(*cordiumv1.Workspace)
	f.nextName++
	name := fmt.Sprintf("ws%d", f.nextName)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name:        name,
			Uid:         "uid-" + name,
			DisplayName: req.GetMetadata().GetDisplayName(),
			CreatedAt:   timestamppb.Now(),
		},
		Spec: req.GetSpec(),
		Status: &cordiumv1.Workspace_Status{
			State:       cordiumv1.Workspace_Status_STOPPED,
			TemplateRef: req.GetStatus().GetTemplateRef(),
		},
	}
	f.workspaces[name] = ws
	return proto.Clone(ws).(*cordiumv1.Workspace), nil
}

func (f *fakeCluster) GetWorkspace(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ws := f.lookupLocked(req.GetName(), req.GetUid())
	if ws == nil {
		return nil, status.Error(codes.NotFound, "no such Workspace")
	}
	return proto.Clone(ws).(*cordiumv1.Workspace), nil
}

func (f *fakeCluster) lookupLocked(name, uid string) *cordiumv1.Workspace {
	for _, ws := range f.workspaces {
		if name != "" && ws.Metadata.Name == name {
			return ws
		}
		if uid != "" && ws.Metadata.Uid == uid {
			return ws
		}
	}
	return nil
}

func (f *fakeCluster) DeleteWorkspace(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ws := f.lookupLocked(req.GetName(), req.GetUid())
	if ws == nil {
		return nil, status.Error(codes.NotFound, "no such Workspace")
	}
	delete(f.workspaces, ws.Metadata.Name)
	f.publishLocked(&cordiumv1.WatchWorkspaceResponse{
		Type: &cordiumv1.WatchWorkspaceResponse_Delete_{
			Delete: &cordiumv1.WatchWorkspaceResponse_Delete{Item: proto.Clone(ws).(*cordiumv1.Workspace)},
		},
	})
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) StartWorkspace(ctx context.Context,
	req *cordiumv1.StartWorkspaceRequest) (*cordiumv1.StartWorkspaceResponse, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.startCalls++
	ws := f.lookupLocked(req.GetWorkspaceRef().GetName(), req.GetWorkspaceRef().GetUid())
	if ws == nil {
		return nil, status.Error(codes.NotFound, "no such Workspace")
	}
	if ws.Status.State != cordiumv1.Workspace_Status_STOPPED {
		return nil, status.Error(codes.AlreadyExists, "the Workspace is already starting or running")
	}
	f.setStateLocked(ws, cordiumv1.Workspace_Status_INIT_REQUEST)
	return &cordiumv1.StartWorkspaceResponse{}, nil
}

func (f *fakeCluster) UpdateWorkspace(ctx context.Context, req *cordiumv1.Workspace) (*cordiumv1.Workspace, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	ws := f.lookupLocked(req.GetMetadata().GetName(), req.GetMetadata().GetUid())
	if ws == nil {
		return nil, status.Error(codes.NotFound, "no such Workspace")
	}
	ws.Spec = req.GetSpec()
	ws.Metadata.DisplayName = req.GetMetadata().GetDisplayName()
	return proto.Clone(ws).(*cordiumv1.Workspace), nil
}

func (f *fakeCluster) StopWorkspace(ctx context.Context,
	req *cordiumv1.StopWorkspaceRequest) (*cordiumv1.StopWorkspaceResponse, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	ws := f.lookupLocked(req.GetWorkspaceRef().GetName(), req.GetWorkspaceRef().GetUid())
	if ws == nil {
		return nil, status.Error(codes.NotFound, "no such Workspace")
	}
	f.setStateLocked(ws, cordiumv1.Workspace_Status_STOPPED)
	return &cordiumv1.StopWorkspaceResponse{}, nil
}

func (f *fakeCluster) ListWorkspace(ctx context.Context,
	req *cordiumv1.ListWorkspaceOptions) (*cordiumv1.WorkspaceList, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.listCalls++

	all := make([]*cordiumv1.Workspace, 0, len(f.workspaces))
	for i := 1; i <= f.nextName; i++ {
		if ws, ok := f.workspaces[fmt.Sprintf("ws%d", i)]; ok {
			all = append(all, proto.Clone(ws).(*cordiumv1.Workspace))
		}
	}

	perPage := int(req.GetCommon().GetItemsPerPage())
	if perPage <= 0 {
		perPage = len(all)
	}
	start := int(req.GetCommon().GetPage()) * perPage
	if start > len(all) {
		start = len(all)
	}
	end := min(start+perPage, len(all))

	return &cordiumv1.WorkspaceList{
		Items: all[start:end],
		ListResponseMeta: &metav1.ListResponseMeta{
			Page:         req.GetCommon().GetPage(),
			ItemsPerPage: uint32(perPage),
			TotalCount:   uint32(len(all)),
			HasMore:      end < len(all),
		},
	}, nil
}

func (f *fakeCluster) WatchWorkspace(req *cordiumv1.WatchWorkspaceRequest,
	srv grpc.ServerStreamingServer[cordiumv1.WatchWorkspaceResponse]) error {

	ch := make(chan *cordiumv1.WatchWorkspaceResponse, 64)

	f.mu.Lock()
	f.watchers[ch] = struct{}{}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		delete(f.watchers, ch)
		f.mu.Unlock()
	}()

	for {
		select {
		case <-srv.Context().Done():
			return srv.Context().Err()
		case msg := <-ch:
			if err := srv.Send(msg); err != nil {
				return err
			}
		}
	}
}

func (f *fakeCluster) Exec(srv grpc.BidiStreamingServer[cordiumv1.ExecRequest, cordiumv1.ExecResponse]) error {
	msg, err := srv.Recv()
	if err != nil {
		return err
	}
	req := msg.GetRequest()
	if req == nil {
		return status.Error(codes.InvalidArgument, "the first message must be a request")
	}

	f.mu.Lock()
	f.lastExecReq = req
	script := f.execScript
	f.mu.Unlock()

	if script == nil {
		return srv.Send(&cordiumv1.ExecResponse{
			Type: &cordiumv1.ExecResponse_Exit_{Exit: &cordiumv1.ExecResponse_Exit{}},
		})
	}
	return script(req, srv)
}

// setState transitions a Workspace and publishes the update to the watchers.
func (f *fakeCluster) setState(name string, state cordiumv1.Workspace_Status_State) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if ws, ok := f.workspaces[name]; ok {
		f.setStateLocked(ws, state)
	}
}

func (f *fakeCluster) setFailure(name string, failure *cordiumv1.Workspace_Status_Failure) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if ws, ok := f.workspaces[name]; ok {
		ws.Status.Failure = failure
		f.setStateLocked(ws, cordiumv1.Workspace_Status_STOPPED)
	}
}

func (f *fakeCluster) setStateLocked(ws *cordiumv1.Workspace, state cordiumv1.Workspace_Status_State) {
	old := proto.Clone(ws).(*cordiumv1.Workspace)
	ws.Status.State = state
	if state == cordiumv1.Workspace_Status_INIT_REQUEST {
		ws.Status.Hostname = ws.Metadata.Name + ".cordium.example.com"
	}

	f.publishLocked(&cordiumv1.WatchWorkspaceResponse{
		Type: &cordiumv1.WatchWorkspaceResponse_Update_{
			Update: &cordiumv1.WatchWorkspaceResponse_Update{
				NewItem: proto.Clone(ws).(*cordiumv1.Workspace),
				OldItem: old,
			},
		},
	})
}

func (f *fakeCluster) publishLocked(msg *cordiumv1.WatchWorkspaceResponse) {
	for ch := range f.watchers {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (f *fakeCluster) watcherCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.watchers)
}

/*
 * Logs and terminals
 */

func (f *fakeCluster) ListenLog(req *cordiumv1.ListenLogRequest,
	srv grpc.ServerStreamingServer[cordiumv1.ListenLogResponse]) error {

	entries := []*cordiumv1.ListenLogResponse{
		{
			CreatedAt: timestamppb.Now(),
			Type:      cordiumv1.ListenLogResponse_TYPE_PULLING_IMAGE,
			Mode:      cordiumv1.ListenLogResponse_MODE_STDOUT,
			Data:      []byte("pulling python:3.11\n"),
		},
		{
			CreatedAt: timestamppb.Now(),
			Type:      cordiumv1.ListenLogResponse_TYPE_TASK,
			Mode:      cordiumv1.ListenLogResponse_MODE_STDERR,
			Data:      []byte("npm WARN deprecated\n"),
		},
	}

	for _, entry := range entries {
		if err := srv.Send(entry); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeCluster) CreateTerminal(ctx context.Context,
	req *cordiumv1.CreateTerminalRequest) (*cordiumv1.CreateTerminalResponse, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.terminalCols, f.terminalRows = req.GetCols(), req.GetRows()
	return &cordiumv1.CreateTerminalResponse{
		Id: req.GetWorkspaceRef().GetName() + "-term1",
	}, nil
}

func (f *fakeCluster) ListTerminal(ctx context.Context,
	req *cordiumv1.ListTerminalRequest) (*cordiumv1.ListTerminalResponse, error) {

	return &cordiumv1.ListTerminalResponse{
		Items: []*cordiumv1.Terminal{{Id: req.GetWorkspaceRef().GetName() + "-term1"}},
	}, nil
}

func (f *fakeCluster) ListenTerminal(req *cordiumv1.ListenTerminalRequest,
	srv grpc.ServerStreamingServer[cordiumv1.ListenTerminalResponse]) error {

	if err := srv.Send(&cordiumv1.ListenTerminalResponse{
		Type: &cordiumv1.ListenTerminalResponse_Stdout_{
			Stdout: &cordiumv1.ListenTerminalResponse_Stdout{Data: []byte("$ ")},
		},
	}); err != nil {
		return err
	}

	f.mu.Lock()
	ch := make(chan []byte, 16)
	f.terminalWrites = ch
	f.mu.Unlock()

	for {
		select {
		case <-srv.Context().Done():
			return srv.Context().Err()
		case data := <-ch:
			if err := srv.Send(&cordiumv1.ListenTerminalResponse{
				Type: &cordiumv1.ListenTerminalResponse_Stdout_{
					Stdout: &cordiumv1.ListenTerminalResponse_Stdout{Data: data},
				},
			}); err != nil {
				return err
			}
		}
	}
}

func (f *fakeCluster) WriteTerminalData(ctx context.Context,
	req *cordiumv1.WriteTerminalDataRequest) (*cordiumv1.WriteTerminalDataResponse, error) {

	f.mu.Lock()
	ch := f.terminalWrites
	f.mu.Unlock()

	if ch != nil {
		select {
		case ch <- req.GetData():
		default:
		}
	}
	return &cordiumv1.WriteTerminalDataResponse{}, nil
}

func (f *fakeCluster) SetTerminalWindowSize(ctx context.Context,
	req *cordiumv1.SetTerminalWindowSizeRequest) (*cordiumv1.SetTerminalWindowSizeResponse, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.terminalCols, f.terminalRows = req.GetCols(), req.GetRows()
	return &cordiumv1.SetTerminalWindowSizeResponse{}, nil
}

func (f *fakeCluster) RemoveTerminal(ctx context.Context,
	req *cordiumv1.RemoveTerminalRequest) (*cordiumv1.RemoveTerminalResponse, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.removedTerminal = req.GetId()
	return &cordiumv1.RemoveTerminalResponse{}, nil
}

/*
 * The Space scoped resources. They echo the request back, with a name assigned
 * where the Cluster would assign one, which is enough for the SDK's own request
 * building to be checked.
 */

func (f *fakeCluster) CreateSpace(ctx context.Context, req *cordiumv1.Space) (*cordiumv1.Space, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastSpace = proto.Clone(req).(*cordiumv1.Space)
	out := proto.Clone(req).(*cordiumv1.Space)
	out.Status.Type = cordiumv1.Space_Status_USER
	if len(req.GetMetadata().GetName()) > len(".cordium") &&
		req.GetMetadata().GetName()[len(req.GetMetadata().GetName())-len(".cordium"):] == ".cordium" {
		out.Status.Type = cordiumv1.Space_Status_ORGANIZATION
	}
	return out, nil
}

func (f *fakeCluster) UpdateSpace(ctx context.Context, req *cordiumv1.Space) (*cordiumv1.Space, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSpace = proto.Clone(req).(*cordiumv1.Space)
	return proto.Clone(req).(*cordiumv1.Space), nil
}

func (f *fakeCluster) GetSpace(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Space, error) {
	return &cordiumv1.Space{
		Metadata: &metav1.Metadata{Name: req.GetName()},
		Spec:     &cordiumv1.Space_Spec{},
		Status:   &cordiumv1.Space_Status{Type: cordiumv1.Space_Status_ORGANIZATION},
	}, nil
}

func (f *fakeCluster) DeleteSpace(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) LeaveSpace(ctx context.Context,
	req *cordiumv1.LeaveSpaceRequest) (*cordiumv1.LeaveSpaceResponse, error) {
	return &cordiumv1.LeaveSpaceResponse{}, nil
}

func (f *fakeCluster) ListSpace(ctx context.Context,
	req *cordiumv1.ListSpaceOptions) (*cordiumv1.SpaceList, error) {

	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSpaceList = req
	return &cordiumv1.SpaceList{ListResponseMeta: &metav1.ListResponseMeta{}}, nil
}

func (f *fakeCluster) CreateTemplate(ctx context.Context, req *cordiumv1.Template) (*cordiumv1.Template, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastTemplate = proto.Clone(req).(*cordiumv1.Template)
	return proto.Clone(req).(*cordiumv1.Template), nil
}

func (f *fakeCluster) UpdateTemplate(ctx context.Context, req *cordiumv1.Template) (*cordiumv1.Template, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastTemplate = proto.Clone(req).(*cordiumv1.Template)
	return proto.Clone(req).(*cordiumv1.Template), nil
}

func (f *fakeCluster) GetTemplate(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Template, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.templateState != nil {
		return proto.Clone(f.templateState).(*cordiumv1.Template), nil
	}
	return &cordiumv1.Template{
		Metadata: &metav1.Metadata{Name: req.GetName()},
		Spec: &cordiumv1.Template_Spec{
			Image: &cordiumv1.Workspace_Spec_Image{
				Type: &cordiumv1.Workspace_Spec_Image_Registry_{
					Registry: &cordiumv1.Workspace_Spec_Image_Registry{Url: "golang:1.25"},
				},
			},
			GitProvider: "github",
		},
		Status: &cordiumv1.Template_Status{},
	}, nil
}

func (f *fakeCluster) DeleteTemplate(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) ListTemplate(ctx context.Context,
	req *cordiumv1.ListTemplateOptions) (*cordiumv1.TemplateList, error) {

	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastTemplateList = req
	return &cordiumv1.TemplateList{ListResponseMeta: &metav1.ListResponseMeta{}}, nil
}

func (f *fakeCluster) BuildTemplate(ctx context.Context,
	req *cordiumv1.BuildTemplateRequest) (*cordiumv1.Template, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastBuild = req
	f.templateState = &cordiumv1.Template{
		Metadata: &metav1.Metadata{Name: req.GetTemplateRef().GetName()},
		Spec:     &cordiumv1.Template_Spec{},
		Status: &cordiumv1.Template_Status{
			BuildInfo: &cordiumv1.Template_Status_BuildInfo{
				CurrentRunningBuildID: "build1",
				Builds: []*cordiumv1.Template_Status_BuildInfo_Build{
					{Id: "build1", State: cordiumv1.Template_Status_BuildInfo_Build_STATE_RUNNING},
				},
			},
		},
	}
	return proto.Clone(f.templateState).(*cordiumv1.Template), nil
}

func (f *fakeCluster) CancelBuildTemplate(ctx context.Context,
	req *cordiumv1.CancelBuildTemplateRequest) (*cordiumv1.Template, error) {
	return &cordiumv1.Template{Status: &cordiumv1.Template_Status{}}, nil
}

// finishBuild moves the running pre-build into a terminal state.
func (f *fakeCluster) finishBuild(state cordiumv1.Template_Status_BuildInfo_Build_State,
	failure *cordiumv1.Workspace_Status_Failure) {

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.templateState == nil {
		return
	}
	f.templateState.Status.BuildInfo.Builds[0].State = state
	f.templateState.Status.BuildInfo.Builds[0].Failure = failure
}

func (f *fakeCluster) CreateSecret(ctx context.Context, req *cordiumv1.Secret) (*cordiumv1.Secret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastSecret = proto.Clone(req).(*cordiumv1.Secret)
	out := proto.Clone(req).(*cordiumv1.Secret)
	out.Data = nil
	return out, nil
}

func (f *fakeCluster) GetSecret(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Secret, error) {
	return &cordiumv1.Secret{Metadata: &metav1.Metadata{Name: req.GetName()}}, nil
}

func (f *fakeCluster) DeleteSecret(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) ListSecret(ctx context.Context,
	req *cordiumv1.ListSecretOptions) (*cordiumv1.SecretList, error) {

	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastSecretList = req
	return &cordiumv1.SecretList{ListResponseMeta: &metav1.ListResponseMeta{}}, nil
}

func (f *fakeCluster) CreateUserSecret(ctx context.Context, req *cordiumv1.UserSecret) (*cordiumv1.UserSecret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastUserSecret = proto.Clone(req).(*cordiumv1.UserSecret)
	out := proto.Clone(req).(*cordiumv1.UserSecret)
	out.Data = nil
	if req.GetSpec().GetType() == cordiumv1.UserSecret_Spec_SSH_KEY {
		out.Status = &cordiumv1.UserSecret_Status{
			Details: &cordiumv1.UserSecret_Status_SshKey{
				SshKey: &cordiumv1.UserSecret_Status_SSHKey{PublicKey: "ecdsa-sha2-nistp256 AAAA"},
			},
		}
	}
	return out, nil
}

func (f *fakeCluster) UpdateUserSecret(ctx context.Context, req *cordiumv1.UserSecret) (*cordiumv1.UserSecret, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastUserSecret = proto.Clone(req).(*cordiumv1.UserSecret)
	return proto.Clone(req).(*cordiumv1.UserSecret), nil
}

func (f *fakeCluster) GetUserSecret(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.UserSecret, error) {
	return &cordiumv1.UserSecret{
		Metadata: &metav1.Metadata{Name: req.GetName()},
		Spec:     &cordiumv1.UserSecret_Spec{},
	}, nil
}

func (f *fakeCluster) DeleteUserSecret(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) ListUserSecret(ctx context.Context,
	req *cordiumv1.ListUserSecretOptions) (*cordiumv1.UserSecretList, error) {
	return &cordiumv1.UserSecretList{ListResponseMeta: &metav1.ListResponseMeta{}}, nil
}

func (f *fakeCluster) CreateGitProvider(ctx context.Context, req *cordiumv1.GitProvider) (*cordiumv1.GitProvider, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastGitProvider = proto.Clone(req).(*cordiumv1.GitProvider)
	return proto.Clone(req).(*cordiumv1.GitProvider), nil
}

func (f *fakeCluster) GetGitProvider(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.GitProvider, error) {
	return &cordiumv1.GitProvider{Metadata: &metav1.Metadata{Name: req.GetName()}}, nil
}

func (f *fakeCluster) DeleteGitProvider(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) ListGitProvider(ctx context.Context,
	req *cordiumv1.ListGitProviderOptions) (*cordiumv1.GitProviderList, error) {
	return &cordiumv1.GitProviderList{ListResponseMeta: &metav1.ListResponseMeta{}}, nil
}

func (f *fakeCluster) CreateMembership(ctx context.Context,
	req *cordiumv1.CreateMembershipRequest) (*cordiumv1.Membership, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	f.lastMembership = req
	return &cordiumv1.Membership{
		Metadata: &metav1.Metadata{Name: "member1"},
		Spec:     &cordiumv1.Membership_Spec{Role: cordiumv1.Membership_Spec_USER},
		Status:   &cordiumv1.Membership_Status{SpaceRef: req.GetSpaceRef()},
	}, nil
}

func (f *fakeCluster) UpdateMembership(ctx context.Context, req *cordiumv1.Membership) (*cordiumv1.Membership, error) {
	return proto.Clone(req).(*cordiumv1.Membership), nil
}

func (f *fakeCluster) GetMembership(ctx context.Context, req *metav1.GetOptions) (*cordiumv1.Membership, error) {
	return &cordiumv1.Membership{
		Metadata: &metav1.Metadata{Name: req.GetName()},
		Spec:     &cordiumv1.Membership_Spec{},
	}, nil
}

func (f *fakeCluster) GetSpaceMembership(ctx context.Context,
	req *cordiumv1.GetSpaceMembershipRequest) (*cordiumv1.Membership, error) {

	return &cordiumv1.Membership{
		Metadata: &metav1.Metadata{Name: "mine"},
		Spec:     &cordiumv1.Membership_Spec{Role: cordiumv1.Membership_Spec_ADMIN},
		Status:   &cordiumv1.Membership_Status{SpaceRef: req.GetSpaceRef()},
	}, nil
}

func (f *fakeCluster) DeleteMembership(ctx context.Context, req *metav1.DeleteOptions) (*metav1.OperationResult, error) {
	return &metav1.OperationResult{}, nil
}

func (f *fakeCluster) ListMembership(ctx context.Context,
	req *cordiumv1.ListMembershipOptions) (*cordiumv1.MembershipList, error) {
	return &cordiumv1.MembershipList{ListResponseMeta: &metav1.ListResponseMeta{}}, nil
}

func (f *fakeCluster) ListRegion(ctx context.Context,
	req *cordiumv1.ListRegionOptions) (*cordiumv1.RegionList, error) {

	return &cordiumv1.RegionList{
		Items: []*cordiumv1.Region{{
			Metadata: &metav1.Metadata{Name: "default"},
			Status:   &cordiumv1.Region_Status{Country: "DE", City: "Frankfurt"},
		}},
		ListResponseMeta: &metav1.ListResponseMeta{},
	}, nil
}

func (f *fakeCluster) GetUserConfig(ctx context.Context,
	req *cordiumv1.GetUserConfigRequest) (*cordiumv1.UserConfig, error) {

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.userConfig == nil {
		f.userConfig = &cordiumv1.UserConfig{
			Metadata: &metav1.Metadata{Name: "jdoe"},
			Spec:     &cordiumv1.UserConfig_Spec{},
			Status:   &cordiumv1.UserConfig_Status{},
		}
	}
	return proto.Clone(f.userConfig).(*cordiumv1.UserConfig), nil
}

func (f *fakeCluster) UpdateUserConfig(ctx context.Context, req *cordiumv1.UserConfig) (*cordiumv1.UserConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.userConfig = proto.Clone(req).(*cordiumv1.UserConfig)
	return proto.Clone(req).(*cordiumv1.UserConfig), nil
}
