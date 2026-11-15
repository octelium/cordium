package portal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type testNetstackPeer struct {
	tun      *netTun
	device   *device.Device
	key      wgtypes.Key
	endpoint string
}

func newTestNetstackPeer(t *testing.T, address string) *testNetstackPeer {
	t.Helper()

	_, addr, err := net.ParseCIDR(address)
	require.Nil(t, err)
	tundev, err := createNetstackTUN(addr)
	require.Nil(t, err)
	key, err := wgtypes.GeneratePrivateKey()
	require.Nil(t, err)
	dev := device.NewDevice(tundev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelError, ""))
	t.Cleanup(dev.Close)
	require.Nil(t, dev.IpcSet(fmt.Sprintf("private_key=%s\nlisten_port=0\n", wgKeyB64ToHex(key.String()))))
	require.Nil(t, dev.Up())
	info, err := dev.IpcGet()
	require.Nil(t, err)
	var port string
	for _, line := range strings.Split(info, "\n") {
		if value, ok := strings.CutPrefix(line, "listen_port="); ok {
			port = value
			break
		}
	}
	require.NotEmpty(t, port)
	require.NotEqual(t, "0", port)

	return &testNetstackPeer{
		tun:      tundev,
		device:   dev,
		key:      key,
		endpoint: net.JoinHostPort("127.0.0.1", port),
	}
}

func TestNetstackWireGuard(t *testing.T) {
	portal := newTestNetstackPeer(t, "10.100.100.2/32")
	workspace := newTestNetstackPeer(t, "10.100.100.1/32")
	require.Nil(t, workspace.device.IpcSet(fmt.Sprintf(
		"replace_peers=true\npublic_key=%s\nreplace_allowed_ips=true\nallowed_ip=10.100.100.2/32\n",
		wgKeyB64ToHex(portal.key.PublicKey().String()),
	)))
	require.Nil(t, portal.device.IpcSet(toUAPI(portal.key, workspace.key.PublicKey(), nil, workspace.endpoint)))

	t.Run("TCP", func(t *testing.T) {
		listener, err := workspace.tun.GetNetstackNet().ListenTCP(&net.TCPAddr{
			IP:   net.ParseIP("10.100.100.1"),
			Port: 3000,
		})
		require.Nil(t, err)
		server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if !assert.Nil(t, err) {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, err = w.Write(body)
			assert.Nil(t, err)
		})}
		served := make(chan error, 1)
		go func() { served <- server.Serve(listener) }()
		t.Cleanup(func() {
			assert.Nil(t, server.Close())
			assert.ErrorIs(t, <-served, http.ErrServerClosed)
		})
		transport := &http.Transport{
			DialContext:       portal.tun.GetNetstackNet().DialContext,
			DisableKeepAlives: true,
		}
		t.Cleanup(transport.CloseIdleConnections)
		client := &http.Client{Transport: transport}
		payload := bytes.Repeat([]byte("portal-wireguard-netstack"), 256)
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, "POST", "http://10.100.100.1:3000/", bytes.NewReader(payload))
		require.Nil(t, err)
		resp, err := client.Do(req)
		require.Nil(t, err)
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		require.Nil(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, payload, body)
	})

	t.Run("UDP", func(t *testing.T) {
		address := &net.UDPAddr{IP: net.ParseIP("10.100.100.1"), Port: 3001}
		server, err := workspace.tun.GetNetstackNet().DialUDP(address, nil)
		require.Nil(t, err)
		t.Cleanup(func() { assert.Nil(t, server.Close()) })
		client, err := portal.tun.GetNetstackNet().DialUDP(nil, address)
		require.Nil(t, err)
		t.Cleanup(func() { assert.Nil(t, client.Close()) })
		require.Nil(t, server.SetDeadline(time.Now().Add(10*time.Second)))
		require.Nil(t, client.SetDeadline(time.Now().Add(10*time.Second)))

		_, err = client.Write([]byte("request"))
		require.Nil(t, err)
		buf := make([]byte, 128)
		n, peer, err := server.ReadFrom(buf)
		require.Nil(t, err)
		assert.Equal(t, "request", string(buf[:n]))
		_, err = server.WriteTo([]byte("response"), peer)
		require.Nil(t, err)
		n, err = client.Read(buf)
		require.Nil(t, err)
		assert.Equal(t, "response", string(buf[:n]))
	})
}
