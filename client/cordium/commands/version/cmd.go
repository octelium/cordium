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

package version

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ghodss/yaml"
	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/go-version"
	"github.com/octelium/octelium/client/common/cliutils"
	"github.com/octelium/octelium/pkg/utils/ldflags"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var cmdArgs args

type args struct {
	Out       string
	CheckMode bool
}

func init() {
	Cmd.PersistentFlags().StringVarP(&cmdArgs.Out, "out", "o", "yaml", "Output format")
	Cmd.PersistentFlags().BoolVar(&cmdArgs.CheckMode, "check", false,
		"Check whether there is a more recent latest release for Cordium CLI")
}

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		return doCmd(cmd, args)
	},
}

type OcteliumVersion struct {
	cliutils.OcteliumCommonVersion
}

func doCmd(cmd *cobra.Command, args []string) error {

	ctx := cmd.Context()
	if cmdArgs.CheckMode {
		return doCheckClient(ctx)
	}

	i := &OcteliumVersion{
		OcteliumCommonVersion: *cliutils.GetOcteliumCommonVersion(),
	}

	switch cmdArgs.Out {
	case "json":
		out, err := json.MarshalIndent(i, "", "    ")
		if err != nil {
			return err
		}

		fmt.Printf("%s", out)
	case "yaml":
		out, err := yaml.Marshal(i)
		if err != nil {
			return err
		}

		fmt.Printf("%s", out)
	default:
		return errors.Errorf("Invalid format `%s`. It must be either yaml or json", cmdArgs.Out)
	}

	return nil
}

func doCheckClient(ctx context.Context) error {

	latestVersion, err := getLatestVersion(ctx)
	if err != nil {
		return err
	}

	currentVersion, err := version.NewSemver(ldflags.SemVer)
	if err != nil {
		return err
	}

	if latestVersion.LessThanOrEqual(currentVersion) {
		cliutils.LineNotify("Your client version is up-to-date.\n")
		cliutils.LineNotify("Current Client Version: %s\n", currentVersion.String())
		cliutils.LineNotify("Latest Client Version: %s\n", latestVersion.String())
		return nil
	}

	cliutils.LineNotify("Current Client Version: %s\n", currentVersion.String())
	cliutils.LineNotify("Latest Client Version: %s\n", latestVersion.String())

	cliutils.LineNotify("Cordium CLI can be upgraded using the following command:\n")

	switch {
	case cliutils.IsLinux(), cliutils.IsDarwin():
		cliutils.LineNotify("curl -fsSL https://octelium.com/install-cordium.sh | bash\n")
	case cliutils.IsWindows():
		cliutils.LineNotify("iwr https://octelium.com/install-cordium.ps1 -useb | iex\n")
	default:
		return errors.Errorf("Unsupported OS platform")
	}

	return nil
}

func getLatestVersion(ctx context.Context) (*version.Version, error) {
	resp, err := resty.New().SetDebug(ldflags.IsDev()).
		R().
		SetContext(ctx).
		Get("https://raw.githubusercontent.com/octelium/cordium/refs/heads/main/unsorted/latest_release")
	if err != nil {
		return nil, err
	}

	if !resp.IsSuccess() {
		return nil, errors.Errorf("Could not get latest Cordium version release")
	}

	return version.NewSemver(string(resp.Body()))
}
