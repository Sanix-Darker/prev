package review

import (
	"path/filepath"
	"strings"

	"github.com/sanix-darker/prev/internal/diffparse"
)

// CategorizedFile is an enriched file change with category and group metadata.
type CategorizedFile struct {
	diffparse.EnrichedFileChange
	Category string // "New", "Modified", "Deleted", "Renamed", "Binary"
	Group    string // "tests", "commands", "core", "docs", "dependencies", "ci/config"
}

// CategorizeChanges assigns category and group to each enriched file change.
func CategorizeChanges(changes []diffparse.EnrichedFileChange) []CategorizedFile {
	result := make([]CategorizedFile, 0, len(changes))

	for _, efc := range changes {
		cf := CategorizedFile{
			EnrichedFileChange: efc,
			Category:           detectCategory(efc.FileChange),
			Group:              detectGroup(efc),
		}
		result = append(result, cf)
	}

	return result
}

func detectCategory(fc diffparse.FileChange) string {
	if fc.IsBinary {
		return "Binary"
	}
	if fc.IsNew {
		return "New"
	}
	if fc.IsDeleted {
		return "Deleted"
	}
	if fc.IsRenamed {
		return "Renamed"
	}
	return "Modified"
}

func detectGroup(efc diffparse.EnrichedFileChange) string {
	name := efc.NewName
	if name == "" {
		name = efc.OldName
	}

	base := filepath.Base(name)
	dir := filepath.Dir(name)

	// Test files
	if strings.HasSuffix(name, "_test.go") ||
		strings.HasSuffix(name, "_test.py") ||
		strings.HasSuffix(name, ".test.js") ||
		strings.HasSuffix(name, ".test.ts") ||
		strings.HasSuffix(name, ".spec.js") ||
		strings.HasSuffix(name, ".spec.ts") ||
		strings.HasPrefix(dir, "tests") ||
		strings.HasPrefix(dir, "test") ||
		strings.Contains(dir, "__tests__") {
		return "tests"
	}

	// Commands
	if strings.HasPrefix(dir, "cmd") || strings.HasPrefix(dir, "cmd/") {
		return "commands"
	}

	// Docs
	if strings.HasPrefix(dir, "docs") || strings.HasPrefix(dir, "doc/") ||
		strings.HasSuffix(name, ".md") {
		return "docs"
	}

	// Dependencies
	switch base {
	case "go.mod", "go.sum", "package.json", "package-lock.json",
		"yarn.lock", "Pipfile", "Pipfile.lock", "requirements.txt",
		"Cargo.toml", "Cargo.lock", "pom.xml", "build.gradle":
		return "dependencies"
	}

	// CI/Config
	switch base {
	case "Dockerfile", "docker-compose.yml", "docker-compose.yaml",
		"Makefile", ".goreleaser.yml", ".goreleaser.yaml":
		return "ci/config"
	}
	if strings.HasPrefix(dir, ".github") || strings.HasPrefix(dir, ".gitlab-ci") ||
		strings.HasPrefix(dir, ".circleci") {
		return "ci/config"
	}

	// Core/internal
	if strings.HasPrefix(dir, "internal") || strings.HasPrefix(dir, "pkg") ||
		strings.HasPrefix(dir, "lib") || strings.HasPrefix(dir, "src") {
		return "core"
	}

	return "other"
}
