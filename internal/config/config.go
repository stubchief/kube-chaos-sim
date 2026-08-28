package config

import "time"

// Chaos action defaults
const (
	DefaultCPUDuration    = 15 // seconds (reduced from 30s for faster demo feedback)
	DefaultLatencySeconds = 5
	DefaultMemoryMB       = 100
)

// Metrics & polling intervals
const (
	CPUMetricsInterval      = 1 * time.Second // was 5s - more responsive UI
	MetricsTickInterval     = 1 * time.Second // was 2s - smoother charts
	MetricsHistorySize      = 120             // 120 points × 1s = 2 minutes
	MetricsServerResolution = 15              // seconds - minimum supported by metrics-server (must be > kubelet-request-timeout 10s)
)

// HPA defaults
const (
	DefaultHPATargetCPU   = 50
	DefaultHPAMinReplicas = 1
	DefaultHPAMaxReplicas = 10
)

// PDB defaults
const (
	DefaultPDBMinAvailable = 1
)