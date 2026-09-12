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

package scenario

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/octelium/octelium/cluster/e2e/scenario"
	"go.uber.org/zap"
)

const (
	DefaultScenario  = "k3s-flannel-cordium"
	CoreMainScenario = "k3s-flannel-cordium-core-main"
)

const Package = "cordium"

const NodeLabel = "octelium.com/node-mode-cordium"

const (
	EnvCoreVersion    = "OCTELIUM_E2E_CORE_VERSION"
	EnvPackageVersion = "OCTELIUM_E2E_PACKAGE_VERSION"
)

const VersionLatest = "latest"

var packageVersions = map[string]string{}

func init() {
	register(DefaultScenario, opts{
		description: "Single-node k3s with flannel and the Cordium components",
	})

	register(CoreMainScenario, opts{
		description: "The Cordium scenario against the core main branch images",
		coreVersion: "main",
	})
}

type opts struct {
	description string
	coreVersion string
	budget      time.Duration
}

func register(id string, o opts) {
	packageVersions[id] = packageVersion()

	scenario.Register(id, func() *scenario.Scenario {
		ret := scenario.MustGet("k3s-flannel")

		ret.Description = o.description
		ret.Install.Version = coreVersion(o.coreVersion)
		ret.Topology.Labels = append(ret.Topology.Labels, NodeLabel)

		ret.Hooks.PostInstall = []scenario.Step{
			{Name: "cordium/install-package", Run: stepInstallPackage},
			{Name: "cordium/readiness", Run: stepWaitDeployments},
		}

		ret.Budget = cmpOr(o.budget, 120*time.Minute)

		return ret
	})
}

func PackageVersionFor(id string) string {
	return displayVersion(packageVersions[id])
}

func coreVersion(def string) string {
	if val := os.Getenv(EnvCoreVersion); val != "" {
		return normalizeVersion(val)
	}

	return normalizeVersion(def)
}

func packageVersion() string {
	if val := os.Getenv(EnvPackageVersion); val != "" {
		return normalizeVersion(val)
	}

	return normalizeVersion(os.Getenv("GITHUB_REF_NAME"))
}

func normalizeVersion(arg string) string {
	if arg == VersionLatest {
		return ""
	}

	return arg
}

func cmpOr[T comparable](vals ...T) T {
	var zero T
	for _, val := range vals {
		if val != zero {
			return val
		}
	}
	return zero
}

func displayVersion(arg string) string {
	return cmpOr(arg, VersionLatest)
}

var Components = []string{
	"nocturne",
	"rscserver",
}

var Deployments = []string{
	"cordium-nocturne",
	"cordium-rscserver",

	"svc-default-cordium-octelium-api",
	"svc-default-cordium",
	"svc-default-ssh-cordium",
}

func stepInstallPackage(ctx context.Context, r *scenario.Runner) error {
	version := packageVersions[r.Scenario.ID]

	zap.L().Info("Installing the Cordium package",
		zap.String("package", Package),
		zap.String("packageVersion", displayVersion(version)),
		zap.String("coreVersion", displayVersion(r.Scenario.Install.Version)))

	versionArg := ""
	if version != "" {
		versionArg = fmt.Sprintf(" --version %s", version)
	}

	if err := r.Bash(ctx, fmt.Sprintf("octops install-package %s --package %s --kubeconfig %s%s",
		r.Scenario.Domain, Package, r.State.KubeconfigPath, versionArg)); err != nil {
		return err
	}

	zap.L().Info(
		"The package installation has been started. It runs asynchronously in the Cluster, " +
			"so the next step waits for its genesis Job and Deployments")

	return nil
}
