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
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/octelium/octelium/octelium-go/authc"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func (p *OcteliumProxy) getHTTPHandler(ctx context.Context) (http.Handler, error) {
	chain := newMiddlewareChain()

	chain = chain.Append(func(next http.Handler) (http.Handler, error) {
		return newHeaderMiddleware(p, next)
	})

	handler, err := chain.Then(p)
	if err != nil {
		return nil, err
	}

	handler = http.AllowQuerySemicolons(handler)

	handler = h2c.NewHandler(handler, &http2.Server{})

	return handler, nil
}

func (s *OcteliumProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxy, err := s.getProxy()
	if err != nil {
		zap.S().Debugf("Could not get proxy: %+v", err)
		w.WriteHeader(http.StatusBadGateway)
		return
	}

	proxy.ServeHTTP(w, r)
}

func (p *OcteliumProxy) getProxy() (http.Handler, error) {

	roundTripper, err := p.getRoundTripper(p.opts.Domain)
	if err != nil {
		return nil, err
	}

	ret := &httputil.ReverseProxy{
		BufferPool: newBufferPool(),
		Transport:  roundTripper,
		Director: func(outReq *http.Request) {

			host := authc.GetAPIServerAddr(p.opts.Domain)
			outReq.URL.Scheme = "https"
			outReq.URL.Host = host
			outReq.Host = host

			outReq.URL.RawQuery = strings.ReplaceAll(outReq.URL.RawQuery, ";", "&")
			outReq.RequestURI = ""

			if _, ok := outReq.Header["User-Agent"]; !ok {
				outReq.Header.Set("User-Agent", "octelium")
			}

			outReq.Proto = "HTTP/2"
			outReq.ProtoMajor = 2
			outReq.ProtoMinor = 0

		},

		FlushInterval: time.Duration(100 * time.Millisecond),

		ErrorHandler: func(w http.ResponseWriter, request *http.Request, err error) {
			statusCode := http.StatusInternalServerError
			zap.S().Debugf("Handling response err: %+v", err)
			switch {
			case errors.Is(err, io.EOF):
				statusCode = http.StatusBadGateway
			case errors.Is(err, context.Canceled):
				statusCode = 499
			default:
				var netErr net.Error
				if errors.As(err, &netErr) {
					if netErr.Timeout() {
						statusCode = http.StatusGatewayTimeout
					} else {
						statusCode = http.StatusBadGateway
					}
				}
			}

			zap.S().Debugf("'%d %s' caused by: %v", statusCode, http.StatusText(statusCode), err)
			w.WriteHeader(statusCode)
			_, werr := w.Write([]byte(http.StatusText(statusCode)))
			if werr != nil {
				zap.S().Debugf("Error while writing status code: %+v", werr)
			}
		},
	}

	return ret, nil
}

const bufferPoolSize = 32 * 1024

func newBufferPool() *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferPoolSize)
			},
		},
	}
}

type bufferPool struct {
	pool sync.Pool
}

func (b *bufferPool) Get() []byte {
	return b.pool.Get().([]byte)
}

func (b *bufferPool) Put(bytes []byte) {
	b.pool.Put(bytes)
}

type roundTripper struct {
	sniName string
}

func (p *OcteliumProxy) getRoundTripper(sniName string) (*roundTripper, error) {
	return &roundTripper{
		sniName: sniName,
	}, nil
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {

	rt, err := r.getRoundTripper(req)
	if err != nil {
		return nil, err
	}

	return rt.RoundTrip(req)
}

func (r *roundTripper) getRoundTripper(req *http.Request) (http.RoundTripper, error) {

	tlsCfg := &tls.Config{
		MinVersion:         tls.VersionTLS12,
		MaxVersion:         tls.VersionTLS13,
		ServerName:         r.sniName,
		InsecureSkipVerify: ldflags.IsDev(),
	}

	return r.getRoundTripperHTTP2(req, tlsCfg)

}

func (r *roundTripper) getRoundTripperHTTP2(req *http.Request, tlsCfg *tls.Config) (http.RoundTripper, error) {
	ret, err := r.getRoundTripperHTTP1(tlsCfg)
	if err != nil {
		return nil, err
	}
	_, err = http2.ConfigureTransports(ret)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *roundTripper) getRoundTripperHTTP1(tlsCfg *tls.Config) (*http.Transport, error) {

	ret := &http.Transport{
		TLSClientConfig: tlsCfg,
		Proxy:           http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}

			return dialer.DialContext(ctx, network, addr)
		},

		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ReadBufferSize:        64 * 1024,
		WriteBufferSize:       64 * 1024,
	}

	return ret, nil
}
