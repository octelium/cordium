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
	"os"
	"path/filepath"
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

func TestLimitedReaderExactLimit(t *testing.T) {
	for _, size := range []int{0, 8} {
		r := &limitedReader{r: bytes.NewReader(make([]byte, size)), n: int64(size)}
		out, err := io.ReadAll(r)
		assert.Nil(t, err)
		assert.Len(t, out, size)
	}
}

func TestFeatureDependencyOrdering(t *testing.T) {
	features := []*Feature{
		{Name: "app", Spec: &Spec{InstallsAfter: []string{"left:1", "right:1", "missing:1"}}},
		{Name: "left", Spec: &Spec{InstallsAfter: []string{"base:1"}}},
		{Name: "right", Spec: &Spec{InstallsAfter: []string{"base:1"}}},
		{Name: "base", Spec: &Spec{}},
	}
	var sorted []*Feature
	for _, feature := range features {
		doAddFeature(features, &sorted, feature, make(map[string]struct{}))
	}
	var names []string
	for _, feature := range sorted {
		names = append(names, feature.Name)
	}
	assert.Equal(t, []string{"base", "left", "right", "app"}, names)
}

func TestFeatureDependencyRegistryPort(t *testing.T) {
	features := []*Feature{
		{Name: "app", Spec: &Spec{InstallsAfter: []string{"registry.example.com:5000/team/base:1"}}},
		{Name: "registry.example.com:5000/team/base", Spec: &Spec{}},
	}
	var sorted []*Feature
	for _, feature := range features {
		doAddFeature(features, &sorted, feature, make(map[string]struct{}))
	}
	assert.Equal(t, features[1].Name, sorted[0].Name)
	assert.Equal(t, "app", sorted[1].Name)
}

func TestGetSortedFeaturesLocalFiles(t *testing.T) {
	base := t.TempDir()
	for name, spec := range map[string]string{
		"app":  `{"id":"app","installsAfter":["base:1"]}`,
		"base": `{"id":"base","name":"Base feature"}`,
	} {
		dir := filepath.Join(base, name)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "devcontainer-feature.json"), []byte(spec), 0600); err != nil {
			t.Fatal(err)
		}
	}
	features, err := GetSortedFeatures(&GetSortedFeaturesOpts{BasePath: base})
	if !assert.Nil(t, err) {
		return
	}
	assert.Len(t, features, 2)
	assert.Equal(t, "base", features[0].Name)
	assert.Equal(t, filepath.Join(base, "base"), features[0].Path)
	assert.Equal(t, "app", features[1].Name)
	_, err = GetSortedFeatures(&GetSortedFeaturesOpts{BasePath: filepath.Join(base, "missing")})
	assert.NotNil(t, err)
	assert.Nil(t, DownloadFeatures(t.Context(), &GetFeaturesOpts{DirBase: base}))
	assert.NotNil(t, DownloadFeatures(t.Context(), nil))
}
