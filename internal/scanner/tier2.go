// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner

import (
	"fmt"
	"go/ast"
	"go/token"
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
// Callers must not modify the returned slices.
type Tier2Result struct {
	Functions    []FunctionInfo
	Imports      []ImportInfo
	TestFiles    []string
	SkippedFiles []SkippedFile
}

// ScanTier2 walks the specified directories under root, parses functions and
// imports, and collects test file paths. Test files are tracked in TestFiles
// rather than being skipped entirely.
func ScanTier2(root string, dirs []string) (*Tier2Result, error) {
	result := &Tier2Result{}
	fset := token.NewFileSet()
	seenImports := map[string]bool{}

	for _, dir := range dirs {
		absDir := filepath.Join(root, dir)
		skipped, err := walkGoFiles(absDir, false, fset, func(path, _ string, f *ast.File) error {
			// Compute path relative to root (not absDir) for consistent output.
			relPath, relErr := filepath.Rel(root, path)
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
