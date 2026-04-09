// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"connectrpc.com/connect"
)

var validSlugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9_/-]*[a-z0-9])?$`)

const maxSlugLength = 256

// maxFieldLen caps free-text RPC fields to prevent unbounded writes to graph storage.
const maxFieldLen = 10000

// maxNotesLen caps the notes field, which holds conversation summaries and
// can be significantly longer than individual fields like seed or scope items.
const maxNotesLen = 100000

// maxFindingsPerRequest caps the number of findings per StoreFindings call.
const maxFindingsPerRequest = 100

// maxFindingDetailLen caps the detail field on analytical findings (64 KB).
const maxFindingDetailLen = 65536

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

// validateStringSlice checks that a repeated string field doesn't exceed maxCount
// items and that no individual item exceeds maxItemLen bytes.
func validateStringSlice(name string, items []string, maxCount, maxItemLen int) error {
	if len(items) > maxCount {
		return connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("%s: too many items (%d > %d)", name, len(items), maxCount))
	}
	for i, item := range items {
		if len(item) > maxItemLen {
			return connect.NewError(connect.CodeInvalidArgument,
				fmt.Errorf("%s[%d]: too long (%d > %d)", name, i, len(item), maxItemLen))
		}
	}
	return nil
}

// validateOptionalField checks that a non-nil optional field does not exceed maxFieldLen.
func validateOptionalField(name string, value *string) error {
	if value != nil && len(*value) > maxFieldLen {
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
