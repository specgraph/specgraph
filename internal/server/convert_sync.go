// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package server

import (
	"fmt"

	specv1 "github.com/specgraph/specgraph/gen/specgraph/v1"
	"github.com/specgraph/specgraph/internal/storage"
)

// --- Sync converters ---

func syncMappingToProto(m *storage.SyncMapping) (*specv1.SyncMapping, error) {
	state, err := syncStateToProto(m.State)
	if err != nil {
		return nil, err
	}
	adapter, err := syncAdapterToProto(m.Adapter)
	if err != nil {
		return nil, err
	}
	return &specv1.SyncMapping{
		SpecId:       m.SpecID,
		SpecSlug:     m.SpecSlug,
		Adapter:      adapter,
		ExternalId:   m.ExternalID,
		State:        state,
		ErrorMessage: m.ErrorMessage,
		LastSync:     timeToProto(m.LastSync),
		CreatedAt:    timeToProto(m.CreatedAt),
	}, nil
}

func syncAdapterToProto(a storage.SyncAdapterType) (specv1.SyncAdapter, error) {
	switch a {
	case storage.SyncAdapterBeads:
		return specv1.SyncAdapter_SYNC_ADAPTER_BEADS, nil
	case storage.SyncAdapterGitHub:
		return specv1.SyncAdapter_SYNC_ADAPTER_GITHUB, nil
	default:
		return specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED, fmt.Errorf("unknown sync adapter: %v", a)
	}
}

func syncAdapterFromProto(a specv1.SyncAdapter) (storage.SyncAdapterType, error) {
	switch a {
	case specv1.SyncAdapter_SYNC_ADAPTER_UNSPECIFIED:
		return "", nil
	case specv1.SyncAdapter_SYNC_ADAPTER_BEADS:
		return storage.SyncAdapterBeads, nil
	case specv1.SyncAdapter_SYNC_ADAPTER_GITHUB:
		return storage.SyncAdapterGitHub, nil
	default:
		return "", fmt.Errorf("unknown sync adapter: %v", a)
	}
}

func syncStateToProto(s storage.SyncStateType) (specv1.SyncState, error) {
	switch s {
	case storage.SyncStatePending:
		return specv1.SyncState_SYNC_STATE_PENDING, nil
	case storage.SyncStateSynced:
		return specv1.SyncState_SYNC_STATE_SYNCED, nil
	case storage.SyncStateConflict:
		return specv1.SyncState_SYNC_STATE_CONFLICT, nil
	case storage.SyncStateError:
		return specv1.SyncState_SYNC_STATE_ERROR, nil
	default:
		return specv1.SyncState_SYNC_STATE_UNSPECIFIED, fmt.Errorf("unknown sync state: %v", s)
	}
}
