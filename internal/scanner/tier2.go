// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// FunctionInfo describes a discovered Go function.
type FunctionInfo struct {
	Name       string
	Package    string
	File       string
	Params     []string
	IsExported bool
}

// ImportInfo describes a discovered Go import.
type ImportInfo struct {
	Package string
	Path    string
}

// Tier2Result holds the output of a Tier 2 scan: functions, imports, and test files.
//
// Performance note: slice fields are returned directly without defensive copies.
// Copying every slice on each scan would be wasteful given typical result sizes.
// Callers must not modify the returned slices; treat them as read-only.
type Tier2Result struct {
	Functions    []FunctionInfo
	Imports      []ImportInfo
	TestFiles    []string
	SkippedFiles []SkippedFile
}

// ScanTier2 walks the specified directories under root, parses functions and
// imports, and collects test file paths. Test files are tracked in TestFiles
// rather than being skipped entirely.
//
// Returns an error if root does not exist or is not a directory, or if any
// dir escapes the root via path traversal (e.g. "../../etc").
func ScanTier2(root string, dirs []string) (*Tier2Result, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("scanner: root %s: %w", root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("scanner: root %s is not a directory", root)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("scanner: abs root %s: %w", root, err)
	}

	result := &Tier2Result{}
	fset := token.NewFileSet()
	seenImports := map[string]bool{}

	for _, dir := range dirs {
		absDir := filepath.Join(absRoot, dir)
		absDir, err = filepath.Abs(absDir)
		if err != nil {
			return nil, fmt.Errorf("scanner: abs dir %s: %w", dir, err)
		}
		// Reject any dir that escapes the root. filepath.Rel returns a path
		// starting with ".." when target is outside base.
		rel, relErr := filepath.Rel(absRoot, absDir)
		if relErr != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("scanner: dir %q escapes root %q", dir, root)
		}

		skipped, err := walkGoFiles(absDir, false, fset, func(path, _ string, f *ast.File) error {
			// Compute path relative to root (not absDir) for consistent output.
			relPath, relErr := filepath.Rel(absRoot, path)
			if relErr != nil {
				return fmt.Errorf("scanner: relative path for %s: %w", path, relErr)
			}

			// Track test files separately rather than processing them.
			if strings.HasSuffix(path, "_test.go") {
				result.TestFiles = append(result.TestFiles, relPath)
				return nil
			}

			pkgName := f.Name.Name

			// Collect imports.
			for _, imp := range f.Imports {
				impPath := strings.Trim(imp.Path.Value, `"`)
				if !seenImports[impPath] {
					seenImports[impPath] = true
					segments := strings.Split(impPath, "/")
					result.Imports = append(result.Imports, ImportInfo{
						Package: segments[len(segments)-1],
						Path:    impPath,
					})
				}
			}

			// Collect functions.
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				fi := FunctionInfo{
					Name:       fn.Name.Name,
					Package:    pkgName,
					File:       relPath,
					IsExported: fn.Name.IsExported(),
				}
				if fn.Type.Params != nil {
					for _, param := range fn.Type.Params.List {
						paramType := exprToString(param.Type)
						if len(param.Names) == 0 {
							fi.Params = append(fi.Params, paramType)
						} else {
							for _, name := range param.Names {
								fi.Params = append(fi.Params, name.Name+" "+paramType)
							}
						}
					}
				}
				result.Functions = append(result.Functions, fi)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scanner: tier2 walk %s: %w", dir, err)
		}
		result.SkippedFiles = append(result.SkippedFiles, skipped...)
	}
	return result, nil
}

// exprToString returns a simple string representation of a type expression.
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.ArrayType:
		return "[]" + exprToString(t.Elt)
	case *ast.MapType:
		return "map[" + exprToString(t.Key) + "]" + exprToString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	default:
		return fmt.Sprintf("<%T>", expr)
	}
}
