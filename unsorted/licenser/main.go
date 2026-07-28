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

package main

import (
	"bytes"
	"context"
	"fmt"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	clusterComponents = []string{
		"apiserver",
		"common",
		"genesis",
		"mockapiserver",
		"nocturne",
		"portal",
		"rscserver",
		"supervisor",
		"vigil",
		"workspace",
	}

	clientComponents = []string{
		"cordium",
	}

	additionalApacheModules = []string{
		"apis",
		"pkg",
	}

	headerRoots = []string{
		"./apis",
		"./pkg",
		"./client",
		"./cluster",
	}
)

func main() {
	if err := doMain(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "licenser: %v\n", err)
		os.Exit(1)
	}
}

func doMain(ctx context.Context) error {
	apachev2, err := os.ReadFile("./unsorted/licenser/apachev2.txt")
	if err != nil {
		return fmt.Errorf("read Apache 2.0 license: %w", err)
	}

	for _, filePath := range licensePaths() {
		if err := writeFileIfChanged(filePath, apachev2, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filePath, err)
		}
	}

	for _, rootPath := range headerRoots {
		if err := setHeader(ctx, rootPath, apacheHeader); err != nil {
			return err
		}
	}

	return nil
}

func licensePaths() []string {
	ret := []string{"LICENSE"}

	for _, component := range clusterComponents {
		ret = append(ret, filepath.Join("cluster", component, "LICENSE"))
	}

	for _, component := range clientComponents {
		ret = append(ret, filepath.Join("client", component, "LICENSE"))
	}

	for _, module := range additionalApacheModules {
		ret = append(ret, filepath.Join(module, "LICENSE"))
	}

	return ret
}

func setHeader(ctx context.Context, rootPath, header string) error {
	header = strings.TrimSpace(header)
	if header == "" {
		return fmt.Errorf("empty header")
	}

	if err := filepath.WalkDir(rootPath, func(filePath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if entry.IsDir() {
			if filePath != rootPath && shouldSkipDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}

		if filepath.Ext(entry.Name()) != ".go" {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", filePath, err)
		}

		if !info.Mode().IsRegular() {
			return nil
		}

		src, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read %s: %w", filePath, err)
		}

		updated, changed, err := rewriteGoHeader(filePath, src, header)
		if err != nil {
			return err
		}
		if !changed {
			return nil
		}

		if err := writeFileIfChanged(filePath, updated, info.Mode().Perm()); err != nil {
			return fmt.Errorf("write %s: %w", filePath, err)
		}

		return nil
	}); err != nil {
		return fmt.Errorf("set headers under %s: %w", rootPath, err)
	}

	return nil
}

func rewriteGoHeader(filePath string, src []byte, header string) ([]byte, bool, error) {
	bom, srcWithoutBOM := splitUTF8BOM(src)

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, srcWithoutBOM, parser.PackageClauseOnly)
	if err != nil {
		return nil, false, fmt.Errorf("parse package clause in %s: %w", filePath, err)
	}

	packageOffset := fset.Position(file.Package).Offset
	if packageOffset < 0 || packageOffset > len(srcWithoutBOM) {
		return nil, false, fmt.Errorf("invalid package offset in %s", filePath)
	}

	preamble := srcWithoutBOM[:packageOffset]
	body := srcWithoutBOM[packageOffset:]
	eol := detectLineEnding(srcWithoutBOM)

	preamble = removeApacheHeaders(preamble)
	buildConstraints, preamble := extractBuildConstraints(preamble)
	preamble = trimLeadingBlankLines(preamble)

	var ret bytes.Buffer
	ret.Grow(len(src) + len(header) + 4*len(eol))
	ret.Write(bom)

	if len(buildConstraints) > 0 {
		for idx, constraint := range buildConstraints {
			if idx > 0 {
				ret.WriteString(eol)
			}
			ret.WriteString(constraint)
		}
		ret.WriteString(eol)
		ret.WriteString(eol)
	}

	ret.WriteString(withLineEnding(header, eol))
	ret.WriteString(eol)
	ret.WriteString(eol)

	if len(bytes.TrimSpace(preamble)) > 0 {
		ret.Write(preamble)
	}
	ret.Write(body)

	updated := ret.Bytes()
	return updated, !bytes.Equal(src, updated), nil
}

func removeApacheHeaders(src []byte) []byte {
	comments := scanComments(src)
	if len(comments) == 0 {
		return src
	}

	var removals []sourceSpan

	for idx := 0; idx < len(comments); idx++ {
		comment := comments[idx]

		if strings.HasPrefix(comment.text, "/*") {
			if isOcteliumApacheHeader(comment.text) {
				removals = append(removals, sourceSpan{start: comment.start, end: comment.end})
			}
			continue
		}

		if !strings.HasPrefix(comment.text, "//") ||
			!strings.Contains(comment.text, octeliumCopyrightText) {
			continue
		}

		endIdx := idx
		for endIdx+1 < len(comments) && adjacentLineComments(src, comments[endIdx], comments[endIdx+1]) {
			next := comments[endIdx+1]
			if isBuildConstraint(next.text) || isGeneratedCodeMarker(next.text) {
				break
			}

			endIdx++
			if strings.Contains(next.text, apacheHeaderTerminator) {
				break
			}
		}

		candidate := string(src[comment.start:comments[endIdx].end])
		if isOcteliumApacheHeader(candidate) {
			removals = append(removals, sourceSpan{
				start: comment.start,
				end:   comments[endIdx].end,
			})
			idx = endIdx
		}
	}

	return removeSpans(src, removals)
}

func scanComments(src []byte) []sourceComment {
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	var ret []sourceComment
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if tok != token.COMMENT {
			continue
		}

		start := file.Offset(pos)
		ret = append(ret, sourceComment{
			start: start,
			end:   start + len(lit),
			text:  lit,
		})
	}

	return ret
}

func extractBuildConstraints(src []byte) ([]string, []byte) {
	lines := splitLines(src)
	if len(lines) == 0 {
		return nil, src
	}

	var goBuild []string
	var plusBuild []string
	var remainder bytes.Buffer

	for _, line := range lines {
		text := strings.TrimSpace(string(bytes.TrimRight(line, "\r\n")))
		switch {
		case isGoBuildConstraint(text):
			goBuild = append(goBuild, text)
		case isPlusBuildConstraint(text):
			plusBuild = append(plusBuild, text)
		default:
			remainder.Write(line)
		}
	}

	constraints := append(goBuild, plusBuild...)
	return constraints, remainder.Bytes()
}

func splitLines(src []byte) [][]byte {
	if len(src) == 0 {
		return nil
	}

	var ret [][]byte
	for len(src) > 0 {
		idx := bytes.IndexByte(src, '\n')
		if idx < 0 {
			ret = append(ret, src)
			break
		}

		idx++
		ret = append(ret, src[:idx])
		src = src[idx:]
	}
	return ret
}

func trimLeadingBlankLines(src []byte) []byte {
	for len(src) > 0 {
		idx := bytes.IndexByte(src, '\n')
		if idx < 0 {
			if len(bytes.TrimSpace(src)) == 0 {
				return nil
			}
			return src
		}

		line := src[:idx+1]
		if len(bytes.TrimSpace(line)) != 0 {
			return src
		}
		src = src[idx+1:]
	}
	return src
}

func adjacentLineComments(src []byte, current, next sourceComment) bool {
	if !strings.HasPrefix(current.text, "//") || !strings.HasPrefix(next.text, "//") {
		return false
	}
	if current.end > next.start {
		return false
	}

	between := src[current.end:next.start]
	if len(bytes.TrimSpace(between)) != 0 {
		return false
	}

	return bytes.Count(between, []byte{'\n'}) == 1
}

func isOcteliumApacheHeader(text string) bool {
	return strings.Contains(text, octeliumCopyrightText) &&
		strings.Contains(text, apacheLicenseText) &&
		strings.Contains(text, apacheHeaderTerminator)
}

func isGeneratedCodeMarker(text string) bool {
	text = strings.TrimSpace(text)
	return strings.HasPrefix(text, "// Code generated ") &&
		strings.HasSuffix(text, " DO NOT EDIT.")
}

func isBuildConstraint(text string) bool {
	text = strings.TrimSpace(text)
	return isGoBuildConstraint(text) || isPlusBuildConstraint(text)
}

func isGoBuildConstraint(text string) bool {
	return text == "//go:build" || strings.HasPrefix(text, "//go:build ")
}

func isPlusBuildConstraint(text string) bool {
	return text == "// +build" || strings.HasPrefix(text, "// +build ")
}

func removeSpans(src []byte, spans []sourceSpan) []byte {
	if len(spans) == 0 {
		return src
	}

	var ret bytes.Buffer
	ret.Grow(len(src))

	cursor := 0
	for _, span := range spans {
		if span.start < cursor || span.end < span.start || span.end > len(src) {
			continue
		}
		ret.Write(src[cursor:span.start])
		cursor = span.end
	}
	ret.Write(src[cursor:])
	return ret.Bytes()
}

func detectLineEnding(src []byte) string {
	if bytes.Contains(src, []byte("\r\n")) {
		return "\r\n"
	}
	return "\n"
}

func withLineEnding(value, eol string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	if eol == "\n" {
		return value
	}
	return strings.ReplaceAll(value, "\n", eol)
}

func splitUTF8BOM(src []byte) ([]byte, []byte) {
	if bytes.HasPrefix(src, utf8BOM) {
		return src[:len(utf8BOM)], src[len(utf8BOM):]
	}
	return nil, src
}

func shouldSkipDirectory(name string) bool {
	switch name {
	case ".git", "node_modules", "third_party", "vendor":
		return true
	default:
		return false
	}
}

func writeFileIfChanged(filePath string, content []byte, perm fs.FileMode) error {
	current, err := os.ReadFile(filePath)
	if err == nil && bytes.Equal(current, content) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return os.WriteFile(filePath, content, perm)
}

type sourceSpan struct {
	start int
	end   int
}

type sourceComment struct {
	start int
	end   int
	text  string
}

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

const (
	octeliumCopyrightText  = "Copyright Octelium Labs, LLC. All rights reserved."
	apacheLicenseText      = "Licensed under the Apache License, Version 2.0"
	apacheHeaderTerminator = "limitations under the License."
)

const apacheHeader = `/*
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
 */`
