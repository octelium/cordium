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

package oproxy

import (
	"net/http"
	"strings"

	"go.uber.org/zap"
)

type Constructor func(http.Handler) (http.Handler, error)

type Chain struct {
	constructors []Constructor
}

func newMiddlewareChain(constructors ...Constructor) Chain {
	return Chain{constructors: constructors}
}

func (c Chain) Then(h http.Handler) (http.Handler, error) {
	if h == nil {
		h = http.DefaultServeMux
	}

	for i := range c.constructors {
		handler, err := c.constructors[len(c.constructors)-1-i](h)
		if err != nil {
			return nil, err
		}
		h = handler
	}

	return h, nil
}

func (c Chain) ThenFunc(fn http.HandlerFunc) (http.Handler, error) {
	if fn == nil {
		return c.Then(nil)
	}
	return c.Then(fn)
}

func (c Chain) Append(constructors ...Constructor) Chain {
	newCons := make([]Constructor, 0, len(c.constructors)+len(constructors))
	newCons = append(newCons, c.constructors...)
	newCons = append(newCons, constructors...)

	return Chain{newCons}
}

func (c Chain) Extend(chain Chain) Chain {
	return c.Append(chain.constructors...)
}

type middleware struct {
	next http.Handler
	p    *OcteliumProxy
}

func newHeaderMiddleware(p *OcteliumProxy, next http.Handler) (http.Handler, error) {
	return &middleware{
		next: next,
		p:    p,
	}, nil
}

func (m *middleware) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	m.setRequestHeaders(req)
	m.next.ServeHTTP(rw, req)
}

func (m *middleware) setRequestHeaders(req *http.Request) {

	m.p.resp.RLock()
	if strings.HasPrefix(req.URL.Path, "/octelium.api.main.auth") {
		zap.L().Debug("Setting refresh token for request", zap.String("path", req.URL.Path))
		req.Header.Set("X-Octelium-Refresh-Token", m.p.resp.resp.RefreshToken)
	} else {
		zap.L().Debug("Setting access token for request", zap.String("path", req.URL.Path))
		req.Header.Set("X-Octelium-Auth", m.p.resp.resp.AccessToken)
	}
	m.p.resp.RUnlock()
}
