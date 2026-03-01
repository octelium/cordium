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

package supervisor

/*
import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/stretchr/testify/assert"
)

func TestDockerCompose(t *testing.T) {

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	spec := &DockerCompose{}

	data := `
version: '3'
services:
  # Some comment
  app:
    build:
      context: .
      dockerfile: Dockerfile
    volumes:
      - ../..:/workspaces:cached
    environment:
      RAILS_ENV: development
      NODE_ENV: development
  db:
    image: postgres
    environment:
      ARG1: VAL1
`
	err = yaml.Unmarshal([]byte(data), spec)
	assert.Nil(t, err, "%s", err)
	assert.Equal(t, spec.Services["app"].Build.Dockerfile, "Dockerfile")
}
*/
