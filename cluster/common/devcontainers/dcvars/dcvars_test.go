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
