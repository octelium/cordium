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

package dcfeatures

import (
	"context"
	"testing"

	"github.com/octelium/cordium/cluster/common/tests"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGetFeature(t *testing.T) {
	ctx := context.Background()

	tst, err := tests.Initialize(nil)
	assert.Nil(t, err, "%+v", err)
	t.Cleanup(func() {
		tst.Destroy()
	})

	err = getFeature(ctx, "ghcr.io/devcontainers/features/aws-cli:1", &GetFeaturesOpts{
		DirBase: "/tmp/tdc01",
	})
	assert.Nil(t, err)

	features, err := GetSortedFeatures(&GetSortedFeaturesOpts{
		BasePath: "/tmp/tdc01",
	})
	assert.Nil(t, err)
	for _, ftr := range features {
		zap.L().Debug("FEA", zap.Any("ftr", ftr))
	}
}
