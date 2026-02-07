package shutdown

import (
	"testing"
	"time"
)

func TestNewEmergencyCleanup(t *testing.T) {
	pool := newMockBrowserPool()
	cleanup := NewEmergencyCleanup(nil, pool)

	if cleanup == nil {
		t.Fatal("expected non-nil cleanup")
	}
	if cleanup.timeout != defaultEmergencyTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultEmergencyTimeout, cleanup.timeout)
	}
}

func TestEmergencyCleanup_WithTimeout(t *testing.T) {
	pool := newMockBrowserPool()
	cleanup := NewEmergencyCleanup(nil, pool).WithTimeout(5 * time.Second)

	if cleanup.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", cleanup.timeout)
	}
}

func TestEmergencyCleanup_Execute_UsesCoordinator(t *testing.T) {
	pool := newMockBrowserPool()
	mgr := newMockSessionManager()

	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		mgr,
		pool,
		nil,
	)

	cleanup := NewEmergencyCleanup(coord, pool)
	cleanup.Execute("test panic")

	// When coordinator is available, it should be used for shutdown
	if !pool.wasStopCalled() {
		t.Error("expected pool.Stop to be called via coordinator")
	}
}

func TestEmergencyCleanup_Execute_UsesBrowserPoolDirectly(t *testing.T) {
	pool := newMockBrowserPool()
	cleanup := NewEmergencyCleanup(nil, pool)

	cleanup.Execute("test panic")

	if !pool.wasStopCalled() {
		t.Error("expected pool.Stop to be called directly")
	}
}

func TestEmergencyCleanup_Execute_NoResources(t *testing.T) {
	cleanup := NewEmergencyCleanup(nil, nil)

	// Should not panic even with no resources
	cleanup.Execute("test panic")
}

func TestEmergencyCleanup_RecoverAndCleanup_NoPanic(t *testing.T) {
	pool := newMockBrowserPool()
	cleanup := NewEmergencyCleanup(nil, pool)

	func() {
		defer cleanup.RecoverAndCleanup()
		// No panic
	}()

	if pool.wasStopCalled() {
		t.Error("expected pool.Stop not to be called when no panic")
	}
}

func TestEmergencyCleanup_RecoverAndCleanup_WithPanic(t *testing.T) {
	pool := newMockBrowserPool()
	cleanup := NewEmergencyCleanup(nil, pool)

	// RecoverAndCleanup must be called directly as the deferred function
	// for recover() to work properly
	func() {
		defer cleanup.RecoverAndCleanup()
		panic("test panic")
	}()

	// After RecoverAndCleanup catches the panic, pool should be stopped
	if !pool.wasStopCalled() {
		t.Error("expected pool.Stop to be called on panic")
	}
}

func TestEmergencyCleanup_Execute_RespectsTimeout(t *testing.T) {
	pool := newMockBrowserPool()
	pool.stopDelay = 1 * time.Second // Slow stop

	cleanup := NewEmergencyCleanup(nil, pool).WithTimeout(50 * time.Millisecond)

	start := time.Now()
	cleanup.Execute("test panic")
	elapsed := time.Since(start)

	// Should timeout before 1 second
	if elapsed >= 500*time.Millisecond {
		t.Errorf("expected timeout to be respected, took %v", elapsed)
	}
}
