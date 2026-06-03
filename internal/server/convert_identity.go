// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// tsOrNil converts a *time.Time to a proto Timestamp, returning nil for nil.
func tsOrNil(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

// userKindToProto maps the storage Kind discriminator to the proto enum.
func userKindToProto(k storage.Kind) (specv1.UserKind, error) {
	switch k {
	case storage.KindHuman:
		return specv1.UserKind_USER_KIND_HUMAN, nil
	case storage.KindServiceAccount:
		return specv1.UserKind_USER_KIND_SERVICE_ACCOUNT, nil
	default:
		return specv1.UserKind_USER_KIND_UNSPECIFIED, fmt.Errorf("unknown user kind %q", k)
	}
}

// userToProto converts a storage.User to its proto representation.
func userToProto(u *storage.User) (*specv1.User, error) {
	kind, err := userKindToProto(u.Kind)
	if err != nil {
		return nil, err
	}
	return &specv1.User{
		Id:          u.ID,
		Kind:        kind,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		Role:        u.Role,
		OwnerUserId: u.OwnerUserID,
		Bootstrap:   u.Bootstrap,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		DeletedAt:   tsOrNil(u.DeletedAt),
	}, nil
}

// usersToProto converts a slice, propagating the first conversion error.
func usersToProto(us []*storage.User) ([]*specv1.User, error) {
	out := make([]*specv1.User, 0, len(us))
	for _, u := range us {
		pb, err := userToProto(u)
		if err != nil {
			return nil, err
		}
		out = append(out, pb)
	}
	return out, nil
}

// apiKeyToProto converts a storage.APIKey, deliberately omitting PHCHash.
func apiKeyToProto(k *storage.APIKey) *specv1.APIKey {
	return &specv1.APIKey{
		Id:            k.ID,
		UserId:        k.UserID,
		Prefix:        k.Prefix,
		RoleDowngrade: k.RoleDowngrade,
		Label:         k.Label,
		ExpiresAt:     tsOrNil(k.ExpiresAt),
		LastUsedAt:    tsOrNil(k.LastUsedAt),
		RevokedAt:     tsOrNil(k.RevokedAt),
		CreatedAt:     timestamppb.New(k.CreatedAt),
	}
}

func apiKeysToProto(ks []*storage.APIKey) []*specv1.APIKey {
	out := make([]*specv1.APIKey, 0, len(ks))
	for _, k := range ks {
		out = append(out, apiKeyToProto(k))
	}
	return out
}

func oidcBindingToProto(b *storage.OIDCBinding) *specv1.OIDCBinding {
	return &specv1.OIDCBinding{
		Id:          b.ID,
		UserId:      b.UserID,
		Issuer:      b.Issuer,
		Subject:     b.Subject,
		EmailAtBind: b.EmailAtBind,
		CreatedAt:   timestamppb.New(b.CreatedAt),
	}
}

func oidcBindingsToProto(bs []*storage.OIDCBinding) []*specv1.OIDCBinding {
	out := make([]*specv1.OIDCBinding, 0, len(bs))
	for _, b := range bs {
		out = append(out, oidcBindingToProto(b))
	}
	return out
}

// userKindFromProto maps the proto enum to the storage Kind (for filters).
// UNSPECIFIED maps to empty Kind (= "all kinds" in ListUsersFilter).
func userKindFromProto(k specv1.UserKind) storage.Kind {
	switch k {
	case specv1.UserKind_USER_KIND_HUMAN:
		return storage.KindHuman
	case specv1.UserKind_USER_KIND_SERVICE_ACCOUNT:
		return storage.KindServiceAccount
	default:
		return ""
	}
}
