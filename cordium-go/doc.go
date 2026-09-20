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

// Package cordium is the official Go SDK for Cordium, the self-hosted sandbox
// and cloud development environment platform that runs on top of Octelium.
//
// The SDK is built around two layers:
//
// The high level layer provides Go-native types and helpers for the resources
// that applications and AI agents work with every day. Workspaces (i.e.
// sandboxes) are created from composable options, started, awaited, executed
// against and destroyed:
//
//	c, err := cordium.New(ctx, cordium.WithDomain("example.com"))
//	if err != nil {
//		return err
//	}
//	defer c.Close()
//
//	ws, err := c.Workspaces().Run(ctx,
//		cordium.WithImage("python:3.11-slim"),
//		cordium.WithRepo("https://github.com/myorg/my-project"),
//		cordium.Ephemeral(),
//	)
//	if err != nil {
//		return err
//	}
//	defer ws.Delete(context.WithoutCancel(ctx))
//
//	res, err := ws.Exec(ctx, "pytest -q")
//	if err != nil {
//		return err
//	}
//	fmt.Println(res.ExitCode, res.StdoutString())
//
// Commands can be run synchronously or streamed, terminals attached, the
// initialization logs followed and files moved in and out of a Workspace.
//
// The low level layer is the generated gRPC API itself. It is always reachable
// through [Client.MainService], [Client.WorkspaceService] and
// [Client.ManagementService] so that nothing the Cluster exposes is ever out of
// reach, and through [Client.Conn] for a raw authenticated connection:
//
//	wsList, err := c.MainService().ListWorkspace(ctx, &cordiumv1.ListWorkspaceOptions{})
//
// A Client is safe for concurrent use. Applications should normally create one
// Client per Cluster and share it for the lifetime of the process.
//
// # Authentication
//
// Cordium runs on top of an Octelium Cluster and therefore uses Octelium
// Credentials. By default the Client reads its credentials from the
// environment, which makes it work unchanged in CI jobs, Kubernetes Pods and
// inside Cordium Workspaces themselves:
//
//	CORDIUM_DOMAIN or OCTELIUM_DOMAIN           the Cluster domain
//	OCTELIUM_ACCESS_TOKEN                       a ready to use access token
//	OCTELIUM_ASSERTION_FILE, OCTELIUM_ASSERTION an OIDC/SAML style assertion
//	OCTELIUM_AUTH_TOKEN                         a Credential authentication token
//
// Credentials can also be supplied explicitly with [WithAuthenticationToken],
// [WithAccessToken], [WithAssertion], [WithAssertionFile] or, for anything
// else, with [WithOcteliumOptions].
package cordium
