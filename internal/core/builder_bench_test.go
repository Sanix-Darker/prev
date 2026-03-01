package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sanix-darker/prev/internal/config"
)

func BenchmarkBuildOptimPrompt(b *testing.B) {
	conf := config.Config{}
	code := strings.Repeat("func work() { x += 1 }\n", 300)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildOptimPrompt(conf, code)
	}
}

func BenchmarkBuildDiff(b *testing.B) {
	dir := b.TempDir()
	oldPath := filepath.Join(dir, "old.go")
	newPath := filepath.Join(dir, "new.go")

	var oldBuf strings.Builder
	var newBuf strings.Builder
	for i := 0; i < 800; i++ {
		oldBuf.WriteString("line ")
		oldBuf.WriteString("A")
		oldBuf.WriteString("\n")
		newBuf.WriteString("line ")
		if i%9 == 0 {
			newBuf.WriteString("B")
		} else {
			newBuf.WriteString("A")
		}
		newBuf.WriteString("\n")
	}

	if err := os.WriteFile(oldPath, []byte(oldBuf.String()), 0o644); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte(newBuf.String()), 0o644); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildDiff(oldPath, newPath); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkBuildDiff_IdenticalFastPath(b *testing.B) {
	dir := b.TempDir()
	pathA := filepath.Join(dir, "a.go")
	pathB := filepath.Join(dir, "b.go")
	content := []byte(strings.Repeat("same line\n", 1500))
	if err := os.WriteFile(pathA, content, 0o644); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(pathB, content, 0o644); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := BuildDiff(pathA, pathB); err != nil {
			b.Fatal(err)
		}
	}
}
