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

package mains

import (
	"fmt"
	"testing"

	"context"

	otests "github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/apiserver/apiserver/admin"
	"github.com/octelium/octelium/cluster/common/tests/tstuser"
	"github.com/octelium/octelium/cluster/common/vutils"
	"github.com/octelium/octelium/pkg/grpcerr"
	"github.com/octelium/octelium/pkg/utils/utilrand"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace(t *testing.T) {

	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err)

	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        "rrr",
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)

		assert.NotZero(t, ws.Status.Limit.Memory.Megabytes)
		assert.NotZero(t, ws.Status.Limit.Cpu.Millicores)

		wsV, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		assert.Equal(t, usr.Usr.Metadata.Uid, wsV.Status.UserRef.Uid)

		assert.Nil(t, wsV.Status.SessionRef)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		wsList, err := srv.ListWorkspace(usr.Ctx(), &cordiumv1.ListWorkspaceOptions{})
		assert.Nil(t, err)

		assert.Equal(t, wsV.Metadata.Uid, wsList.Items[0].Metadata.Uid)

		_, err = srv.DeleteWorkspace(usr2.Ctx(), &metav1.DeleteOptions{Uid: ws.Metadata.Uid})
		assert.True(t, grpcerr.IsUnauthorized(err))

		_, err = srv.DeleteWorkspace(usr.Ctx(), &metav1.DeleteOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil,
			corev1.User_Spec_WORKLOAD, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		_, err = srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)
		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        utilrand.GetRandomStringCanonical(8),
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)

		wsV, err := fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		assert.Equal(t, usr.Usr.Metadata.Uid, wsV.Status.UserRef.Uid)

		/*
			assert.NotNil(t, wsV.Status.SessionRef)
			sess, err := fakeC.OcteliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{Uid: wsV.Status.SessionRef.Uid})
			assert.Nil(t, err)
			assert.Equal(t, usr.Session.Metadata.Uid, sess.Status.ParentSessionRef.Uid)
		*/
	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        utilrand.GetRandomStringCanonical(8),
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)

		assert.Nil(t, ws.Status.SessionRef)
		assert.Equal(t, 0, len(ws.Status.Runs))

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)

		ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		assert.Equal(t, cordiumv1.Workspace_Status_INIT_REQUEST, ws.Status.State)

		assert.Equal(t, 1, len(ws.Status.Runs))
		assert.True(t, ws.Status.Runs[0].Id != "")
		assert.True(t, ws.Status.Runs[0].InitializedAt.IsValid())

		assert.NotNil(t, ws.Status.SessionRef)
		// sess, err := fakeC.OcteliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{Uid: ws.Status.SessionRef.Uid})
		// assert.Nil(t, err)
		// assert.Equal(t, usr.Session.Metadata.Uid, sess.Status.ParentSessionRef.Uid)

		_, err = srv.StopWorkspace(usr.Ctx(), &cordiumv1.StopWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)
		ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err, "%+v", err)
		ws.Status.State = cordiumv1.Workspace_Status_RUNNING
		ws, err = fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws)
		assert.Nil(t, err)

		_, err = srv.StopWorkspace(usr.Ctx(), &cordiumv1.StopWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)

		ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		assert.Equal(t, cordiumv1.Workspace_Status_STOPPING_REQUEST, ws.Status.State)

	}

	{
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		usr2, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        utilrand.GetRandomStringCanonical(8),
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)

		assert.Nil(t, ws.Status.SessionRef)

		_, err = srv.StartWorkspace(usr2.Ctx(), &cordiumv1.StartWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)

		_, err = srv.StartWorkspace(usr.Ctx(), &cordiumv1.StartWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)

		ws, err = fakeC.OcteliumC.CordiumC().GetWorkspace(ctx, &rmetav1.GetOptions{Uid: ws.Metadata.Uid})
		assert.Nil(t, err)
		ws.Status.State = cordiumv1.Workspace_Status_RUNNING
		ws, err = fakeC.OcteliumC.CordiumC().UpdateWorkspace(ctx, ws)
		assert.Nil(t, err)

		_, err = srv.StopWorkspace(usr2.Ctx(), &cordiumv1.StopWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsUnauthorized(err), "%+v", err)

		_, err = srv.StopWorkspace(usr.Ctx(), &cordiumv1.StopWorkspaceRequest{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)
	}

	t.Run("share-ports", func(t *testing.T) {
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.cordium", utilrand.GetRandomStringCanonical(8)),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        "rrr",
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{
				Applications: []*cordiumv1.Workspace_Spec_Application{
					{
						Name: "app-1",
						Port: 8080,
					},
				},
			},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: &metav1.ObjectReference{
					Name: fmt.Sprintf("default.%s", spc.Metadata.Name),
				},
			},
		})
		assert.Nil(t, err, "%+v", err)

		_, err = srv.ShareWorkspacePort(usr.Ctx(), &cordiumv1.ShareWorkspacePortRequest{
			Uid:             ws.Metadata.Uid,
			ApplicationName: "app-1",
		})
		assert.NotNil(t, err)

		_, err = srv.ShareWorkspacePort(usr.Ctx(), &cordiumv1.ShareWorkspacePortRequest{
			Uid:             ws.Metadata.Uid,
			ApplicationName: "does-not-exist",
			Mode:            cordiumv1.ShareWorkspacePortRequest_MEMBERS,
		})
		assert.NotNil(t, err)
		assert.True(t, grpcerr.IsInvalidArg(err))

		_, err = srv.ShareWorkspacePort(usr.Ctx(), &cordiumv1.ShareWorkspacePortRequest{
			Uid:             ws.Metadata.Uid,
			ApplicationName: "app-1",
			Mode:            cordiumv1.ShareWorkspacePortRequest_MEMBERS,
		})
		assert.Nil(t, err)

		ws, err = srv.GetWorkspace(usr.Ctx(), &metav1.GetOptions{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)

		assert.Equal(t, 1, len(ws.Status.SharedPorts))
		assert.Equal(t, "app-1", ws.Status.SharedPorts[0].ApplicationName)
		assert.Equal(t, cordiumv1.Workspace_Status_SharedPort_MEMBERS, ws.Status.SharedPorts[0].Mode)

		{
			_, err = srv.ShareWorkspacePort(usr.Ctx(), &cordiumv1.ShareWorkspacePortRequest{
				Uid:             ws.Metadata.Uid,
				ApplicationName: "app-1",
				Mode:            cordiumv1.ShareWorkspacePortRequest_ALL,
			})
			assert.Nil(t, err)

			ws, err = srv.GetWorkspace(usr.Ctx(), &metav1.GetOptions{
				Uid: ws.Metadata.Uid,
			})
			assert.Nil(t, err)

			assert.Equal(t, 1, len(ws.Status.SharedPorts))
			assert.Equal(t, "app-1", ws.Status.SharedPorts[0].ApplicationName)
			assert.Equal(t, cordiumv1.Workspace_Status_SharedPort_ALL, ws.Status.SharedPorts[0].Mode)
		}

		_, err = srv.UnshareWorkspacePort(usr.Ctx(), &cordiumv1.UnshareWorkspacePortRequest{
			Uid:             ws.Metadata.Uid,
			ApplicationName: "app-1",
		})
		assert.Nil(t, err)

		ws, err = srv.GetWorkspace(usr.Ctx(), &metav1.GetOptions{
			Uid: ws.Metadata.Uid,
		})
		assert.Nil(t, err)

		assert.Equal(t, 0, len(ws.Status.SharedPorts))

		_, err = srv.UnshareWorkspacePort(usr.Ctx(), &cordiumv1.UnshareWorkspacePortRequest{
			Uid:             ws.Metadata.Uid,
			ApplicationName: "app-1",
		})
		assert.Nil(t, err)

		_, err = srv.UnshareWorkspacePort(usr.Ctx(), &cordiumv1.UnshareWorkspacePortRequest{
			Uid:             ws.Metadata.Uid,
			ApplicationName: "does-not-exist",
		})
		assert.Nil(t, err)
	})

	t.Run("default-space-template-create", func(t *testing.T) {
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        "rrr",
				DisplayName: "My Workspace",
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		})
		assert.Nil(t, err, "%+v", err)

		spc, err := srv.GetSpace(usr.Ctx(), &metav1.GetOptions{
			Name: ws.Status.SpaceRef.Name,
		})
		assert.Nil(t, err)
		assert.Nil(t, srv.checkResourceDefault(spc))

		tmpl, err := srv.GetTemplate(usr.Ctx(), &metav1.GetOptions{
			Name: ws.Status.TemplateRef.Name,
		})
		assert.Nil(t, err)
		assert.Nil(t, srv.checkResourceDefault(tmpl))
	})

}

/*
func TestWorkspaceMerged(t *testing.T) {

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C
	tstAllowAllOwnSpace(t, fakeC.OcteliumC)
	srv, err := NewServer(context.Background(), fakeC.OcteliumC)
	assert.Nil(t, err)
	adminSrv := admin.NewServer(&admin.Opts{
		OcteliumC:  fakeC.OcteliumC,
		IsEmbedded: true,
	})

	t.Run("project", func(t *testing.T) {
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		prj, err := srv.CreateEnvironment(usr.Ctx(), &cordiumv1.Environment{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), spc.Metadata.Name),
			},
			Spec: &cordiumv1.Environment_Spec{
				Repository: &cordiumv1.Workspace_Spec_Repository{
					Url: "https://github.com/linux/linux",
				},
			},
			Status: &cordiumv1.Environment_Status{
				SpaceRef: umetav1.GetObjectReference(spc),
			},
		})
		assert.Nil(t, err)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        "rrr",
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				EnvironmentRef: umetav1.GetObjectReference(prj),
			},
		})
		assert.Nil(t, err, "%+v", err)

		assert.True(t, pbutils.IsEqual(ws.Status.EnvironmentRef, umetav1.GetObjectReference(prj)))

		assert.Equal(t, prj.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)

		ws.Spec.Repository = &cordiumv1.Workspace_Spec_Repository{
			Url: "https://github.com/linux2/linux2",
		}

		ws, err = srv.UpdateWorkspace(usr.Ctx(), ws)
		assert.Nil(t, err)

		assert.Equal(t, ws.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)
	})

	t.Run("template", func(t *testing.T) {
		usr, err := tstuser.NewUserWithType(fakeC.OcteliumC, adminSrv, nil, nil, corev1.User_Spec_HUMAN, corev1.Session_Status_CLIENTLESS)
		assert.Nil(t, err)

		spc, err := srv.CreateSpace(usr.Ctx(), &cordiumv1.Space{
			Metadata: &metav1.Metadata{
				Name: utilrand.GetRandomStringCanonical(8),
			},
			Spec: &cordiumv1.Space_Spec{},
			Status: &cordiumv1.Space_Status{
				Type: cordiumv1.Space_Status_ORGANIZATION,
			},
		})
		assert.Nil(t, err)

		prj, err := srv.CreateEnvironment(usr.Ctx(), &cordiumv1.Environment{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), spc.Metadata.Name),
			},
			Spec: &cordiumv1.Environment_Spec{
				Repository: &cordiumv1.Workspace_Spec_Repository{
					Url: "https://github.com/linux/linux",
				},
				Runtime: &cordiumv1.Workspace_Spec_Runtime{
					Entrypoint: "/usr/bin/init",
				},
			},
			Status: &cordiumv1.Environment_Status{},
		})
		assert.Nil(t, err)

		tmpl, err := srv.CreateTemplate(usr.Ctx(), &cordiumv1.Template{
			Metadata: &metav1.Metadata{
				Name: fmt.Sprintf("%s.%s", utilrand.GetRandomStringCanonical(8), prj.Metadata.Name),
			},
			Spec: &cordiumv1.Template_Spec{
				Repository: &cordiumv1.Workspace_Spec_Repository{
					Url: "https://github.com/linux2/linux2",
				},
			},
			Status: &cordiumv1.Template_Status{},
		})
		assert.Nil(t, err)

		assert.Equal(t, tmpl.Spec.Repository.Url, ucordiumv1.ToTemplate(tmpl).GetMergedSpec().Repository.Url)
		assert.Equal(t, "/usr/bin/init", ucordiumv1.ToTemplate(tmpl).GetMergedSpec().Runtime.Entrypoint)

		ws, err := srv.CreateWorkspace(usr.Ctx(), &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name:        "rrr",
				DisplayName: "My Workspace",
			},
			Spec: &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{
				TemplateRef: umetav1.GetObjectReference(tmpl),
			},
		})
		assert.Nil(t, err, "%+v", err)

		assert.True(t, pbutils.IsEqual(ws.Status.EnvironmentRef, umetav1.GetObjectReference(prj)))
		assert.True(t, pbutils.IsEqual(ws.Status.TemplateRef, umetav1.GetObjectReference(tmpl)))

		assert.Equal(t, tmpl.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)

		ws.Spec.Repository = &cordiumv1.Workspace_Spec_Repository{
			Url: "https://github.com/linux3/linux3",
		}

		ws, err = srv.UpdateWorkspace(usr.Ctx(), ws)
		assert.Nil(t, err)

		assert.Equal(t, ws.Spec.Repository.Url, ucordiumv1.ToWorkspace(ws).GetMergedSpec().Repository.Url)
		assert.Equal(t, "/usr/bin/init", ucordiumv1.ToWorkspace(ws).GetMergedSpec().Runtime.Entrypoint)
	})

}
*/

func TestWorkspaceLimit(t *testing.T) {

	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	srv, err := NewServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err)

	ws := &cordiumv1.Workspace{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		},
		Spec:   &cordiumv1.Workspace_Spec{},
		Status: &cordiumv1.Workspace_Status{},
	}

	spc := &cordiumv1.Space{
		Metadata: &metav1.Metadata{
			Name: utilrand.GetRandomStringCanonical(8),
			Uid:  vutils.UUIDv4(),
		},
		Spec: &cordiumv1.Space_Spec{},
		Status: &cordiumv1.Space_Status{
			Type: cordiumv1.Space_Status_ORGANIZATION,
		},
	}

	cc, err := srv.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err)
	err = srv.setWorkspaceLimit(ctx, ws, spc, cc)
	assert.Nil(t, err)

	assert.NotZero(t, ws.Status.Limit.Cpu.Millicores)
	assert.NotZero(t, ws.Status.Limit.Memory.Megabytes)

	assert.NotZero(t, ws.Status.Limit.Storage.Megabytes)

	cc, err = srv.octeliumC.CordiumV1Utils().GetClusterConfig(ctx)
	assert.Nil(t, err)

	cc.Spec.Workspace = &cordiumv1.ClusterConfig_Spec_Workspace{
		Limit: &cordiumv1.ClusterConfig_Spec_Workspace_Limit{

			DefaultOrganizationSpaceLimit: &cordiumv1.Workspace_Spec_Limit{
				Cpu: &cordiumv1.Workspace_Spec_Limit_CPU{
					Millicores: uint32(utilrand.GetRandomRangeMath(1000, 2000)),
				},
				Memory: &cordiumv1.Workspace_Spec_Limit_Memory{
					Megabytes: uint32(utilrand.GetRandomRangeMath(1000, 2000)),
				},

				Storage: &cordiumv1.Workspace_Spec_Limit_Storage{
					Megabytes: uint32(utilrand.GetRandomRangeMath(1000, 2000)),
				},
			},
		},
	}

	cc, err = srv.octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err)

	ws.Status.Limit = nil
	err = srv.setWorkspaceLimit(ctx, ws, spc, cc)
	assert.Nil(t, err)
	assert.Equal(t, cc.Spec.Workspace.Limit.DefaultOrganizationSpaceLimit.Cpu.Millicores,
		ws.Status.Limit.Cpu.Millicores)
	assert.Equal(t, cc.Spec.Workspace.Limit.DefaultOrganizationSpaceLimit.Memory.Megabytes,
		ws.Status.Limit.Memory.Megabytes)

	assert.Equal(t, cc.Spec.Workspace.Limit.DefaultOrganizationSpaceLimit.Storage.Megabytes,
		ws.Status.Limit.Storage.Megabytes)

	cc.Spec.Workspace.Limit.MaxLimit = &cordiumv1.Workspace_Spec_Limit{
		Cpu: &cordiumv1.Workspace_Spec_Limit_CPU{
			Millicores: uint32(utilrand.GetRandomRangeMath(500, 900)),
		},
		Memory: &cordiumv1.Workspace_Spec_Limit_Memory{
			Megabytes: uint32(utilrand.GetRandomRangeMath(500, 900)),
		},

		Storage: &cordiumv1.Workspace_Spec_Limit_Storage{
			Megabytes: uint32(utilrand.GetRandomRangeMath(500, 900)),
		},
	}
	cc, err = srv.octeliumC.CordiumC().UpdateClusterConfig(ctx, cc)
	assert.Nil(t, err)

	ws.Status.Limit = nil
	err = srv.setWorkspaceLimit(ctx, ws, spc, cc)
	assert.Nil(t, err)
	assert.Equal(t, cc.Spec.Workspace.Limit.MaxLimit.Cpu.Millicores, ws.Status.Limit.Cpu.Millicores)
	assert.Equal(t, cc.Spec.Workspace.Limit.MaxLimit.Memory.Megabytes, ws.Status.Limit.Memory.Megabytes)

	assert.Equal(t, cc.Spec.Workspace.Limit.MaxLimit.Storage.Megabytes, ws.Status.Limit.Storage.Megabytes)

}

func TestGenWorkspaceName(t *testing.T) {

	ctx := context.Background()

	tst, err := otests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})
	fakeC := tst.C

	srv, err := NewServer(ctx, fakeC.OcteliumC)
	assert.Nil(t, err)

	for i := 0; i < 3000; i++ {
		name, err := srv.genWorkspaceName(ctx)
		assert.Nil(t, err)
		assert.LessOrEqual(t, len(name), 4)

		ws := &cordiumv1.Workspace{
			Metadata: &metav1.Metadata{
				Name: name,
			},
			Spec:   &cordiumv1.Workspace_Spec{},
			Status: &cordiumv1.Workspace_Status{},
		}

		_, err = srv.octeliumC.CordiumC().CreateWorkspace(ctx, ws)
		assert.Nil(t, err)
	}
}
