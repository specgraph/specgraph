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

	// readyCh signals that g1 has acquired the lock. g2 waits on the channel
	// before attempting its own acquire — this replaces a brittle time.Sleep
	// with a deterministic happens-before edge so the test exercises true
	// contention rather than relying on goroutine scheduling timing.
	readyCh := make(chan struct{})

	go func() {
		defer wg.Done()
		unlock, err := acquireFileLock(target)
		if err != nil {
			close(readyCh) // unblock g2 even on failure so it doesn't deadlock
			t.Errorf("g1 acquire: %v", err)
			return
		}
		close(readyCh) // g1 holds the lock; g2 may now contend
		time.Sleep(50 * time.Millisecond)
		firstReleased.Store(time.Now().UnixNano())
		_ = unlock()
	}()

	go func() {
		defer wg.Done()
		<-readyCh // wait until g1 holds the lock before contending
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
