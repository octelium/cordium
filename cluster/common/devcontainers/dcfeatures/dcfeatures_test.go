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

package dcfeatures

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
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
	}, make(map[string]struct{}), 0)
	assert.Nil(t, err)

	features, err := GetSortedFeatures(&GetSortedFeaturesOpts{
		BasePath: "/tmp/tdc01",
	})
	assert.Nil(t, err)
	for _, ftr := range features {
		zap.L().Debug("FEA", zap.Any("ftr", ftr))
	}
}

func TestGetFeatureLimits(t *testing.T) {
	ctx := context.Background()

	featureURL := "ghcr.io/devcontainers/features/aws-cli:1"

	{
		err := getFeature(ctx, featureURL, &GetFeaturesOpts{
			DirBase: "/tmp/tdc02",
		}, make(map[string]struct{}), maxFeatureDepth+1)
		assert.NotNil(t, err)
	}

	{
		ref, err := name.ParseReference(featureURL)
		assert.Nil(t, err)

		visited := map[string]struct{}{
			ref.Name(): {},
		}

		err = getFeature(ctx, featureURL, &GetFeaturesOpts{
			DirBase: "/tmp/tdc02",
		}, visited, 0)
		assert.Nil(t, err)
	}
}

func TestDoAddFeatureCycle(t *testing.T) {

	features := []*Feature{
		{
			Name: "a",
			Spec: &Spec{
				InstallsAfter: []string{"b"},
			},
		},
		{
			Name: "b",
			Spec: &Spec{
				InstallsAfter: []string{"a"},
			},
		},
		{
			Name: "c",
			Spec: &Spec{
				InstallsAfter: []string{"c"},
			},
		},
	}

	var sortedFeatures []*Feature
	for _, ftr := range features {
		doAddFeature(features, &sortedFeatures, ftr, make(map[string]struct{}))
	}

	assert.Equal(t, len(features), len(sortedFeatures))
}

func TestLimitedReader(t *testing.T) {

	{
		r := &limitedReader{r: bytes.NewReader(make([]byte, 8)), n: 32}
		out, err := io.ReadAll(r)
		assert.Nil(t, err)
		assert.Equal(t, 8, len(out))
	}

	{
		r := &limitedReader{r: bytes.NewReader(make([]byte, 64)), n: 32}
		out, err := io.ReadAll(r)
		assert.NotNil(t, err)
		assert.Equal(t, 32, len(out))
	}
}
