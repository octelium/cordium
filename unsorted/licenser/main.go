// Copyright Octelium Labs, LLC. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	if err := doMain(context.Background()); err != nil {
		panic(err)
	}
}

func doMain(ctx context.Context) error {

	clusterComponents := []string{
		"apiserver",
		"common",
		"genesis",
		"nocturne",
		"portal",
		"rscserver",
		"vigil",
		"supervisor",
		"workspace",
		"mockapiserver",
	}

	clientComponents := []string{
		"cordium",
	}

	additionalApacheModules := []string{
		"apis",
		"pkg",
	}

	agplv3, err := os.ReadFile("./unsorted/licenser/agplv3.txt")
	if err != nil {
		return err
	}

	/*
		apachev2, err := os.ReadFile("./unsorted/licenser/apachev2.txt")
		if err != nil {
			return err
		}

		if err := os.WriteFile("LICENSE-APACHE", []byte(apachev2), 0666); err != nil {
			return err
		}
	*/

	if err := os.WriteFile("LICENSE-AGPL-3.0", []byte(agplv3), 0666); err != nil {
		return err
	}

	for _, comp := range clusterComponents {

		if err := os.WriteFile(path.Join("cluster", comp, "LICENSE"), []byte(agplv3), 0666); err != nil {
			return err
		}
	}

	for _, comp := range clientComponents {
		if err := os.WriteFile(path.Join("client", comp, "LICENSE"), []byte(agplv3), 0666); err != nil {
			return err
		}
	}

	for _, mod := range additionalApacheModules {
		if err := os.WriteFile(path.Join(mod, "LICENSE"), []byte(agplv3), 0666); err != nil {
			return err
		}
	}

	if err := setHeader(ctx, "./apis", agplHeader); err != nil {
		return err
	}

	if err := setHeader(ctx, "./pkg", agplHeader); err != nil {
		return err
	}

	if err := setHeader(ctx, "./client", agplHeader); err != nil {
		return err
	}

	if err := setHeader(ctx, "./cluster", agplHeader); err != nil {
		return err
	}

	return nil
}

func setClusterHeader(ctx context.Context) error {
	return setHeader(ctx, "./cluster", agplHeader)
}

func setHeader(ctx context.Context, rootPath string, header string) error {

	if err := filepath.Walk(rootPath,
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

			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			cn, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			pkgIdx := getIdx(cn[:])
			if pkgIdx < 0 {
				return nil
			}

			newFile := header + "\n" + string(cn[pkgIdx:])

			if err := os.WriteFile(path, []byte(newFile), info.Mode().Perm()); err != nil {
				return err
			}

			return nil
		}); err != nil {
		return err
	}
	return nil
}

func getIdx(src []byte) int {

	ret := bytes.Index(src, []byte("package "))
	if idx := bytes.Index(src, []byte("//go:build")); idx > 0 && idx < ret {
		ret = idx
	}

	if idx := bytes.Index(src, []byte("// +build")); idx > 0 && idx < ret {
		ret = idx
	}

	if idx := bytes.Index(src, []byte("// Code generated")); idx > 0 && idx < ret {
		ret = idx
	}

	return ret
}

const agplHeader = `/*
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
`
