package config

import (
	"runtime"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	// With nothing set, every field should fall back to its default. t.Setenv
	// is not used here; we explicitly clear to guard against an inherited env.
	for _, k := range []string{"PORT", "MAX_FILE_SIZE_MB", "WORKER_COUNT", "JOB_TTL_MINUTES"} {
		t.Setenv(k, "")
	}

	cfg := Load()

	if cfg.Port != defaultPort {
		t.Errorf("Port = %q, want %q", cfg.Port, defaultPort)
	}
	if want := int64(defaultMaxFileMB) << 20; cfg.MaxFileBytes != want {
		t.Errorf("MaxFileBytes = %d, want %d", cfg.MaxFileBytes, want)
	}
	if cfg.MaxUploadBytes != cfg.MaxFileBytes+uploadHeadroomBytes {
		t.Errorf("MaxUploadBytes = %d, want %d", cfg.MaxUploadBytes, cfg.MaxFileBytes+uploadHeadroomBytes)
	}
	if cfg.Workers != runtime.NumCPU() {
		t.Errorf("Workers = %d, want %d", cfg.Workers, runtime.NumCPU())
	}
	if cfg.JobTTL != defaultJobTTLMin*time.Minute {
		t.Errorf("JobTTL = %s, want %s", cfg.JobTTL, defaultJobTTLMin*time.Minute)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("PORT", "8080")
	t.Setenv("MAX_FILE_SIZE_MB", "10")
	t.Setenv("WORKER_COUNT", "3")
	t.Setenv("JOB_TTL_MINUTES", "1")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want 8080", cfg.Port)
	}
	if want := int64(10) << 20; cfg.MaxFileBytes != want {
		t.Errorf("MaxFileBytes = %d, want %d", cfg.MaxFileBytes, want)
	}
	if cfg.Workers != 3 {
		t.Errorf("Workers = %d, want 3", cfg.Workers)
	}
	if cfg.JobTTL != time.Minute {
		t.Errorf("JobTTL = %s, want 1m", cfg.JobTTL)
	}
}

func TestLoadInvalidFallsBackToDefault(t *testing.T) {
	// Non-numeric and non-positive values must not break startup; they fall back.
	t.Setenv("MAX_FILE_SIZE_MB", "not-a-number")
	t.Setenv("WORKER_COUNT", "0")
	t.Setenv("JOB_TTL_MINUTES", "-5")

	cfg := Load()

	if want := int64(defaultMaxFileMB) << 20; cfg.MaxFileBytes != want {
		t.Errorf("MaxFileBytes = %d, want default %d", cfg.MaxFileBytes, want)
	}
	if cfg.Workers != runtime.NumCPU() {
		t.Errorf("Workers = %d, want default %d", cfg.Workers, runtime.NumCPU())
	}
	if cfg.JobTTL != defaultJobTTLMin*time.Minute {
		t.Errorf("JobTTL = %s, want default %s", cfg.JobTTL, defaultJobTTLMin*time.Minute)
	}
}
