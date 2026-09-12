//go:build e2e

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

package tests

import (
	"os"
	"testing"

	csuite "github.com/octelium/cordium/cluster/e2e/suite"
	"github.com/octelium/octelium/cluster/e2e/suite"
)

func TestMain(m *testing.M) {
	os.Exit(suite.Bootstrap(m))
}

func TestE2E(t *testing.T) {
	suite.Run(t, csuite.All())
}
