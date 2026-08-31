package clikit_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// This project is MIT. It can absorb permissive dependencies but not copyleft
// ones, and "we checked once at review time" does not survive a dependency
// bump. This test is the gate: it walks every module that compiles into a user
// binary, on every platform we support, and fails on anything that is not
// recognisably permissive — including anything it cannot recognise at all,
// because an unclassified licence needs a human to look at it.

var permissiveLicenses = map[string]bool{
	"MIT":          true,
	"BSD-2-Clause": true,
	"BSD-3-Clause": true,
	"Apache-2.0":   true,
	"ISC":          true,
	"Unlicense":    true,
}

var licenseFilenames = []string{
	"LICENSE", "LICENSE.md", "LICENSE.txt",
	"LICENCE", "LICENCE.md", "LICENCE.txt",
	"COPYING", "COPYING.md",
	"UNLICENSE",
}

func TestDependencyLicensesArePermissive(t *testing.T) {
	if testing.Short() {
		t.Skip("license audit shells out to `go list`")
	}

	modules := map[string]string{}
	for _, goos := range []string{"darwin", "linux", "windows"} {
		for path, dir := range compiledModules(t, goos) {
			modules[path] = dir
		}
	}
	if len(modules) < 10 {
		t.Fatalf("found only %d dependencies, the audit is not seeing the module graph", len(modules))
	}

	for path, dir := range modules {
		file, text := findLicense(dir)
		if file == "" {
			t.Errorf("%s: no license file in %s", path, dir)
			continue
		}
		license := classifyLicense(text)
		if !permissiveLicenses[license] {
			t.Errorf("%s: %s license in %s — this project is MIT and cannot take it", path, license, filepath.Base(file))
			continue
		}
		t.Logf("%-14s %s", license, path)
	}
}

// compiledModules maps module path to source directory for every module that
// ends up in a binary for the given platform. Test-only and graph-only modules
// are excluded: they are never distributed, so they carry no obligation.
func compiledModules(t *testing.T, goos string) map[string]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{.Path}}\t{{.Dir}}{{end}}", "./...")
	cmd.Env = append(os.Environ(), "GOOS="+goos)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list for GOOS=%s: %v", goos, err)
	}

	modules := map[string]string{}
	for _, line := range strings.Split(string(output), "\n") {
		path, dir, found := strings.Cut(strings.TrimSpace(line), "\t")
		if !found || dir == "" || strings.HasPrefix(path, "github.com/xlshcn/clikit-go") {
			continue
		}
		modules[path] = dir
	}
	return modules
}

// findLicense looks in the module root, then walks up: Go sub-modules such as
// charmbracelet/x/ansi often sit under a repository-level license.
func findLicense(dir string) (string, string) {
	for current := dir; current != "" && current != string(filepath.Separator); current = filepath.Dir(current) {
		for _, name := range licenseFilenames {
			path := filepath.Join(current, name)
			if content, err := os.ReadFile(path); err == nil {
				return path, string(content)
			}
		}
		if strings.Contains(filepath.Base(current), "@") {
			break // reached the module cache root for this version
		}
	}
	return "", ""
}

// classifyLicense identifies a license from its operative wording rather than
// its title, which is often just a copyright line.
func classifyLicense(text string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(text), " "))
	switch {
	case strings.Contains(normalized, "gnu general public license"),
		strings.Contains(normalized, "gnu lesser general public"),
		strings.Contains(normalized, "gnu affero general public"):
		return "GPL-family"
	case strings.Contains(normalized, "mozilla public license"):
		return "MPL-2.0"
	case strings.Contains(normalized, "eclipse public license"):
		return "EPL"
	case strings.Contains(normalized, "apache license") && strings.Contains(normalized, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(normalized, "redistribution and use in source and binary forms"):
		if strings.Contains(normalized, "endorse or promote products derived") {
			return "BSD-3-Clause"
		}
		return "BSD-2-Clause"
	case strings.Contains(normalized, "permission is hereby granted, free of charge"):
		return "MIT"
	case strings.Contains(normalized, "permission to use, copy, modify, and/or distribute"):
		return "ISC"
	case strings.Contains(normalized, "released into the public domain"):
		return "Unlicense"
	default:
		return "unrecognized"
	}
}

// The audit above passes today, which proves nothing about whether it would
// catch a bad dependency tomorrow. These are the cases it has to get right.
func TestClassifyLicense(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{"MIT", "Copyright (c) 2026 Someone\n\nPermission is hereby granted, free of charge, to any person obtaining a copy", "MIT"},
		{"BSD-3", "Redistribution and use in source and binary forms, with or without modification\nNeither the name of X nor the names of its contributors may be used to endorse or promote products derived from this software", "BSD-3-Clause"},
		{"BSD-2", "Redistribution and use in source and binary forms, with or without modification, are permitted", "BSD-2-Clause"},
		{"Apache", "Apache License\nVersion 2.0, January 2004", "Apache-2.0"},
		{"ISC", "Permission to use, copy, modify, and/or distribute this software for any purpose", "ISC"},
		{"GPL", "GNU GENERAL PUBLIC LICENSE\nVersion 3, 29 June 2007", "GPL-family"},
		{"AGPL", "GNU AFFERO GENERAL PUBLIC LICENSE\nVersion 3", "GPL-family"},
		{"LGPL", "GNU LESSER GENERAL PUBLIC LICENSE\nVersion 2.1", "GPL-family"},
		{"MPL", "Mozilla Public License Version 2.0", "MPL-2.0"},
		{"gibberish", "this file intentionally left blank", "unrecognized"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := classifyLicense(testCase.text)
			if got != testCase.want {
				t.Errorf("classifyLicense = %q, want %q", got, testCase.want)
			}
			if permissiveLicenses[got] != permissiveLicenses[testCase.want] {
				t.Errorf("%q and %q disagree on whether the dependency is acceptable", got, testCase.want)
			}
		})
	}
}
