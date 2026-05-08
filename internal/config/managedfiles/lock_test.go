// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package managedfiles

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireFileLock_BasicAcquireRelease(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "agents.md")
	if err := os.WriteFile(target, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireFileLock(target)
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if err := unlock(); err != nil {
		t.Errorf("unlock: %v", err)
	}
}

func TestAcquireFileLock_ContendedSerializes(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "agents.md")
	if err := os.WriteFile(target, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	var (
		firstReleased  atomic.Int64
		secondAcquired atomic.Int64
	)

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		unlock, err := acquireFileLock(target)
		if err != nil {
			t.Errorf("g1 acquire: %v", err)
			return
		}
		time.Sleep(50 * time.Millisecond)
		firstReleased.Store(time.Now().UnixNano())
		_ = unlock()
	}()

	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		unlock, err := acquireFileLock(target)
		if err != nil {
			t.Errorf("g2 acquire: %v", err)
			return
		}
		secondAcquired.Store(time.Now().UnixNano())
		_ = unlock()
	}()

	wg.Wait()
	if secondAcquired.Load() < firstReleased.Load() {
		t.Errorf("second acquired before first released: %d < %d",
			secondAcquired.Load(), firstReleased.Load())
	}
}

func TestAcquireFileLock_LockfileSurvivesUnlock(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "agents.md")
	if err := os.WriteFile(target, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}

	unlock, err := acquireFileLock(target)
	if err != nil {
		t.Fatal(err)
	}
	_ = unlock()

	if _, err := os.Stat(target + ".lock"); err != nil {
		t.Errorf("lock file should persist after unlock, got: %v", err)
	}
}
