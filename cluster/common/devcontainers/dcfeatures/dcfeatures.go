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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/codeclysm/extract/v4"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const (
	maxFeatureDepth   = 16
	maxFeatureArchive = 512 * 1024 * 1024
)

type GetFeaturesOpts struct {
	FeaturesMap  map[string]any
	DirBase      string
	ContainerEnv []string
	Workspace    *cordiumv1.Workspace
}

func DownloadFeatures(ctx context.Context, o *GetFeaturesOpts) error {

	if o == nil || o.DirBase == "" {
		return errors.Errorf("Could not install devcontainers features. Invalid opts")
	}

	featuresMap := make(map[string]any)

	if len(o.FeaturesMap) > 0 {
		for k, v := range o.FeaturesMap {
			featuresMap[k] = v
		}
	}

	if o.Workspace != nil &&
		o.Workspace.Spec.Runtime != nil &&
		o.Workspace.Spec.Runtime.Devcontainers != nil &&
		len(o.Workspace.Spec.Runtime.Devcontainers.Features) > 0 {
		for _, ftr := range o.Workspace.Spec.Runtime.Devcontainers.Features {

			optMap := make(map[string]string)

			for _, opt := range ftr.Options {
				if opt.Key != "" && opt.Value != "" {
					optMap[opt.Key] = opt.Value
				}
			}

			featuresMap[ftr.Reference] = optMap
		}
	}

	if len(featuresMap) == 0 {
		return nil
	}

	if err := os.MkdirAll(o.DirBase, 0755); err != nil {
		return err
	}

	zap.L().Debug("Downloading devcontainers features", zap.Any("featureMap", featuresMap))

	visited := make(map[string]struct{})

	for ftr, _ := range featuresMap {
		if err := getFeature(ctx, ftr, o, visited, 0); err != nil {
			return err
		}
	}

	return nil
}

func getFeature(ctx context.Context, featureURL string, o *GetFeaturesOpts,
	visited map[string]struct{}, depth int) error {
	if depth > maxFeatureDepth {
		return errors.Errorf("Devcontainer feature dependencies are too deep: %s", featureURL)
	}

	featureURL = strings.TrimSpace(featureURL)
	ref, err := name.ParseReference(featureURL)
	if err != nil {
		return errors.Errorf("Could not parse devcontainer feature ref: %+v", err)
	}

	if _, ok := visited[ref.Name()]; ok {
		zap.L().Debug("Devcontainer feature has already been fetched. Skipping",
			zap.String("name", ref.Name()))
		return nil
	}
	visited[ref.Name()] = struct{}{}

	dirBase := o.DirBase

	zap.L().Debug("Starting getting feature",
		zap.String("name", ref.Name()),
		zap.String("id", ref.Identifier()), zap.String("reg", ref.Context().RegistryStr()),
		zap.String("repo", ref.Context().RepositoryStr()),
	)

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	manifest, err := img.Manifest()
	if err != nil {
		return errors.Errorf("Could not get manifest: %+v", err)
	}

	if manifest.Config.MediaType != "application/vnd.devcontainers" {
		return errors.Errorf("Invalid media type")
	} else if len(manifest.Layers) == 0 {
		return errors.Errorf("No layers found")
	} else if manifest.Layers[0].Size > maxFeatureArchive {
		return errors.Errorf("Devcontainer feature layer is too large: %d", manifest.Layers[0].Size)
	}

	layer, err := img.LayerByDigest(manifest.Layers[0].Digest)
	if err != nil {
		return err
	}

	data, err := layer.Uncompressed()
	if err != nil {
		return err
	}
	defer data.Close()

	dirPath := filepath.Join(dirBase, ref.Context().RegistryStr(), ref.Context().RepositoryStr())

	os.RemoveAll(dirPath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return err
	}

	zap.L().Debug("Extracting feature from archive", zap.String("path", dirPath))

	if err := extract.Archive(ctx, &limitedReader{r: data, n: maxFeatureArchive}, dirPath, nil); err != nil {
		return errors.Errorf("Could not extract archive: %+v", err)
	}

	featureJSON, err := os.ReadFile(filepath.Join(dirPath, "devcontainer-feature.json"))
	if err != nil {
		return errors.Errorf("Could not read devcontainer-feature.json: %+v", err)
	}

	spec, err := LoadSpec(&LoadSpecOpts{
		Data:         featureJSON,
		ContainerEnv: o.ContainerEnv,
		Workspace:    o.Workspace,
	})
	if err != nil {
		return err
	}

	zap.L().Debug("Features spec", zap.Any("spec", spec))

	for _, installAfter := range spec.InstallsAfter {
		if err := getFeature(ctx, installAfter, o, visited, depth+1); err != nil {
			return err
		}
	}

	return nil
}

type limitedReader struct {
	r io.Reader
	n int64
}

func (l *limitedReader) Read(p []byte) (int, error) {
	if l.n <= 0 {
		return 0, errors.Errorf("Devcontainer feature archive is too large")
	}
	if int64(len(p)) > l.n {
		p = p[0:l.n]
	}
	n, err := l.r.Read(p)
	l.n = l.n - int64(n)
	return n, err
}

type Feature struct {
	Name string
	Path string
	Spec *Spec
}

type GetSortedFeaturesOpts struct {
	BasePath     string
	ContainerEnv []string
	Workspace    *cordiumv1.Workspace
}

func GetSortedFeatures(o *GetSortedFeaturesOpts) ([]*Feature, error) {
	var features []*Feature
	basePath := o.BasePath

	zap.L().Debug("Sorting devcontainers features", zap.String("basePath", basePath))

	if err := filepath.Walk(basePath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if !info.Mode().IsRegular() {
				return nil
			}

			if info.Name() == "devcontainer-feature.json" {
				featureJSON, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				spec, err := LoadSpec(&LoadSpecOpts{
					Data:         featureJSON,
					ContainerEnv: o.ContainerEnv,
					Workspace:    o.Workspace,
				})
				if err != nil {
					return err
				}

				featurePath := filepath.Dir(path)

				features = append(features, &Feature{
					Path: featurePath,
					Name: strings.TrimPrefix(featurePath, fmt.Sprintf("%s/", basePath)),
					Spec: spec,
				})

			}

			return nil
		}); err != nil {
		return nil, err
	}

	var sortedFeatures []*Feature

	for _, ftr := range features {
		doAddFeature(features, &sortedFeatures, ftr, make(map[string]struct{}))
	}

	zap.L().Debug("Sorted features", zap.Any("features", sortedFeatures))

	return sortedFeatures, nil
}

func isInList(lst []*Feature, name string) bool {
	for _, ftr := range lst {
		if ftr.Name == name {
			return true
		}
	}
	return false
}

func getByName(lst []*Feature, name string) *Feature {

	for _, ftr := range lst {
		if ftr.Name == name {
			return ftr
		}
	}

	return nil
}

func doAddFeature(features []*Feature, sortedFeatures *[]*Feature, ftr *Feature,
	visiting map[string]struct{}) {
	if _, ok := visiting[ftr.Name]; ok {
		zap.L().Warn("Skipping a cyclic devcontainer feature dependency",
			zap.String("name", ftr.Name))
		return
	}
	visiting[ftr.Name] = struct{}{}
	defer delete(visiting, ftr.Name)

	for _, dep := range ftr.Spec.InstallsAfter {
		args := strings.Split(dep, ":")
		if len(args) < 1 {
			continue
		}
		name := args[0]
		if !isInList(*sortedFeatures, name) {
			if ftr := getByName(features, name); ftr != nil {
				doAddFeature(features, sortedFeatures, ftr, visiting)
			}
		}
	}

	if !isInList(*sortedFeatures, ftr.Name) {
		*sortedFeatures = append(*sortedFeatures, ftr)
	}
}
