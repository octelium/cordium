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

package cordium

import (
	"fmt"
	"strings"

	"github.com/octelium/octelium/apis/main/metav1"
)

// Argv turns an argument vector into a single command line that the Workspace
// shell interprets as one command, quoting every argument so that spaces,
// globs, quotes and the other shell metacharacters are passed through
// literally. It is what makes the execution of caller-supplied or
// model-generated arguments safe:
//
//	res, err := ws.Exec(ctx, cordium.Argv("git", "clone", url, dir))
//
// Cordium executes a command through a shell, so a raw string such as
// "cat a.txt | grep foo" keeps its pipes and redirections, while Argv
// deliberately takes them away.
func Argv(args ...string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, ShellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

// ShellQuote quotes a single argument for a POSIX shell.
func ShellQuote(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, "\\\"'`$&|;<>()[]{}*?!#~^ \t\n\r") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
}

// ShortName returns the first label of a qualified Cordium resource name, which
// is how the names are normally shown to people (e.g. "ml-env" for
// "ml-env.research.jdoe").
func ShortName(name string) string {
	if name == "" {
		return ""
	}
	return strings.Split(name, ".")[0]
}

// OrganizationSpaceName qualifies a Space name so that the Cluster creates a
// shared, multi-Member ORGANIZATION Space rather than a personal one. See
// [SpaceClient.Create].
func OrganizationSpaceName(name string) string {
	if name == "" || strings.Contains(name, ".") {
		return name
	}
	return name + ".cordium"
}

func portAppName(port int) string {
	return fmt.Sprintf("port-%d", port)
}

func nameRef(name string) *metav1.ObjectReference {
	return &metav1.ObjectReference{Name: name}
}

func uidRef(uid string) *metav1.ObjectReference {
	return &metav1.ObjectReference{Uid: uid}
}

func getOptionsFor(name string) *metav1.GetOptions {
	return &metav1.GetOptions{Name: name}
}

func deleteOptionsFor(name string) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{Name: name}
}
