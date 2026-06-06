// Package config centralizes the application's runtime configuration. It is
// tag-free (no libvips import) so it compiles and tests run in any build mode.
// All values come from environment variables with documented defaults; see
// README.md for the operator-facing table.
package config

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"time"
)

// Config holds the resolved runtime settings. Construct it with Load.
type Config struct {
	// Port is the TCP port the HTTP server listens on (PORT, default "3000").
	Port string

	// MaxFileBytes is the per-uploaded-file size cap (MAX_FILE_SIZE_MB,
	// default 50 MB), enforced in the upload handler.
	MaxFileBytes int64

	// MaxUploadBytes caps the whole multipart request body. It is derived from
	// MaxFileBytes plus headroom for multiple files and multipart boundaries, and
	// is used for Fiber's BodyLimit.
	MaxUploadBytes int64

	// Workers caps the number of concurrent libvips pipelines (WORKER_COUNT,
	// default runtime.NumCPU()). This is the real concurrency limit that keeps
	// memory flat under load.
	Workers int

	// JobTTL is how long a job's in-memory state (including its output bytes)
	// is retained before the reaper frees it (JOB_TTL_MINUTES, default 10m).
	JobTTL time.Duration
}

const (
	defaultPort         = "3000"
	defaultMaxFileMB    = 50
	defaultJobTTLMin    = 10
	uploadHeadroomBytes = 14 << 20 // multipart boundaries + multi-file slack
)

// Load reads configuration from the environment, applies defaults for unset or
// invalid values, and logs the resolved settings once. Invalid numeric values
// (non-numeric or <= 0) fall back to the default rather than failing startup.
func Load() Config {
	maxFileMB := getenvInt("MAX_FILE_SIZE_MB", defaultMaxFileMB)
	maxFileBytes := int64(maxFileMB) << 20

	workers := getenvInt("WORKER_COUNT", runtime.NumCPU())

	ttlMin := getenvInt("JOB_TTL_MINUTES", defaultJobTTLMin)

	cfg := Config{
		Port:           getenvStr("PORT", defaultPort),
		MaxFileBytes:   maxFileBytes,
		MaxUploadBytes: maxFileBytes + uploadHeadroomBytes,
		Workers:        workers,
		JobTTL:         time.Duration(ttlMin) * time.Minute,
	}

	log.Printf("config: port=%s maxFile=%dMB workers=%d jobTTL=%s",
		cfg.Port, cfg.MaxFileBytes>>20, cfg.Workers, cfg.JobTTL)

	return cfg
}

// getenvStr returns the env var value, or def if unset/empty.
func getenvStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// getenvInt parses a positive integer env var. Unset, non-numeric, or
// non-positive values fall back to def (a warning is logged for malformed ones).
func getenvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		log.Printf("config: invalid %s=%q, using default %d", key, v, def)
		return def
	}
	return n
}
