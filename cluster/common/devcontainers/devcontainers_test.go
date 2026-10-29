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

package devcontainers

import (
	"encoding/json"
	"os/exec"
	"slices"
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestDevContainers(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	{

		spec := &Spec{}

		data := `
		{
			/*
			 This is a comment
			*/
		"name": "my-project-devcontainer",
		"image": "mcr.microsoft.com/devcontainers/base:ubuntu",  // Any generic, debian-based image.
		"features": {
			"ghcr.io/devcontainers/features/go:1": {
				"version": "1.18"
			},
			"ghcr.io/devcontainers/features/docker-in-docker:1": {
				"version": "latest",
				"moby": true
			}
		}
	}
		`

		spec, err := LoadSpec(&LoadSpecOpts{
			Data: []byte(data),
		})
		assert.Nil(t, err)
		zap.L().Debug("Spec", zap.Any("spec", spec))
	}

	{

		data := `
		// For format details, see https://aka.ms/vscode-remote/devcontainer.json or this file's README at:
		// https://github.com/microsoft/vscode-dev-containers/tree/v0.126.0/containers/python-3
		{
			"name": "Python 3",
			"build": {
				"dockerfile": "Dockerfile",
				"context": "..",
				// Update 'VARIANT' to pick a Python version. Rebuild the container 
				// if it already exists to update. Available variants: 3, 3.6, 3.7, 3.8 
				"args": { "VARIANT": "3" }
			},
		
			// Set *default* container specific settings.json values on container create.
			"settings": { 
				"terminal.integrated.shell.linux": "/bin/bash",
				"python.pythonPath": "/usr/local/bin/python",
				"python.linting.enabled": true,
				"python.linting.pylintEnabled": true,
				"python.formatting.autopep8Path": "/usr/local/py-utils/bin/autopep8",
				"python.formatting.blackPath": "/usr/local/py-utils/bin/black",
				"python.formatting.yapfPath": "/usr/local/py-utils/bin/yapf",
				"python.linting.banditPath": "/usr/local/py-utils/bin/bandit",
				"python.linting.flake8Path": "/usr/local/py-utils/bin/flake8",
				"python.linting.mypyPath": "/usr/local/py-utils/bin/mypy",
				"python.linting.pycodestylePath": "/usr/local/py-utils/bin/pycodestyle",
				"python.linting.pydocstylePath": "/usr/local/py-utils/bin/pydocstyle",
				"python.linting.pylintPath": "/usr/local/py-utils/bin/pylint"
			},
		
			// Add the IDs of extensions you want installed when the container is created.
			"extensions": [
				"ms-python.python"
			],
		
			// Use 'forwardPorts' to make a list of ports inside the container available locally.
			// "forwardPorts": [],
		
			// Use 'postCreateCommand' to run commands after the container is created.
			"postCreateCommand": "pip3 install --user -r requirements.txt",
		
			// Uncomment to connect as a non-root user. See https://aka.ms/vscode-remote/containers/non-root.
			// "remoteUser": "vscode"
		}
		`

		spec, err := LoadSpec(&LoadSpecOpts{
			Data: []byte(data),
		})
		assert.Nil(t, err)
		zap.L().Debug("Spec", zap.Any("spec", spec))
		assert.NotNil(t, spec.PostCreateCommand)
		assert.Equal(t, 1, len(spec.PostCreateCommand.Commands))
		assert.Equal(t, "pip3 install --user -r requirements.txt", spec.PostCreateCommand.Commands[0])
	}

}

func TestCommand(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	type wrp struct {
		Cmd *SpecCommand `json:"cmd"`
	}

	{
		wrp := &wrp{}
		err = json.Unmarshal([]byte(`{"cmd": "val-1"}`), wrp)
		assert.Nil(t, err)
		assert.Equal(t, "val-1", wrp.Cmd.Commands[0])
	}

	{
		wrp := &wrp{}
		err = json.Unmarshal([]byte(`{"cmd": ["ls", "-la"]}`), wrp)
		assert.Nil(t, err)
		assert.Equal(t, "ls -la", wrp.Cmd.Commands[0])
	}

	{
		wrp := &wrp{}
		err = json.Unmarshal([]byte(`
{"cmd": {
  "cmd1": "ls",
	"cmd2": ["ls", "-la"]
}
}`), wrp)
		assert.Nil(t, err)
		assert.True(t, slices.Contains(wrp.Cmd.Commands, "ls"))
		assert.True(t, slices.Contains(wrp.Cmd.Commands, "ls -la"))
	}

}

func TestSpecCommandVariants(t *testing.T) {
	for _, tc := range []struct {
		name    string
		input   string
		want    []string
		invalid bool
	}{
		{name: "shell command", input: `"echo ready"`, want: []string{"echo ready"}},
		{name: "simple argv", input: `["echo","ready"]`, want: []string{"echo ready"}},
		{name: "parallel commands", input: `{"first":"echo one","second":["echo","two"]}`, want: []string{"echo one", "echo two"}},
		{name: "number", input: `42`, invalid: true},
		{name: "boolean", input: `true`, invalid: true},
		{name: "mixed argv", input: `["echo",42]`, invalid: true},
		{name: "invalid parallel command", input: `{"first":false}`, invalid: true},
		{name: "invalid nested argv", input: `{"first":["echo",null]}`, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := &SpecCommand{}
			err := json.Unmarshal([]byte(tc.input), command)
			if tc.invalid {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.ElementsMatch(t, tc.want, command.Commands)
			}
		})
	}
}

func TestSpecCommandPreservesArgv(t *testing.T) {
	for _, arg := range []string{"a b", "$CORDIUM_UNDEFINED_VARIABLE", "semi;printf injected", "quote'and\"double"} {
		t.Run(arg, func(t *testing.T) {
			data, err := json.Marshal([]string{"printf", "%s", arg})
			if err != nil {
				t.Fatal(err)
			}
			command := &SpecCommand{}
			if err := json.Unmarshal(data, command); err != nil {
				t.Fatal(err)
			}
			require.Len(t, command.Commands, 1)
			out, err := exec.CommandContext(t.Context(), "sh", "-c", command.Commands[0]).CombinedOutput()
			assert.Nil(t, err)
			assert.Equal(t, arg, string(out))
		})
	}
}

func TestLoadSpecJSONAndSubstitution(t *testing.T) {
	for _, value := range []string{"plain", "with space", `quote"value`, `path\value`, "line\nbreak"} {
		t.Run(value, func(t *testing.T) {
			spec, err := LoadSpec(&LoadSpecOpts{Data: []byte(`{"name":"${containerEnv:NAME}","image":"alpine"}`), ContainerEnv: []string{"NAME=" + value}})
			if assert.Nil(t, err) {
				assert.Equal(t, value, spec.Name)
			}
		})
	}
	_, err := LoadSpec(&LoadSpecOpts{Data: []byte(`{"name":42}`)})
	assert.NotNil(t, err)
}
