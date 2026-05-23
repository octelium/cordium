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

package sshagent

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	utils_cert "github.com/octelium/octelium/pkg/utils/cert"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const SocketPath = "/tmp/octelium-ssh-agent.socket"

type privKey struct {
	signer  ssh.Signer
	comment string
	expire  *time.Time
}

type keyring struct {
	mu   sync.Mutex
	keys []privKey

	locked bool
}

var errLocked = errors.New("agent: locked")
var errUnsupported = errors.New("unsupported operation")

func NewKeyring() agent.Agent {
	return &keyring{}
}

func (r *keyring) RemoveAll() error {

	return errUnsupported
}

func (r *keyring) Remove(key ssh.PublicKey) error {
	return errUnsupported
}

func (r *keyring) Lock(passphrase []byte) error {
	return errUnsupported
}

func (r *keyring) Unlock(passphrase []byte) error {
	return errUnsupported
}

func (r *keyring) List() ([]*agent.Key, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locked {
		return nil, nil
	}

	var ids []*agent.Key
	for _, k := range r.keys {
		pub := k.signer.PublicKey()
		ids = append(ids, &agent.Key{
			Format:  pub.Type(),
			Blob:    pub.Marshal(),
			Comment: k.comment})
	}
	return ids, nil
}

func (r *keyring) Add(key agent.AddedKey) error {

	return errUnsupported
}

func (r *keyring) Sign(key ssh.PublicKey, data []byte) (*ssh.Signature, error) {
	return r.SignWithFlags(key, data, 0)
}

func (r *keyring) SignWithFlags(key ssh.PublicKey, data []byte, flags agent.SignatureFlags) (*ssh.Signature, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locked {
		return nil, errLocked
	}

	wanted := key.Marshal()
	for _, k := range r.keys {
		if bytes.Equal(k.signer.PublicKey().Marshal(), wanted) {
			if flags == 0 {
				return k.signer.Sign(rand.Reader, data)
			} else {
				if algorithmSigner, ok := k.signer.(ssh.AlgorithmSigner); !ok {
					return nil, fmt.Errorf("agent: signature does not support non-default signature algorithm: %T", k.signer)
				} else {
					var algorithm string
					switch flags {
					case agent.SignatureFlagRsaSha256:
						algorithm = ssh.KeyAlgoRSASHA256
					case agent.SignatureFlagRsaSha512:
						algorithm = ssh.KeyAlgoRSASHA512
					default:
						return nil, fmt.Errorf("agent: unsupported signature flags: %d", flags)
					}
					return algorithmSigner.SignWithAlgorithm(rand.Reader, data, algorithm)
				}
			}
		}
	}
	return nil, errors.New("not found")
}

func (r *keyring) Signers() ([]ssh.Signer, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.locked {
		return nil, errLocked
	}

	s := make([]ssh.Signer, 0, len(r.keys))
	for _, k := range r.keys {
		s = append(s, k.signer)
	}
	return s, nil
}

func (r *keyring) Extension(extensionType string, contents []byte) ([]byte, error) {
	return nil, agent.ErrExtensionUnsupported
}

type Agent struct {
	keyring *keyring
}

type Opts struct {
	UserSecretList *cordiumv1.UserSecretList
}

func NewAgent(opts *Opts) (*Agent, error) {
	zap.L().Debug("Initializing a new SSH agent")

	ret := &Agent{
		keyring: &keyring{},
	}

	if opts == nil || opts.UserSecretList == nil || len(opts.UserSecretList.Items) == 0 {
		return ret, nil
	}

	for _, sec := range opts.UserSecretList.Items {

		if sec.Spec.Type != cordiumv1.UserSecret_Spec_SSH_KEY {
			continue
		}

		zap.L().Debug("Adding secret to SSH keyring", zap.String("name", sec.Metadata.Name))
		k, err := utils_cert.LoadECDSA(sec.Data.GetValueBytes())
		if err != nil {
			continue
		}

		signer, err := ssh.NewSignerFromKey(k.PrivateKey)
		if err != nil {
			continue
		}

		ret.keyring.keys = append(ret.keyring.keys, privKey{
			signer:  signer,
			comment: fmt.Sprintf("cordium-%s", sec.Metadata.Name),
		})
	}

	zap.L().Debug("Successfully initialized the SSH agent", zap.Int("keys", len(ret.keyring.keys)))
	return ret, nil
}

func (a *Agent) serveConn(c net.Conn) {
	if err := agent.ServeAgent(a.keyring, c); err != io.EOF {
		zap.L().Error("Agent client connection ended with error:", zap.Error(err))
	}
}

func (a *Agent) Run(ctx context.Context) error {
	go func(ctx context.Context) {
		if err := a.doRun(ctx); err != nil {
			zap.L().Error("Could not doRun for ssh agent", zap.Error(err))
		}
	}(ctx)
	zap.L().Debug("SSH agent started running")
	return nil
}

func (a *Agent) doRun(ctx context.Context) error {
	os.Remove(SocketPath)
	lis, err := net.Listen("unix", SocketPath)
	if err != nil {
		zap.L().Error("Could not listen to unix path", zap.Error(err), zap.String("path", SocketPath))
		return err
	}

	if err := os.Chmod(SocketPath, 0766); err != nil {
		return err
	}

	type temporary interface {
		Temporary() bool
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			c, err := lis.Accept()
			if err != nil {

				if err, ok := err.(temporary); ok && err.Temporary() {
					zap.L().Error("Could not temporarily accept connection")
					time.Sleep(1 * time.Second)
					continue
				}
				zap.L().Error("Could not accept connection", zap.Error(err))
			}
			go a.serveConn(c)
		}
	}
}
