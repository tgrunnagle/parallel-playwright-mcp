package shutdown

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.DrainTimeout != 30*time.Second {
		t.Errorf("expected DrainTimeout 30s, got %v", cfg.DrainTimeout)
	}
	if cfg.PhaseTimeout != 30*time.Second {
		t.Errorf("expected PhaseTimeout 30s, got %v", cfg.PhaseTimeout)
	}
	if cfg.TotalTimeout != 90*time.Second {
		t.Errorf("expected TotalTimeout 90s, got %v", cfg.TotalTimeout)
	}
}

func TestConfigWithDrainTimeout(t *testing.T) {
	cfg := DefaultConfig().WithDrainTimeout(10 * time.Second)

	if cfg.DrainTimeout != 10*time.Second {
		t.Errorf("expected DrainTimeout 10s, got %v", cfg.DrainTimeout)
	}
	// Other values should remain at default
	if cfg.PhaseTimeout != 30*time.Second {
		t.Errorf("expected PhaseTimeout 30s, got %v", cfg.PhaseTimeout)
	}
}

func TestConfigWithPhaseTimeout(t *testing.T) {
	cfg := DefaultConfig().WithPhaseTimeout(15 * time.Second)

	if cfg.PhaseTimeout != 15*time.Second {
		t.Errorf("expected PhaseTimeout 15s, got %v", cfg.PhaseTimeout)
	}
	// Other values should remain at default
	if cfg.DrainTimeout != 30*time.Second {
		t.Errorf("expected DrainTimeout 30s, got %v", cfg.DrainTimeout)
	}
}

func TestConfigWithTotalTimeout(t *testing.T) {
	cfg := DefaultConfig().WithTotalTimeout(120 * time.Second)

	if cfg.TotalTimeout != 120*time.Second {
		t.Errorf("expected TotalTimeout 120s, got %v", cfg.TotalTimeout)
	}
}

func TestConfigChaining(t *testing.T) {
	cfg := DefaultConfig().
		WithDrainTimeout(5 * time.Second).
		WithPhaseTimeout(10 * time.Second).
		WithTotalTimeout(30 * time.Second)

	if cfg.DrainTimeout != 5*time.Second {
		t.Errorf("expected DrainTimeout 5s, got %v", cfg.DrainTimeout)
	}
	if cfg.PhaseTimeout != 10*time.Second {
		t.Errorf("expected PhaseTimeout 10s, got %v", cfg.PhaseTimeout)
	}
	if cfg.TotalTimeout != 30*time.Second {
		t.Errorf("expected TotalTimeout 30s, got %v", cfg.TotalTimeout)
	}
}
