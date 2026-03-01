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

package auth

import (
	"context"
	"net/http"

	"github.com/octelium/cordium/cluster/common/octeliumc"
	"github.com/octelium/cordium/cluster/portal/portal/middlewares"
	"github.com/octelium/octelium/apis/cluster/coctovigilv1"
	"github.com/octelium/octelium/apis/main/corev1"
	"github.com/octelium/octelium/apis/rsc/rmetav1"
	"github.com/octelium/octelium/cluster/common/octovigilc"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type middleware struct {
	octeliumC  octeliumc.ClientInterface
	next       http.Handler
	octovigilC octovigilc.ClientInterface
}

func New(ctx context.Context, next http.Handler, octeliumC octeliumc.ClientInterface) (http.Handler, error) {

	octovigilC, err := octovigilc.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &middleware{
		next:       next,
		octeliumC:  octeliumC,
		octovigilC: octovigilC,
	}, nil
}

func (m *middleware) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	reqCtx, err := m.getContext(req)
	if err != nil {
		zap.L().Debug("Could no get reqCtx", zap.Error(err))
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	rCtx := middlewares.GetCtxRequestContext(req.Context())
	rCtx.Session = reqCtx.Session

	m.next.ServeHTTP(w, req)
}

func (m *middleware) getContext(r *http.Request) (*middlewares.RequestContext, error) {
	ctx := r.Context()

	sessUID := r.Header.Get("X-Octelium-Session-Uid")
	if sessUID == "" {
		return nil, errors.Errorf("Session UID is empty")
	}

	var err error
	var sess *corev1.Session
	// var usr *corev1.User

	if ldflags.IsTest() {

		sess, err = m.octeliumC.CoreC().GetSession(ctx, &rmetav1.GetOptions{
			Uid: sessUID,
		})
		if err != nil {
			return nil, err
		}

		/*
			usr, err = m.octeliumC.CoreC().GetUser(ctx, &rmetav1.GetOptions{
				Uid: sess.Status.UserRef.Uid,
			})
			if err != nil {
				return nil, err
			}
		*/
	} else {
		di, err := m.octovigilC.InternalC().GetDownstreamFromSessionUID(ctx,
			&coctovigilv1.GetDownstreamFromSessionUIDRequest{
				SessionUID: sessUID,
			})
		if err != nil {
			return nil, errors.Errorf("Could not get downstream: %+v", err)
		}

		sess = di.Session
		// usr = di.User
	}

	if sess.Status.Type != corev1.Session_Status_CLIENTLESS {
		return nil, errors.Errorf("Session type is not CLIENTLESS")
	}

	/*
		if sess.Status.DeviceRef == nil {
			return nil, errors.Errorf("Deviceless Session")
		}

		if usr.Spec.Type != corev1.User_Spec_HUMAN {
			return nil, errors.Errorf("User type is not HUMAN")
		}
	*/

	return &middlewares.RequestContext{
		Session: sess,
	}, nil
}
