package shutdown

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/browser"
	"github.com/tgrunnagle/parallel-playwright-mcp/pkg/session"
)

// mockBrowserPool implements browser.BrowserPool for testing
type mockBrowserPool struct {
	mu            sync.Mutex
	stopCalled    bool
	stopErr       error
	stopDelay     time.Duration
	running       bool
	startErr      error
	newContextErr error
}

func newMockBrowserPool() *mockBrowserPool {
	return &mockBrowserPool{running: true}
}

func (m *mockBrowserPool) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return m.startErr
	}
	m.running = true
	return nil
}

func (m *mockBrowserPool) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopDelay > 0 {
		select {
		case <-time.After(m.stopDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.stopCalled = true
	m.running = false
	return m.stopErr
}

func (m *mockBrowserPool) NewContext(_ context.Context, _ browser.BrowserType, _ browser.ContextOptions) (playwright.BrowserContext, error) {
	if m.newContextErr != nil {
		return nil, m.newContextErr
	}
	return nil, nil
}

func (m *mockBrowserPool) CloseContext(_ context.Context, _ playwright.BrowserContext) error {
	return nil
}

func (m *mockBrowserPool) Stats() browser.PoolStats {
	return browser.PoolStats{
		Browsers: make(map[browser.BrowserType]browser.BrowserStats),
	}
}

func (m *mockBrowserPool) wasStopCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopCalled
}

// mockSessionManager implements session.BrowserSessionManager for testing
type mockSessionManager struct {
	mu                   sync.Mutex
	closeAllForMCPCalled map[string]bool
	closeAllForMCPErr    error
	closeAllForMCPDelay  time.Duration
	closeAllCalled       bool
	closeAllErr          error
	createSessionErr     error
	closeSessionErr      error
	sessions             map[string]*session.BrowserSession
}

func newMockSessionManager() *mockSessionManager {
	return &mockSessionManager{
		closeAllForMCPCalled: make(map[string]bool),
		sessions:             make(map[string]*session.BrowserSession),
	}
}

func (m *mockSessionManager) CreateSession(_ context.Context, _ string, _ session.SessionOptions) (*session.BrowserSession, error) {
	if m.createSessionErr != nil {
		return nil, m.createSessionErr
	}
	return nil, nil
}

func (m *mockSessionManager) GetSession(_, browserSessionID string) (*session.BrowserSession, bool) {
	s, ok := m.sessions[browserSessionID]
	return s, ok
}

func (m *mockSessionManager) CloseSession(_ context.Context, _, _ string) error {
	return m.closeSessionErr
}

func (m *mockSessionManager) ListSessions(_ string) []*session.SessionInfo {
	return nil
}

func (m *mockSessionManager) CloseAllForMCP(ctx context.Context, mcpSessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closeAllForMCPDelay > 0 {
		select {
		case <-time.After(m.closeAllForMCPDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	m.closeAllForMCPCalled[mcpSessionID] = true
	return m.closeAllForMCPErr
}

func (m *mockSessionManager) CloseAll(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closeAllCalled = true
	return m.closeAllErr
}

func (m *mockSessionManager) wasCloseAllCalled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeAllCalled
}

func (m *mockSessionManager) Cleanup(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}

func (m *mockSessionManager) wasCloseAllForMCPCalled(mcpSessionID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeAllForMCPCalled[mcpSessionID]
}

func TestNewCoordinator(t *testing.T) {
	pool := newMockBrowserPool()
	mgr := newMockSessionManager()
	tracker := NewRequestTracker()

	coord := NewCoordinator(
		DefaultConfig(),
		nil,
		mgr,
		pool,
		tracker,
	)

	if coord == nil {
		t.Fatal("expected non-nil coordinator")
	}
	if coord.RequestTracker() != tracker {
		t.Error("expected coordinator to use provided tracker")
	}
}

func TestNewCoordinator_NilTracker(t *testing.T) {
	pool := newMockBrowserPool()
	mgr := newMockSessionManager()

	coord := NewCoordinator(
		DefaultConfig(),
		nil,
		mgr,
		pool,
		nil,
	)

	if coord.RequestTracker() == nil {
		t.Error("expected coordinator to create tracker when nil provided")
	}
}

func TestCoordinator_RegisterMCPSession(t *testing.T) {
	coord := NewCoordinator(DefaultConfig(), nil, nil, nil, nil)

	coord.RegisterMCPSession("mcp-1")
	coord.RegisterMCPSession("mcp-2")

	if count := coord.ActiveMCPSessionCount(); count != 2 {
		t.Errorf("expected 2 sessions, got %d", count)
	}
}

func TestCoordinator_UnregisterMCPSession(t *testing.T) {
	coord := NewCoordinator(DefaultConfig(), nil, nil, nil, nil)

	coord.RegisterMCPSession("mcp-1")
	coord.RegisterMCPSession("mcp-2")
	coord.UnregisterMCPSession("mcp-1")

	if count := coord.ActiveMCPSessionCount(); count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

func TestCoordinator_UnregisterMCPSession_NonExistent(t *testing.T) {
	coord := NewCoordinator(DefaultConfig(), nil, nil, nil, nil)

	coord.RegisterMCPSession("mcp-1")
	coord.UnregisterMCPSession("mcp-nonexistent")

	if count := coord.ActiveMCPSessionCount(); count != 1 {
		t.Errorf("expected 1 session, got %d", count)
	}
}

func TestCoordinator_Shutdown_StopsBrowserPool(t *testing.T) {
	pool := newMockBrowserPool()
	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		nil,
		pool,
		nil,
	)

	ctx := context.Background()
	err := coord.Shutdown(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !pool.wasStopCalled() {
		t.Error("expected browser pool Stop() to be called")
	}
}

func TestCoordinator_Shutdown_ClosesAllSessions(t *testing.T) {
	mgr := newMockSessionManager()
	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		mgr,
		nil,
		nil,
	)

	ctx := context.Background()
	err := coord.Shutdown(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !mgr.wasCloseAllCalled() {
		t.Error("expected CloseAll to be called")
	}
}

func TestCoordinator_Shutdown_ExecutesPhasesInOrder(t *testing.T) {
	// Use ordered mocks that track call order
	pool := &orderedMockPool{order: new([]string), mu: new(sync.Mutex)}
	mgr := &orderedMockSessionManager{order: pool.order, mu: pool.mu}

	coord := NewCoordinator(
		DefaultConfig().WithDrainTimeout(10*time.Millisecond).WithPhaseTimeout(100*time.Millisecond),
		nil, // No HTTP server for this test
		mgr,
		pool,
		nil,
	)
	coord.RegisterMCPSession("mcp-1")

	ctx := context.Background()
	coord.Shutdown(ctx)

	pool.mu.Lock()
	order := *pool.order
	pool.mu.Unlock()

	// Sessions (phase 2) should happen before pool (phase 3)
	if len(order) < 2 {
		t.Fatalf("expected at least 2 phases, got %d: %v", len(order), order)
	}

	sessionsIdx := -1
	poolIdx := -1
	for i, phase := range order {
		if phase == "sessions" && sessionsIdx == -1 {
			sessionsIdx = i
		}
		if phase == "pool" && poolIdx == -1 {
			poolIdx = i
		}
	}

	if sessionsIdx == -1 {
		t.Error("expected sessions phase to be recorded")
	}
	if poolIdx == -1 {
		t.Error("expected pool phase to be recorded")
	}
	if sessionsIdx > poolIdx {
		t.Errorf("expected sessions to be closed before pool, got order: %v", order)
	}
}

// orderedMockPool tracks the order of Stop calls
type orderedMockPool struct {
	order *[]string
	mu    *sync.Mutex
}

func (m *orderedMockPool) Start(_ context.Context) error { return nil }
func (m *orderedMockPool) Stop(_ context.Context) error {
	m.mu.Lock()
	*m.order = append(*m.order, "pool")
	m.mu.Unlock()
	return nil
}
func (m *orderedMockPool) NewContext(_ context.Context, _ browser.BrowserType, _ browser.ContextOptions) (playwright.BrowserContext, error) {
	return nil, nil
}
func (m *orderedMockPool) CloseContext(_ context.Context, _ playwright.BrowserContext) error {
	return nil
}
func (m *orderedMockPool) Stats() browser.PoolStats {
	return browser.PoolStats{Browsers: make(map[browser.BrowserType]browser.BrowserStats)}
}

// orderedMockSessionManager tracks the order of CloseAll calls
type orderedMockSessionManager struct {
	order *[]string
	mu    *sync.Mutex
}

func (m *orderedMockSessionManager) CreateSession(_ context.Context, _ string, _ session.SessionOptions) (*session.BrowserSession, error) {
	return nil, nil
}
func (m *orderedMockSessionManager) GetSession(_, _ string) (*session.BrowserSession, bool) {
	return nil, false
}
func (m *orderedMockSessionManager) CloseSession(_ context.Context, _, _ string) error { return nil }
func (m *orderedMockSessionManager) ListSessions(_ string) []*session.SessionInfo      { return nil }
func (m *orderedMockSessionManager) CloseAllForMCP(_ context.Context, _ string) error {
	return nil
}
func (m *orderedMockSessionManager) CloseAll(_ context.Context) error {
	m.mu.Lock()
	*m.order = append(*m.order, "sessions")
	m.mu.Unlock()
	return nil
}
func (m *orderedMockSessionManager) Cleanup(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}

func TestCoordinator_Shutdown_ContinuesOnError(t *testing.T) {
	mgr := newMockSessionManager()
	mgr.closeAllErr = errors.New("session close error")

	pool := newMockBrowserPool()

	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		mgr,
		pool,
		nil,
	)

	ctx := context.Background()
	err := coord.Shutdown(ctx)

	// Should return error but continue to next phase
	if err == nil {
		t.Error("expected error from session close failure")
	}
	if !pool.wasStopCalled() {
		t.Error("expected pool Stop() to be called even after session error")
	}
}

func TestCoordinator_Shutdown_ReturnsAggregatedErrors(t *testing.T) {
	mgr := newMockSessionManager()
	mgr.closeAllErr = errors.New("session error")

	pool := newMockBrowserPool()
	pool.stopErr = errors.New("pool error")

	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		mgr,
		pool,
		nil,
	)

	ctx := context.Background()
	err := coord.Shutdown(ctx)

	if err == nil {
		t.Fatal("expected error")
	}
	errStr := err.Error()
	if !errors.Is(err, mgr.closeAllErr) && !strings.Contains(errStr, "session") {
		t.Error("expected error to contain session error")
	}
}

func TestCoordinator_Shutdown_Idempotent(t *testing.T) {
	pool := newMockBrowserPool()
	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		nil,
		pool,
		nil,
	)

	ctx := context.Background()
	err1 := coord.Shutdown(ctx)
	err2 := coord.Shutdown(ctx)

	if err1 != err2 {
		t.Errorf("expected same error on repeated shutdown, got %v and %v", err1, err2)
	}
}

func TestCoordinator_Shutdown_NoMCPSessions(t *testing.T) {
	pool := newMockBrowserPool()
	mgr := newMockSessionManager()

	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil,
		mgr,
		pool,
		nil,
	)

	ctx := context.Background()
	err := coord.Shutdown(ctx)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if pool.wasStopCalled() != true {
		t.Error("expected pool to be stopped even with no sessions")
	}
}

func TestCoordinator_Shutdown_NilDependencies(t *testing.T) {
	coord := NewCoordinator(
		DefaultConfig().WithPhaseTimeout(100*time.Millisecond),
		nil, // nil http server
		nil, // nil session manager
		nil, // nil browser pool
		nil,
	)

	ctx := context.Background()
	err := coord.Shutdown(ctx)

	if err != nil {
		t.Errorf("unexpected error with nil dependencies: %v", err)
	}
}
