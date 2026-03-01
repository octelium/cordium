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
