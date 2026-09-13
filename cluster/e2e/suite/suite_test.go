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

package suite

import (
	"slices"
	"testing"

	"github.com/octelium/octelium/cluster/e2e/suite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func names(phases []suite.Phase) []string {
	ret := make([]string, 0, len(phases))
	for _, p := range phases {
		ret = append(ret, p.Name)
	}

	return ret
}

func TestCordiumPhases(t *testing.T) {
	phases := Cordium()
	require.Nil(t, suite.Validate(phases))

	got := names(phases)
	assert.Equal(t, "ClusterReady", got[0])
	assert.Equal(t, "CordiumReady", got[1])

	for _, name := range names(Phases()) {
		assert.Contains(t, got, name)
	}

	for _, name := range names(TailPhases()) {
		assert.Contains(t, got, name)
	}

	assert.Less(t, slices.Index(got, "WorkspaceAPI"), slices.Index(got, "WorkspaceLifecycle"))
	assert.Less(t, slices.Index(got, "WorkspaceValidation"),
		slices.Index(got, "WorkspaceNetworkValidation"))
	assert.Less(t, slices.Index(got, "WorkspaceNetworkValidation"),
		slices.Index(got, "WorkspaceLifecycle"))
	assert.Less(t, slices.Index(got, "WorkspaceSandbox"),
		slices.Index(got, "WorkspaceNetwork"))
	assert.Less(t, slices.Index(got, "WorkspaceNetwork"),
		slices.Index(got, "WorkspaceNetworkPolicy"))
	assert.Less(t, slices.Index(got, "WorkspaceTasks"),
		slices.Index(got, "WorkspaceTaskFailure"))
	assert.Less(t, slices.Index(got, "WorkspaceEphemeral"),
		slices.Index(got, "WorkspaceStorage"))
	assert.Less(t, slices.Index(got, "WorkspaceLifecycle"),
		slices.Index(got, "WorkspaceIsolation"))
	assert.Less(t, slices.Index(got, "WorkspaceIsolation"),
		slices.Index(got, "CordiumCLIWorkspace"))
	assert.Less(t, slices.Index(got, "CordiumCLIWorkspace"),
		slices.Index(got, "CordiumComponentHealth"))
}

func TestWithCorePhases(t *testing.T) {
	t.Setenv(EnvSuite, SuiteAll)

	phases := All()
	require.Nil(t, suite.Validate(phases))

	got := names(phases)
	for _, name := range names(suite.Phases()) {
		assert.Contains(t, got, name)
	}

	for _, name := range names(Phases()) {
		assert.Contains(t, got, name)
	}

	assert.Less(t, slices.Index(got, "CordiumReady"), slices.Index(got, "CordiumAPI"))
	assert.Less(t, slices.Index(got, "WorkspaceLifecycle"), slices.Index(got, "Apply"))
}

func TestCordiumSuiteIsTheDefault(t *testing.T) {
	require.Nil(t, suite.Validate(All()))

	assert.Equal(t, names(Cordium()), names(All()))
}
