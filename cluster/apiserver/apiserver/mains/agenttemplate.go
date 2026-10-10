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

package mains

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"github.com/octelium/octelium/cluster/common/apivalidation"
	"github.com/octelium/octelium/cluster/common/grpcutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
)

const (
	defaultAgentVersion = "dev"
	agentPackage        = "@octelium/cordium-agent"
	agentConfigEnvVar   = "CORDIUM_AGENT_CONFIG_JSON"

	agentApplicationName = "agent"
	agentApplicationPort = 8080

	maxAgentConfigBytes     = 64 * 1024
	maxAgentModelLen        = 256
	maxAgentImageURLLen     = 1024
	maxAgentInlineImageLen  = 64 * 1024
	maxAgentLimitMillicores = 1000000
	maxAgentLimitMegabytes  = 10000000
)

var rgxAgentVersion = regexp.MustCompile(`^[0-9A-Za-z][0-9A-Za-z._+-]{0,127}$`)

const agentInstallScript = `set -e
if command -v apt-get >/dev/null 2>&1 && ! command -v rg >/dev/null 2>&1; then
  (apt-get update && apt-get install -y --no-install-recommends ripgrep fd-find git curl ca-certificates) >/dev/null 2>&1 || true
fi
if command -v node >/dev/null 2>&1 && command -v npx >/dev/null 2>&1 && node -e 'const [major, minor] = process.versions.node.split(".").map(Number); process.exit(major > 22 || (major === 22 && minor >= 19) ? 0 : 1)'; then
  exit 0
fi
case "$(uname -m)" in
  x86_64) arch=x64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) echo "Unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac
base=https://nodejs.org/dist/latest-v24.x
dir=$(mktemp -d)
trap 'rm -rf "$dir"' EXIT
cd "$dir"
curl -fsSLO "$base/SHASUMS256.txt"
file=$(grep -o "node-v[0-9.]*-linux-$arch.tar.gz" SHASUMS256.txt | head -n 1)
curl -fsSLO "$base/$file"
grep " $file\$" SHASUMS256.txt | sha256sum -c -
tar -xzf "$file" -C /usr/local --strip-components=1
`

func getAgentVersion(cfg *cordiumv1.ClusterConfig_Spec_Agent) (string, error) {
	ret := "latest"

	if ldflags.IsDev() {
		ret = cfg.GetVersion()
		if ret == "" {
			ret = ldflags.GitBranch
		}
		if ret == "" {
			ret = defaultAgentVersion
		}
	} else if cfg.GetVersion() != "" {
		ret = cfg.GetVersion()
	}

	if !rgxAgentVersion.MatchString(ret) {
		return "", grpcutils.InvalidArg("Invalid agent version: %s", ret)
	}

	return ret, nil
}

func getAgentTemplateSpec(cfg *cordiumv1.ClusterConfig_Spec_Agent) (*cordiumv1.Template_Spec, error) {
	if err := ValidateAgentConfig(cfg); err != nil {
		return nil, err
	}

	version, err := getAgentVersion(cfg)
	if err != nil {
		return nil, err
	}

	agentConfig, err := getAgentConfig(cfg)
	if err != nil {
		return nil, err
	}

	ret := &cordiumv1.Template_Spec{
		Image: cfg.GetImage(),
		Limit: cfg.GetLimit(),
		Runtime: &cordiumv1.Workspace_Spec_Runtime{
			Tasks: []*cordiumv1.Workspace_Spec_Runtime_Task{
				{
					Name:      "install-cordium-agent-deps",
					Type:      cordiumv1.Workspace_Spec_Runtime_Task_ON_CREATE,
					Run:       agentInstallScript,
					RunAsRoot: true,
					OnFailure: cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_CONTINUE,
				},
				{
					Name:         "cordium-agent",
					Type:         cordiumv1.Workspace_Spec_Runtime_Task_POST_START,
					Run:          fmt.Sprintf("exec npx --yes --prefer-online %s@%s serve", agentPackage, version),
					IsBackground: true,
					OnFailure:    cordiumv1.Workspace_Spec_Runtime_Task_ON_FAILURE_CONTINUE,
				},
			},
		},
	}

	if agentConfig != "" {
		ret.Runtime.EnvVars = []*cordiumv1.Workspace_Spec_Runtime_EnvVar{
			{
				Key: agentConfigEnvVar,
				Type: &cordiumv1.Workspace_Spec_Runtime_EnvVar_Value{
					Value: agentConfig,
				},
			},
		}
	}

	return ret, nil
}

func getAgentConfig(cfg *cordiumv1.ClusterConfig_Spec_Agent) (string, error) {
	ret := make(map[string]any)

	if llm := cfg.GetLlm(); llm.GetService() != "" || llm.GetModel() != "" {
		llmCfg := map[string]any{
			"provider": "octelium",
		}
		if llm.Service != "" {
			llmCfg["service"] = llm.Service
		}
		if llm.Model != "" {
			llmCfg["model"] = llm.Model
		}
		ret["llm"] = llmCfg
	}

	if cfg.GetConfig() != nil {
		mergeAgentConfig(ret, cfg.Config.AsMap())
	}

	if len(ret) == 0 {
		return "", nil
	}

	out, err := json.Marshal(ret)
	if err != nil {
		return "", grpcutils.InvalidArg("Could not marshal the agent config: %+v", err)
	}

	if len(out) > maxAgentConfigBytes {
		return "", grpcutils.InvalidArg("The agent config is too large")
	}

	return string(out), nil
}

func mergeAgentConfig(dst map[string]any, src map[string]any) {
	for k, v := range src {
		srcMap, srcOK := v.(map[string]any)
		dstMap, dstOK := dst[k].(map[string]any)
		if srcOK && dstOK {
			mergeAgentConfig(dstMap, srcMap)
			continue
		}
		dst[k] = v
	}
}

func ValidateAgentConfig(cfg *cordiumv1.ClusterConfig_Spec_Agent) error {
	if cfg == nil {
		return nil
	}

	if cfg.Version != "" && !rgxAgentVersion.MatchString(cfg.Version) {
		return grpcutils.InvalidArg("Invalid agent version: %s", cfg.Version)
	}

	if llm := cfg.Llm; llm != nil {
		if llm.Service != "" {
			if err := apivalidation.ValidateName(llm.Service, 0, 1); err != nil {
				return grpcutils.InvalidArg("Invalid agent LLM Service name: %s", llm.Service)
			}
		}

		if len(llm.Model) > maxAgentModelLen || !utf8.ValidString(llm.Model) ||
			strings.ContainsFunc(llm.Model, unicode.IsControl) {
			return grpcutils.InvalidArg("Invalid agent LLM model")
		}
	}

	if err := validateAgentImage(cfg.Image); err != nil {
		return err
	}

	if err := validateAgentLimit(cfg.Limit); err != nil {
		return err
	}

	if cfg.Config != nil {
		out, err := json.Marshal(cfg.Config.AsMap())
		if err != nil {
			return grpcutils.InvalidArg("Invalid agent config")
		}
		if len(out) > maxAgentConfigBytes {
			return grpcutils.InvalidArg("The agent config is too large")
		}
	}

	return nil
}

func validateAgentImage(img *cordiumv1.Workspace_Spec_Image) error {
	if img == nil {
		return nil
	}

	switch img.Type.(type) {
	case *cordiumv1.Workspace_Spec_Image_Registry_:
		registry := img.GetRegistry()
		if registry.Url == "" || len(registry.Url) > maxAgentImageURLLen ||
			strings.ContainsFunc(registry.Url, unicode.IsSpace) {
			return grpcutils.InvalidArg("Invalid agent image registry URL")
		}
		if registry.Authentication != nil {
			return grpcutils.InvalidArg("Agent image registry authentication is not supported")
		}
	case *cordiumv1.Workspace_Spec_Image_Dockerfile_:
		dockerfile := img.GetDockerfile()
		switch dockerfile.Type.(type) {
		case *cordiumv1.Workspace_Spec_Image_Dockerfile_Inline:
			if dockerfile.GetInline() == "" || len(dockerfile.GetInline()) > maxAgentInlineImageLen {
				return grpcutils.InvalidArg("Invalid agent image inline Dockerfile")
			}
		case *cordiumv1.Workspace_Spec_Image_Dockerfile_Url:
			if err := validateAgentHTTPSURL(dockerfile.GetUrl()); err != nil {
				return err
			}
		default:
			return grpcutils.InvalidArg("The agent image Dockerfile must be set")
		}
	case *cordiumv1.Workspace_Spec_Image_Git_:
		if err := validateAgentHTTPSURL(img.GetGit().Url); err != nil {
			return err
		}
	default:
		return grpcutils.InvalidArg("Unsupported agent image type")
	}

	return nil
}

func validateAgentHTTPSURL(arg string) error {
	if arg == "" || len(arg) > maxAgentImageURLLen {
		return grpcutils.InvalidArg("Invalid agent image URL")
	}

	u, err := url.Parse(arg)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return grpcutils.InvalidArg("Invalid agent image URL: %s", arg)
	}

	return nil
}

func validateAgentLimit(limit *cordiumv1.Workspace_Spec_Limit) error {
	if limit == nil {
		return nil
	}

	if limit.GetCpu().GetMillicores() > maxAgentLimitMillicores {
		return grpcutils.InvalidArg("The agent CPU limit is too large")
	}

	if limit.GetMemory().GetMegabytes() > maxAgentLimitMegabytes {
		return grpcutils.InvalidArg("The agent memory limit is too large")
	}

	if limit.GetStorage().GetMegabytes() > maxAgentLimitMegabytes {
		return grpcutils.InvalidArg("The agent storage limit is too large")
	}

	return nil
}
