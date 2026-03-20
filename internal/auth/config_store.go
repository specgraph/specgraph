// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package auth

import (
	"context"
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
func NewConfigStore(cfg config.AuthConfig) (*ConfigStore, error) {
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

// ResolveAPIKey looks up the identity for the given API key using constant-time comparison.
// Always iterates all keys to prevent timing side-channels from leaking which slot matched.
func (s *ConfigStore) ResolveAPIKey(_ context.Context, key string) (*Identity, error) {
	var matched *Identity
	for storedKey, id := range s.identities {
		if subtle.ConstantTimeCompare([]byte(storedKey), []byte(key)) == 1 {
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
