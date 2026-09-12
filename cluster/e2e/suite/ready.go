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
	"testing"

	charness "github.com/octelium/cordium/cluster/e2e/harness"
	cscenario "github.com/octelium/cordium/cluster/e2e/scenario"
	"github.com/octelium/octelium/cluster/e2e/harness"
)

func testCordiumReady(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	for _, name := range cscenario.Deployments {
		h.MustWaitDeployment(t, name)
	}

	h.StartCordiumLogStreams(t, cscenario.Components...)

	h.PrintClusterDiagnostics(t)

	h.MustRun(t, "kubectl get pods -n octelium")
	h.MustRun(t, "kubectl get pods -n "+charness.WorkspaceNamespace)
	h.MustRun(t, "kubectl get pvc -A")
}

func testCordiumComponentHealth(t *testing.T, ch *harness.H) {
	h := charness.Wrap(ch)

	for _, component := range cscenario.Components {
		h.CheckCordiumRestarts(t, component)
	}
}
