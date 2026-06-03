// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package credentials manages the multi-server CLI credentials file
// (~/.config/specgraph/credentials.yaml), keyed by server base URL. Each
// server entry carries a bearer token and an optional human-friendly label.
package credentials

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// fileHeader is prepended to the credentials file on Save.
const fileHeader = "# SpecGraph CLI credentials. Managed by `specgraph login`.\n" +
	"# Keyed by server base URL. Tokens are bearer credentials; keep this file private.\n"

// ServerCreds holds the credentials for a single SpecGraph server.
type ServerCreds struct {
	Token string `yaml:"token"`
	Label string `yaml:"label,omitempty"`
}

// File is the on-disk multi-server credentials document.
type File struct {
	Servers map[string]ServerCreds `yaml:"servers"`
}

// normalize canonicalizes a server URL for use as a map key by trimming any
// trailing slashes.
func normalize(serverURL string) string {
	return strings.TrimRight(serverURL, "/")
}

// TokenFor returns the token stored for the given server URL, or "" if none.
// The URL is normalized so trailing-slash variants match the same entry.
func (f *File) TokenFor(serverURL string) string {
	if f == nil || f.Servers == nil {
		return ""
	}
	return f.Servers[normalize(serverURL)].Token
}

// Upsert inserts or replaces the credentials for a server URL, preserving any
// other server entries.
func (f *File) Upsert(serverURL string, creds ServerCreds) {
	if f.Servers == nil {
		f.Servers = make(map[string]ServerCreds)
	}
	f.Servers[normalize(serverURL)] = creds
}

// Load reads the credentials file at path. A missing file yields an empty File
// and a nil error. Unmarshaling is lenient: an old-shape file (e.g. one using
// the legacy `api_keys:` key) parses into a File with no servers.
func Load(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{}, nil
		}
		return nil, fmt.Errorf("read credentials file: %w", err)
	}

	var f File
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("parse credentials file: %w", err)
	}
	return &f, nil
}

// Save atomically writes the credentials file to path with 0600 permissions,
// creating parent directories (0700) as needed. It writes to a temporary file
// in the same directory and renames it into place.
func (f *File) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}

	body, err := yaml.Marshal(f)
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".credentials-*.yaml.tmp")
	if err != nil {
		return fmt.Errorf("create temp credentials file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() //nolint:errcheck // best-effort cleanup if rename succeeded

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close() //nolint:errcheck // already returning an error
		return fmt.Errorf("chmod temp credentials file: %w", err)
	}
	if _, err := tmp.WriteString(fileHeader); err != nil {
		_ = tmp.Close() //nolint:errcheck // already returning an error
		return fmt.Errorf("write credentials header: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close() //nolint:errcheck // already returning an error
		return fmt.Errorf("write credentials body: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp credentials file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename credentials file: %w", err)
	}
	return nil
}

// CheckPermissions returns a warning message if the credentials file at path is
// readable or writable by group or others (perm & 0o077 != 0). It returns "" if
// permissions are acceptable or the file does not exist.
func CheckPermissions(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return fmt.Sprintf(
			"warning: credentials file %s has permissions %04o; recommend 0600 (chmod 600 %q)",
			path, perm, path,
		)
	}
	return ""
}
