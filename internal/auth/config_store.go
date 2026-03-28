// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"strings"

	"github.com/specgraph/specgraph/internal/config"
)

// DefaultRolePermissions defines the built-in role permission bundles.
var DefaultRolePermissions = map[Role][]string{
	RoleReader: {"*:read"},
	RoleWriter: {"*:read", "*:write"},
	RoleAdmin:  {"*:*"},
}

// ConfigStore implements IdentityStore backed by static config file entries.
type ConfigStore struct {
	identities map[string]*Identity
	hasKeys    bool
}

// NewConfigStore builds a ConfigStore from the auth section of the config file.
// It validates that all key IDs and values are unique and that referenced roles exist.
// credentialsPath is reserved for future credential bootstrap (Task 7) and is unused here.
func NewConfigStore(cfg config.AuthConfig, credentialsPath string) (*ConfigStore, error) {
	_ = credentialsPath
	roles := make(map[string][]string)
	for role, perms := range DefaultRolePermissions {
		roles[string(role)] = perms
	}
	for name, rc := range cfg.Roles {
		roles[name] = rc.Permissions
	}

	identities := make(map[string]*Identity, len(cfg.APIKeys))
	seenIDs := make(map[string]bool, len(cfg.APIKeys))

	for _, ak := range cfg.APIKeys {
		if strings.TrimSpace(ak.ID) == "" {
			return nil, fmt.Errorf("API key ID must not be empty")
		}
		if strings.TrimSpace(ak.Key) == "" {
			return nil, fmt.Errorf("API key %s must have a non-empty value", ak.ID)
		}
		if seenIDs[ak.ID] {
			return nil, fmt.Errorf("duplicate API key ID: %s", ak.ID)
		}
		seenIDs[ak.ID] = true
		if _, exists := identities[ak.Key]; exists {
			return nil, fmt.Errorf("duplicate API key value for IDs: %s", ak.ID)
		}
		perms, ok := roles[ak.Role]
		if !ok {
			return nil, fmt.Errorf("API key %s references unknown role: %s", ak.ID, ak.Role)
		}
		permMap := make(map[string]bool, len(perms))
		for _, p := range perms {
			permMap[p] = true
		}
		identities[ak.Key] = &Identity{
			Subject:     "apikey:" + ak.ID,
			DisplayName: ak.Name,
			Role:        Role(ak.Role),
			Permissions: permMap,
			Source:      "apikey",
		}
	}

	return &ConfigStore{
		identities: identities,
		hasKeys:    len(identities) > 0,
	}, nil
}

// fixedLenCompare compares two strings in constant time regardless of length.
// It hashes both values with HMAC-SHA256 so the inputs to ConstantTimeCompare
// are always 32 bytes, eliminating the length side-channel.
func fixedLenCompare(a, b string) bool {
	mac := hmac.New(sha256.New, []byte("specgraph-key-compare"))
	mac.Write([]byte(a))
	ha := mac.Sum(nil)
	mac.Reset()
	mac.Write([]byte(b))
	hb := mac.Sum(nil)
	return subtle.ConstantTimeCompare(ha, hb) == 1
}

// ResolveAPIKey looks up the identity for the given API key using constant-time comparison.
// Always iterates all keys to prevent timing side-channels from leaking which slot matched.
func (s *ConfigStore) ResolveAPIKey(_ context.Context, key string) (*Identity, error) {
	var matched *Identity
	for storedKey, id := range s.identities {
		if fixedLenCompare(storedKey, key) {
			matched = id
		}
	}
	if matched != nil {
		return matched, nil
	}
	return nil, ErrUnknownKey
}

// HasKeys reports whether any API keys are configured.
func (s *ConfigStore) HasKeys() bool {
	return s.hasKeys
}

// ResolveJWT is a no-op for ConfigStore — it doesn't support OIDC.
func (s *ConfigStore) ResolveJWT(_ context.Context, _ string) (*Identity, error) {
	return nil, ErrNoOIDC
}

// HasAuth reports whether any API keys are configured.
func (s *ConfigStore) HasAuth() bool {
	return s.hasKeys
}

// AllowUnauthenticated returns true when no API keys are configured,
// preserving the existing local-identity fallback behavior.
func (s *ConfigStore) AllowUnauthenticated() bool {
	return !s.hasKeys
}
