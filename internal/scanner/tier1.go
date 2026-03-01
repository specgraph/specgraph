// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package scanner

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
)

// PackageInfo describes a discovered Go package.
type PackageInfo struct {
	Name string
	Path string
}

// InterfaceInfo describes a discovered Go interface.
type InterfaceInfo struct {
	Name    string
	Package string
	Methods []string
}

// StructInfo describes a discovered Go struct.
type StructInfo struct {
	Name    string
	Package string
	Fields  []string
}

// Tier1Result holds the output of a Tier 1 scan: packages, interfaces, and structs.
type Tier1Result struct {
	Packages   []PackageInfo
	Interfaces []InterfaceInfo
	Structs    []StructInfo
}

// ScanTier1 walks Go files under root, parses AST, and extracts packages,
// interfaces, and structs. It skips vendor, node_modules, dotfiles, and test files.
func ScanTier1(root string) (*Tier1Result, error) {
	result := &Tier1Result{}
	seenPkgs := map[string]bool{}
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// Skip test files.
		if strings.HasSuffix(path, "_test.go") {
			return nil
		}

		f, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil //nolint:nilerr // skip unparseable files gracefully
		}

		pkgName := f.Name.Name
		relDir, relErr := filepath.Rel(root, filepath.Dir(path))
		if relErr != nil {
			return fmt.Errorf("scanner: relative path for %s: %w", path, relErr)
		}
		pkgKey := relDir + ":" + pkgName
		if !seenPkgs[pkgKey] {
			seenPkgs[pkgKey] = true
			result.Packages = append(result.Packages, PackageInfo{
				Name: pkgName,
				Path: relDir,
			})
		}

		for _, decl := range f.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				switch t := ts.Type.(type) {
				case *ast.InterfaceType:
					iface := InterfaceInfo{
						Name:    ts.Name.Name,
						Package: pkgName,
					}
					if t.Methods != nil {
						for _, m := range t.Methods.List {
							for _, name := range m.Names {
								iface.Methods = append(iface.Methods, name.Name)
							}
						}
					}
					result.Interfaces = append(result.Interfaces, iface)
				case *ast.StructType:
					st := StructInfo{
						Name:    ts.Name.Name,
						Package: pkgName,
					}
					if t.Fields != nil {
						for _, field := range t.Fields.List {
							for _, name := range field.Names {
								st.Fields = append(st.Fields, name.Name)
							}
						}
					}
					result.Structs = append(result.Structs, st)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanner: tier1 walk: %w", err)
	}
	return result, nil
}
