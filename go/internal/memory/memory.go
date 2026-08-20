// Package memory implements the LUNA.md memory file system, the equivalent
// of Claude Code's CLAUDE.md auto-loaded project context.
//
// Memory files are loaded at session start and injected into the system prompt.
// They support:
//   - Multi-level loading: ~/.luna/LUNA.md (user) then ./LUNA.md (project)
//   - @path/to/file import syntax (recursive, with cycle detection)
//   - Max depth guard (default 10 levels)
//
// SESSION-59 scope: loader, @import parser, cycle detection, unit tests.
package memory

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// MemoryFileName is the default memory file name.
	MemoryFileName = "LUNA.md"
	// MaxImportDepth is the maximum recursion depth for @import resolution.
	MaxImportDepth = 10
)

// LoadMemoryFiles reads and concatenates memory files from all levels,
// resolving @import directives. Returns the combined content string.
//
// Search order:
//  1. ~/.luna/LUNA.md (user-level)
//  2. <projectRoot>/LUNA.md (project-level)
//
// Missing files are silently skipped. Import errors are collected but
// do not prevent the rest of the content from loading.
func LoadMemoryFiles(projectRoot, userConfigDir string) (string, []string, error) {
	var parts []string
	var warnings []string
	visited := make(map[string]bool)

	// Level 1: user-level memory
	userPath := filepath.Join(userConfigDir, MemoryFileName)
	if content, warns, err := loadFileWithImports(userPath, visited, 0); err != nil {
		warnings = append(warnings, fmt.Sprintf("memory: gagal baca %s: %v", userPath, err))
	} else if content != "" {
		parts = append(parts, "# User Memory ("+userPath+")\n\n"+content)
		warnings = append(warnings, warns...)
	}

	// Level 2: project-level memory
	if projectRoot != "" {
		projPath := filepath.Join(projectRoot, MemoryFileName)
		if content, warns, err := loadFileWithImports(projPath, visited, 0); err != nil {
			warnings = append(warnings, fmt.Sprintf("memory: gagal baca %s: %v", projPath, err))
		} else if content != "" {
			parts = append(parts, "# Project Memory ("+projPath+")\n\n"+content)
			warnings = append(warnings, warns...)
		}
	}

	return strings.Join(parts, "\n\n---\n\n"), warnings, nil
}

// loadFileWithImports reads a file and recursively resolves @import
// directives. Cycle detection is done via the visited map; max depth
// prevents runaway recursion even without cycles (e.g. deeply nested
// legitimate imports).
func loadFileWithImports(path string, visited map[string]bool, depth int) (string, []string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}

	// Check if file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", nil, nil // silently skip missing files
	}

	// Cycle detection
	if visited[absPath] {
		return "", []string{fmt.Sprintf("memory: siklus import terdeteksi: %s", absPath)}, nil
	}
	visited[absPath] = true

	// Depth guard
	if depth > MaxImportDepth {
		return "", []string{fmt.Sprintf("memory: kedalaman import melebihi batas (%d): %s", MaxImportDepth, absPath)}, nil
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", nil, err
	}

	baseDir := filepath.Dir(absPath)
	return resolveImports(string(data), baseDir, visited, depth)
}

// resolveImports scans content line by line. Lines starting with "@"
// followed by a file path are treated as import directives. The imported
// file's content (with its own imports resolved recursively) replaces
// the @-line.
//
// Import syntax:
//
//	@path/to/file.md        (relative to the importing file's directory)
//	@/absolute/path/file.md (absolute path)
func resolveImports(content, baseDir string, visited map[string]bool, depth int) (string, []string, error) {
	var result []string
	var warnings []string

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "@") && len(trimmed) > 1 {
			importPath := strings.TrimSpace(trimmed[1:])
			if importPath == "" {
				result = append(result, line)
				continue
			}

			// Resolve relative paths against the importing file's directory
			if !filepath.IsAbs(importPath) {
				importPath = filepath.Join(baseDir, importPath)
			}

			imported, warns, err := loadFileWithImports(importPath, visited, depth+1)
			warnings = append(warnings, warns...)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("memory: gagal import %s: %v", importPath, err))
				result = append(result, fmt.Sprintf("<!-- import gagal: %s -->", importPath))
			} else if imported != "" {
				result = append(result, imported)
			} else {
				// File not found, keep the line as a comment
				result = append(result, fmt.Sprintf("<!-- file tidak ditemukan: %s -->", importPath))
			}
		} else {
			result = append(result, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return "", warnings, err
	}

	return strings.Join(result, "\n"), warnings, nil
}
