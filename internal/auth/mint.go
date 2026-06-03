// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package auth provides identity resolution and authentication helpers for SpecGraph.
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters for minting new API key hashes.
// MUST match the values the resolver's argon2idVerify reads from the PHC string.
// The resolver is parameter-agnostic (reads m/t/p from the PHC), so these
// values determine the work factor baked into every new key. Chosen to match
// the e2e seed credential (m=19456,t=2,p=1) confirming round-trip verifiability.
const (
	mintArgonTime    uint32 = 2
	mintArgonMemory  uint32 = 19456
	mintArgonThreads uint8  = 1
	mintArgonKeyLen  uint32 = 32
	mintArgonSaltLen        = 16
)

// GenerateAPIKeySecret generates a fresh API key secret and its argon2id PHC hash.
// Returns (secret, phcHash, err).
//
// secret is apiKeySecretLen hex characters derived from apiKeySecretLen/2 random bytes.
// phcHash is a PHC-encoded argon2id string:
//
//	$argon2id$v=19$m=<m>,t=<t>,p=<p>$<RawStdEncoding salt>$<RawStdEncoding hash>
//
// The returned secret is NEVER persisted; only phcHash goes to storage.
func GenerateAPIKeySecret() (secret, phcHash string, err error) {
	// Generate apiKeySecretLen/2 random bytes → hex-encode to apiKeySecretLen chars.
	rawSecret := make([]byte, apiKeySecretLen/2)
	if _, err = rand.Read(rawSecret); err != nil {
		return "", "", fmt.Errorf("auth: generate api key secret: rand read: %w", err)
	}
	secret = hex.EncodeToString(rawSecret)

	// Generate random salt.
	salt := make([]byte, mintArgonSaltLen)
	if _, err = rand.Read(salt); err != nil {
		return "", "", fmt.Errorf("auth: generate api key secret: rand salt: %w", err)
	}

	// Derive the key hash.
	hash := argon2.IDKey([]byte(secret), salt, mintArgonTime, mintArgonMemory, mintArgonThreads, mintArgonKeyLen)

	// Encode as PHC string using RawStdEncoding (no padding) to match the verifier.
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	phcHash = fmt.Sprintf(
		"$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		mintArgonMemory, mintArgonTime, mintArgonThreads,
		b64Salt, b64Hash,
	)

	return secret, phcHash, nil
}

// FormatAPIKeyToken assembles the full bearer token from the storage-assigned
// prefix and the plaintext secret. The token format is:
//
//	<apiKeyPrefix><prefix>_<secret>
//
// prefix comes from storage.APIKey.Prefix (assigned by CreateAPIKey/RotateAPIKey).
// secret is the value returned by GenerateAPIKeySecret.
func FormatAPIKeyToken(prefix, secret string) string {
	return apiKeyPrefix + prefix + "_" + secret
}

// APIKeyTokenPrefix returns the constant prefix that all API key tokens begin with.
// Useful for callers that need to validate token shape without importing internals.
func APIKeyTokenPrefix() string {
	return apiKeyPrefix
}
