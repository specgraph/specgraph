// SPDX-License-Identifier: MIT
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var validSlugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9_/-]*[a-z0-9])?$`)

const maxSlugLength = 256

// maxFieldLen caps free-text RPC fields to prevent unbounded writes to graph storage.
const maxFieldLen = 10000

// validateRequiredField checks that a field is non-empty and within maxFieldLen.
func validateRequiredField(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > maxFieldLen {
		return fmt.Errorf("%s exceeds maximum length of %d characters", name, maxFieldLen)
	}
	return nil
}

func validateSlug(slug string) error {
	if slug == "" {
		return errors.New("slug is required")
	}
	if len(slug) > maxSlugLength {
		return fmt.Errorf("slug exceeds maximum length of %d characters", maxSlugLength)
	}
	if strings.Contains(slug, "..") {
		return errors.New("slug contains path traversal")
	}
	if !validSlugRe.MatchString(slug) {
		return errors.New("slug contains invalid characters")
	}
	return nil
}
