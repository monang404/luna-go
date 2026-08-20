package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadMemoryFiles_SingleProjectFile(t *testing.T) {
	projectRoot := t.TempDir()
	userDir := t.TempDir()

	// Create project LUNA.md
	content := "# Project Rules\n\nAlways use tabs for indentation.\n"
	if err := os.WriteFile(filepath.Join(projectRoot, MemoryFileName), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, warnings, err := LoadMemoryFiles(projectRoot, userDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !strings.Contains(result, "Always use tabs") {
		t.Errorf("result missing project content:\n%s", result)
	}
}

func TestLoadMemoryFiles_BothLevels(t *testing.T) {
	projectRoot := t.TempDir()
	userDir := t.TempDir()

	// Create user LUNA.md
	userContent := "# Global Rules\nPrefer Go over Python.\n"
	if err := os.WriteFile(filepath.Join(userDir, MemoryFileName), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create project LUNA.md
	projContent := "# Project Rules\nUse gofmt.\n"
	if err := os.WriteFile(filepath.Join(projectRoot, MemoryFileName), []byte(projContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, warnings, err := LoadMemoryFiles(projectRoot, userDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Both should be present, user first
	if !strings.Contains(result, "Prefer Go over Python") {
		t.Error("missing user memory content")
	}
	if !strings.Contains(result, "Use gofmt") {
		t.Error("missing project memory content")
	}

	// User content should come before project content
	userIdx := strings.Index(result, "Prefer Go over Python")
	projIdx := strings.Index(result, "Use gofmt")
	if userIdx > projIdx {
		t.Error("user memory should come before project memory")
	}
}

func TestLoadMemoryFiles_MissingFiles(t *testing.T) {
	projectRoot := t.TempDir()
	userDir := t.TempDir()

	result, warnings, err := LoadMemoryFiles(projectRoot, userDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if result != "" {
		t.Errorf("expected empty result for missing files, got: %q", result)
	}
}

func TestLoadMemoryFiles_NoProjectRoot(t *testing.T) {
	userDir := t.TempDir()

	// User LUNA.md only
	userContent := "Global rules here.\n"
	if err := os.WriteFile(filepath.Join(userDir, MemoryFileName), []byte(userContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, err := LoadMemoryFiles("", userDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "Global rules") {
		t.Error("missing user memory content with empty projectRoot")
	}
}

func TestResolveImports_SimpleImport(t *testing.T) {
	dir := t.TempDir()

	// Create an imported file
	importedContent := "Imported rules: use consistent naming.\n"
	importedPath := filepath.Join(dir, "naming.md")
	if err := os.WriteFile(importedPath, []byte(importedContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Main file with @import
	mainContent := "# Rules\n@naming.md\n## End\n"
	mainPath := filepath.Join(dir, MemoryFileName)
	if err := os.WriteFile(mainPath, []byte(mainContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, warnings, err := loadFileWithImports(mainPath, make(map[string]bool), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !strings.Contains(result, "Imported rules") {
		t.Errorf("imported content missing:\n%s", result)
	}
	if !strings.Contains(result, "# Rules") {
		t.Error("original content missing")
	}
	if !strings.Contains(result, "## End") {
		t.Error("trailing content missing")
	}
}

func TestResolveImports_NestedImport(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "sub")
	os.MkdirAll(subDir, 0o755)

	// Level 3: deepest file
	if err := os.WriteFile(filepath.Join(subDir, "deep.md"), []byte("deepest content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Level 2: middle file imports deep.md
	if err := os.WriteFile(filepath.Join(dir, "middle.md"), []byte("middle content\n@sub/deep.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Level 1: top file imports middle.md
	topContent := "top content\n@middle.md\n"
	topPath := filepath.Join(dir, "top.md")
	if err := os.WriteFile(topPath, []byte(topContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, warnings, err := loadFileWithImports(topPath, make(map[string]bool), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if !strings.Contains(result, "top content") {
		t.Error("missing top content")
	}
	if !strings.Contains(result, "middle content") {
		t.Error("missing middle content")
	}
	if !strings.Contains(result, "deepest content") {
		t.Error("missing deepest content")
	}
}

func TestResolveImports_CycleDetection(t *testing.T) {
	dir := t.TempDir()

	// A imports B, B imports A -> cycle
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("content A\n@b.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("content B\n@a.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, warnings, err := loadFileWithImports(filepath.Join(dir, "a.md"), make(map[string]bool), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have a cycle warning
	hasCycleWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "siklus import") {
			hasCycleWarning = true
			break
		}
	}
	if !hasCycleWarning {
		t.Error("expected cycle detection warning")
	}

	// Both files' content should still be present (just the cycle re-import blocked)
	if !strings.Contains(result, "content A") {
		t.Error("missing content A")
	}
	if !strings.Contains(result, "content B") {
		t.Error("missing content B")
	}
}

func TestResolveImports_MaxDepthGuard(t *testing.T) {
	dir := t.TempDir()

	// Create a chain deeper than MaxImportDepth
	for i := 0; i <= MaxImportDepth+2; i++ {
		name := filepath.Join(dir, "level"+strings.Repeat("_", i)+".md")
		nextName := "level" + strings.Repeat("_", i+1) + ".md"
		content := "level " + strings.Repeat("_", i) + "\n@" + nextName + "\n"
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, warnings, err := loadFileWithImports(filepath.Join(dir, "level.md"), make(map[string]bool), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have a depth warning
	hasDepthWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "kedalaman import melebihi batas") {
			hasDepthWarning = true
			break
		}
	}
	if !hasDepthWarning {
		t.Errorf("expected depth limit warning, got warnings: %v", warnings)
	}

	// Early levels should still be present
	if !strings.Contains(result, "level ") {
		t.Error("missing early level content")
	}
}

func TestResolveImports_MissingImportFile(t *testing.T) {
	dir := t.TempDir()

	mainContent := "before\n@nonexistent.md\nafter\n"
	mainPath := filepath.Join(dir, "main.md")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, err := loadFileWithImports(mainPath, make(map[string]bool), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-existent import should leave a comment placeholder
	if !strings.Contains(result, "file tidak ditemukan") {
		t.Errorf("expected 'not found' comment for missing import:\n%s", result)
	}
	if !strings.Contains(result, "before") || !strings.Contains(result, "after") {
		t.Error("surrounding content should be preserved")
	}
}

func TestResolveImports_AtSignNotImport(t *testing.T) {
	dir := t.TempDir()

	// Bare @ or @ in middle of line should not trigger import
	mainContent := "@\n@ \nemail@example.com\n@valid_import.md\n"
	mainPath := filepath.Join(dir, "main.md")
	if err := os.WriteFile(mainPath, []byte(mainContent), 0o644); err != nil {
		t.Fatal(err)
	}

	result, _, err := loadFileWithImports(mainPath, make(map[string]bool), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The @ and @ lines should be preserved as-is (not treated as imports)
	if !strings.Contains(result, "email@example.com") {
		t.Error("non-import @ line should be preserved")
	}
}
