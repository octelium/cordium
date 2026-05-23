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
