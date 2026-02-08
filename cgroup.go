// Package cgroupv2 provides a simple API for containers to read their own
// cgroup v2 resource usage as percentages of configured limits.
//
// This package is designed for applications running inside containers that need
// to know their resource consumption relative to container limits (not host resources).
// Common use cases include backpressure, auto-scaling decisions, and resource monitoring.
//
// The package reads directly from the cgroup v2 unified hierarchy mounted at
// /sys/fs/cgroup. It requires cgroup v2 (unified hierarchy) which is the default
// on modern Linux distributions and Kubernetes v1.25+.
package cgroupv2

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultCgroupPath    = "/sys/fs/cgroup"
	unlimitedValue       = "max"
	cgroupControllersFile = "cgroup.controllers"
)

// Option configures a Monitor.
type Option func(*Monitor)

// WithCgroupPath sets a custom cgroup path (useful for testing).
func WithCgroupPath(path string) Option {
	return func(m *Monitor) {
		m.cgroupPath = path
	}
}

// Stats represents the current cgroup resource usage.
type Stats struct {
	// CPUPercent is CPU usage as percentage of limit (0-100+).
	// Can exceed 100% if using more than allocated quota temporarily.
	// Returns 0 if no CPU limit is set or on first sample.
	CPUPercent float64

	// MemoryPercent is memory usage as percentage of limit (0-100).
	MemoryPercent float64

	// MemoryBytes is current memory usage in bytes.
	MemoryBytes uint64

	// MemoryLimitBytes is the memory limit in bytes.
	// Returns 0 if no limit is set.
	MemoryLimitBytes uint64

	// CPULimitCores is the CPU limit in cores (e.g., 0.5, 1.0, 2.0).
	// Returns 0 if no limit is set.
	CPULimitCores float64
}

// Monitor tracks cgroup resource usage over time.
// It maintains state needed for CPU percentage calculation (which requires delta between samples).
type Monitor struct {
	mu sync.Mutex

	cgroupPath string

	lastCPUUsageUsec  uint64
	lastCPUSampleTime time.Time
	hasBaseline       bool
}

// NewMonitor creates a new cgroup monitor.
func NewMonitor(opts ...Option) *Monitor {
	m := &Monitor{
		cgroupPath: defaultCgroupPath,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Stats reads current cgroup resource usage.
// CPU percentage requires at least two calls to calculate (returns 0 on first call).
func (m *Monitor) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()

	var s Stats

	cpuPercent, cpuLimit, _ := m.readCPU()
	s.CPUPercent = cpuPercent
	s.CPULimitCores = cpuLimit

	memPercent, memBytes, memLimit, _ := m.readMemory()
	s.MemoryPercent = memPercent
	s.MemoryBytes = memBytes
	s.MemoryLimitBytes = memLimit

	return s
}

// CPUPercent returns current CPU usage as percentage of limit.
// Returns 0 on first call or if no CPU limit is set.
func (m *Monitor) CPUPercent() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	pct, _, _ := m.readCPU()
	return pct
}

// MemoryPercent returns current memory usage as percentage of limit.
// Returns 0 if no memory limit is set.
func (m *Monitor) MemoryPercent() float64 {
	pct, _, _, _ := m.readMemory()
	return pct
}

// Available returns true if cgroup v2 is available on this system.
func Available() bool {
	_, err := os.Stat(defaultCgroupPath + "/" + cgroupControllersFile)
	return err == nil
}

func readFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func parseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
