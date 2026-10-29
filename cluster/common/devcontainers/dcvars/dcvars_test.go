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

package dcvars

import (
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/apis/main/metav1"
	"github.com/stretchr/testify/assert"
)

func TestDevContainers(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	{

		in := `
{
	"path": "${PATH}"
}
`

		out := `
{
	"path": "/usr/custom/bin"
}
`

		res, err := SubstituteVars(&SubstituteVarsOpts{
			Input: in,
			ContainerEnv: []string{
				"PATH=/usr/custom/bin",
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, out, res)
	}

}

func TestSubstituteVarsCases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		env   []string
		want  string
	}{
		{name: "plain input", input: `{"image":"alpine"}`, want: `{"image":"alpine"}`},
		{name: "repeated variable", input: "${PATH}:${PATH}", env: []string{"PATH=/bin"}, want: "/bin:/bin"},
		{name: "container environment", input: "${containerEnv:EDITOR}", env: []string{"EDITOR=vim"}, want: "vim"},
		{name: "local environment", input: "before${localEnv:PRIVATE}after", want: "beforeafter"},
		{name: "unknown variable", input: "${unknown}", want: "${unknown}"},
		{name: "workspace folder", input: "${localWorkspaceFolder}", want: "${localWorkspaceFolder}"},
		{name: "unterminated variable", input: "${containerEnv:EDITOR", want: "${containerEnv:EDITOR"},
		{name: "distinct variables", input: "${containerEnv:FIRST}/${containerEnv:SECOND}", env: []string{"FIRST=one", "SECOND=two"}, want: "one/two"},
		{name: "environment contains equals", input: "${containerEnv:TOKEN}", env: []string{"TOKEN=a=b=c"}, want: "a=b=c"},
		{name: "missing environment", input: "before${containerEnv:MISSING}after", want: "beforeafter"},
		{name: "empty environment", input: "before${containerEnv:EMPTY}after", env: []string{"EMPTY="}, want: "beforeafter"},
		{name: "last environment value wins", input: "${containerEnv:EDITOR}", env: []string{"EDITOR=vi", "EDITOR=nano"}, want: "nano"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SubstituteVars(&SubstituteVarsOpts{Input: tc.input, ContainerEnv: tc.env})
			assert.Nil(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSubstituteVarsWorkspaceID(t *testing.T) {
	got, err := SubstituteVars(&SubstituteVarsOpts{
		Input:     "${devcontainerId}",
		Workspace: &cordiumv1.Workspace{Metadata: &metav1.Metadata{Uid: "workspace-uid"}},
	})
	assert.Nil(t, err)
	assert.Equal(t, "workspace-uid", got)
}
