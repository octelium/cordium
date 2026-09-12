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
	"os"
	"strings"

	"github.com/octelium/octelium/cluster/e2e/suite"
)

func Phases() []suite.Phase {
	return []suite.Phase{
		{Name: "CordiumAPI", Run: testCordiumAPI},
		{Name: "CordiumClusterConfig", Run: testCordiumClusterConfig},

		{Name: "Space", Run: testSpace},
		{Name: "SpaceMembership", Run: testSpaceMembership},
		{Name: "Template", Run: testTemplate},
		{Name: "Secret", Run: testSecret},
		{Name: "UserSecret", Run: testUserSecret},

		{Name: "WorkspaceAPI", Run: testWorkspaceAPI},
		{Name: "WorkspaceValidation", Run: testWorkspaceValidation},
		{Name: "WorkspacePorts", Run: testWorkspacePorts},

		{Name: "WorkspaceLifecycle", Run: testWorkspaceLifecycle},
		{Name: "WorkspaceStopWhileStarting", Run: testWorkspaceStopWhileStarting},
		{Name: "WorkspaceRuntime", Run: testWorkspaceRuntime},
		{Name: "WorkspaceTemplateMerge", Run: testWorkspaceTemplateMerge},
		{Name: "WorkspaceTasks", Run: testWorkspaceTasks},
		{Name: "WorkspaceAutoStop", Run: testWorkspaceAutoStop},
		{Name: "WorkspaceImage", Run: testWorkspaceImage},
		{Name: "WorkspaceEphemeral", Run: testWorkspaceEphemeral},
		{Name: "WorkspaceIsolation", Run: testWorkspaceIsolation},

		{Name: "CordiumCLI", Run: testCordiumCLI},
		{Name: "CordiumCLIWorkspace", Run: testCordiumCLIWorkspace},
	}
}

func TailPhases() []suite.Phase {
	return []suite.Phase{
		{Name: "CordiumComponentHealth", Run: testCordiumComponentHealth},
	}
}

func ReadyPhase() suite.Phase {
	return suite.Phase{Name: "CordiumReady", Run: testCordiumReady}
}

const EnvSuite = "OCTELIUM_E2E_SUITE"

const (
	SuiteCordium = "cordium"
	SuiteAll     = "all"
)

func All() []suite.Phase {
	if strings.EqualFold(os.Getenv(EnvSuite), SuiteAll) {
		return withCore()
	}

	return Cordium()
}

func Cordium() []suite.Phase {
	ret := suite.Only(suite.Phases(), "ClusterReady")
	ret = append(ret, ReadyPhase())
	ret = append(ret, Phases()...)
	ret = append(ret, suite.Only(suite.Phases(), "ComponentHealth")...)

	return append(ret, TailPhases()...)
}

func withCore() []suite.Phase {
	ret := suite.InsertAfter(suite.Phases(), "ClusterReady", ReadyPhase())
	ret = suite.InsertBefore(ret, "Apply", Phases()...)

	return append(ret, TailPhases()...)
}
