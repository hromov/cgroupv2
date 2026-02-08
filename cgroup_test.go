package cgroupv2

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMonitor_CPUPercent(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:     "100000 100000", // 1 core limit
		cpuStat:    "usage_usec 1000000",
		memoryMax:  "1073741824", // 1GB
		memoryCur:  "536870912",  // 512MB
	})
	m := NewMonitor(WithCgroupPath(dir))

	// First call returns 0 (baseline)
	pct := m.CPUPercent()
	if pct != 0 {
		t.Errorf("first call should return 0, got %f", pct)
	}

	// Simulate time passing and CPU usage
	time.Sleep(10 * time.Millisecond)
	writeCgroupFile(t, filepath.Join(dir, "cpu.stat"), "usage_usec 1100000") // +100ms of CPU

	pct = m.CPUPercent()
	// Should be positive after delta
	if pct <= 0 {
		t.Errorf("second call should return positive percentage, got %f", pct)
	}
}

func TestMonitor_Stats(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:     "50000 100000", // 0.5 core limit
		cpuStat:    "usage_usec 5000000",
		memoryMax:  "2147483648",  // 2GB
		memoryCur:  "1073741824",  // 1GB = 50%
	})
	m := NewMonitor(WithCgroupPath(dir))

	stats := m.Stats()

	if stats.CPULimitCores != 0.5 {
		t.Errorf("CPULimitCores = %f, want 0.5", stats.CPULimitCores)
	}

	if stats.MemoryPercent != 50.0 {
		t.Errorf("MemoryPercent = %f, want 50.0", stats.MemoryPercent)
	}

	if stats.MemoryBytes != 1073741824 {
		t.Errorf("MemoryBytes = %d, want 1073741824", stats.MemoryBytes)
	}

	if stats.MemoryLimitBytes != 2147483648 {
		t.Errorf("MemoryLimitBytes = %d, want 2147483648", stats.MemoryLimitBytes)
	}
}

func TestMonitor_MemoryPercent(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		memoryMax: "1000000000",
		memoryCur: "250000000", // 25%
	})
	m := NewMonitor(WithCgroupPath(dir))
	pct := m.MemoryPercent()
	if pct != 25.0 {
		t.Errorf("MemoryPercent() = %f, want 25.0", pct)
	}
}

func TestAvailable(t *testing.T) {
	// Test with valid cgroup - create temp dir with controllers file
	dir := setupTestCgroup(t, testCgroupFiles{})
	defer os.RemoveAll(dir)

	// Available() uses defaultCgroupPath, so we can't easily test true case
	// without root. Just test that it doesn't panic and returns bool.
	_ = Available()
}

func TestNoCPULimit(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:  "max 100000", // No limit
		cpuStat: "usage_usec 1000000",
	})
	m := NewMonitor(WithCgroupPath(dir))
	stats := m.Stats()

	if stats.CPUPercent != 0 {
		t.Errorf("CPUPercent with no limit = %f, want 0", stats.CPUPercent)
	}
	if stats.CPULimitCores != 0 {
		t.Errorf("CPULimitCores with no limit = %f, want 0", stats.CPULimitCores)
	}
}

func TestNoMemoryLimit(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		memoryMax: "max",
		memoryCur: "1000000",
	})
	m := NewMonitor(WithCgroupPath(dir))
	pct := m.MemoryPercent()
	if pct != 0 {
		t.Errorf("MemoryPercent with no limit = %f, want 0", pct)
	}
}

func TestParseCPUStatUsage(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    uint64
	}{
		{
			name:    "standard format",
			content: "usage_usec 12345678\nuser_usec 10000000\nsystem_usec 2345678",
			want:    12345678,
		},
		{
			name:    "usage not first",
			content: "nr_periods 100\nusage_usec 99999\nnr_throttled 5",
			want:    99999,
		},
		{
			name:    "missing usage",
			content: "user_usec 10000000\nsystem_usec 2345678",
			want:    0,
		},
		{
			name:    "empty content",
			content: "",
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCPUStatUsage(tt.content)
			if got != tt.want {
				t.Errorf("parseCPUStatUsage() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWithCgroupPath(t *testing.T) {
	m := NewMonitor(WithCgroupPath("/custom/path"))
	if m.cgroupPath != "/custom/path" {
		t.Errorf("cgroupPath = %s, want /custom/path", m.cgroupPath)
	}
}

func TestDefaultCgroupPath(t *testing.T) {
	m := NewMonitor()
	if m.cgroupPath != defaultCgroupPath {
		t.Errorf("cgroupPath = %s, want %s", m.cgroupPath, defaultCgroupPath)
	}
}

func TestZeroInitialUsage(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:  "100000 100000",
		cpuStat: "usage_usec 0", // Container starts with zero usage
	})
	m := NewMonitor(WithCgroupPath(dir))

	// First call should establish baseline and return 0
	pct := m.CPUPercent()
	if pct != 0 {
		t.Errorf("first call should return 0, got %f", pct)
	}

	// Simulate CPU usage
	time.Sleep(10 * time.Millisecond)
	writeCgroupFile(t, filepath.Join(dir, "cpu.stat"), "usage_usec 50000")

	// Second call should return positive percentage (baseline was established)
	pct = m.CPUPercent()
	if pct <= 0 {
		t.Errorf("second call should return positive percentage, got %f", pct)
	}
}

func TestNegativeQuota(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:  "-100000 100000", // Negative quota - invalid
		cpuStat: "usage_usec 1000000",
	})
	m := NewMonitor(WithCgroupPath(dir))
	stats := m.Stats()

	// Should treat negative quota as no limit
	if stats.CPUPercent != 0 {
		t.Errorf("CPUPercent with negative quota = %f, want 0", stats.CPUPercent)
	}
	if stats.CPULimitCores != 0 {
		t.Errorf("CPULimitCores with negative quota = %f, want 0", stats.CPULimitCores)
	}
}

func TestCounterReset(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:  "100000 100000",
		cpuStat: "usage_usec 1000000",
	})
	m := NewMonitor(WithCgroupPath(dir))

	// Establish baseline
	m.CPUPercent()
	time.Sleep(10 * time.Millisecond)

	// Simulate counter reset (container restart, cgroup reset)
	writeCgroupFile(t, filepath.Join(dir, "cpu.stat"), "usage_usec 500") // Less than before

	// Should handle gracefully, not produce negative/huge values
	pct := m.CPUPercent()
	if pct < 0 || pct > 10000 {
		t.Errorf("counter reset should be handled gracefully, got %f", pct)
	}
}

func TestZeroPeriod(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:  "100000 0", // Zero period - should not panic
		cpuStat: "usage_usec 1000000",
	})
	m := NewMonitor(WithCgroupPath(dir))

	// Should not panic, should return 0
	stats := m.Stats()

	if stats.CPUPercent != 0 {
		t.Errorf("CPUPercent with zero period = %f, want 0", stats.CPUPercent)
	}
	if stats.CPULimitCores != 0 {
		t.Errorf("CPULimitCores with zero period = %f, want 0", stats.CPULimitCores)
	}
}

func TestMonitor_ConcurrentAccess(t *testing.T) {
	dir := setupTestCgroup(t, testCgroupFiles{
		cpuMax:    "100000 100000",
		cpuStat:   "usage_usec 1000000",
		memoryMax: "1073741824",
		memoryCur: "536870912",
	})
	m := NewMonitor(WithCgroupPath(dir))

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				m.Stats()
				m.CPUPercent()
				m.MemoryPercent()
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// Benchmarks

func BenchmarkStats(b *testing.B) {
	dir := setupTestCgroup(b, testCgroupFiles{
		cpuMax:    "100000 100000",
		cpuStat:   "usage_usec 1000000",
		memoryMax: "1073741824",
		memoryCur: "536870912",
	})
	m := NewMonitor(WithCgroupPath(dir))
	m.Stats() // baseline

	b.ResetTimer()
	for b.Loop() {
		m.Stats()
	}
}

func BenchmarkCPUPercent(b *testing.B) {
	dir := setupTestCgroup(b, testCgroupFiles{
		cpuMax:    "100000 100000",
		cpuStat:   "usage_usec 1000000",
		memoryMax: "1073741824",
		memoryCur: "536870912",
	})
	m := NewMonitor(WithCgroupPath(dir))
	m.CPUPercent() // baseline

	b.ResetTimer()
	for b.Loop() {
		m.CPUPercent()
	}
}

func BenchmarkMemoryPercent(b *testing.B) {
	dir := setupTestCgroup(b, testCgroupFiles{
		cpuMax:    "100000 100000",
		cpuStat:   "usage_usec 1000000",
		memoryMax: "1073741824",
		memoryCur: "536870912",
	})
	m := NewMonitor(WithCgroupPath(dir))

	b.ResetTimer()
	for b.Loop() {
		m.MemoryPercent()
	}
}

// Test helpers

type testCgroupFiles struct {
	cpuMax    string
	cpuStat   string
	memoryMax string
	memoryCur string
}

func setupTestCgroup(tb testing.TB, files testCgroupFiles) string {
	tb.Helper()

	dir, err := os.MkdirTemp("", "cgroupv2-test-*")
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { os.RemoveAll(dir) })

	// Create cgroup.controllers to indicate v2 is available
	writeCgroupFile(tb, filepath.Join(dir, "cgroup.controllers"), "cpu memory")

	if files.cpuMax != "" {
		writeCgroupFile(tb, filepath.Join(dir, "cpu.max"), files.cpuMax)
	}
	if files.cpuStat != "" {
		writeCgroupFile(tb, filepath.Join(dir, "cpu.stat"), files.cpuStat)
	}
	if files.memoryMax != "" {
		writeCgroupFile(tb, filepath.Join(dir, "memory.max"), files.memoryMax)
	}
	if files.memoryCur != "" {
		writeCgroupFile(tb, filepath.Join(dir, "memory.current"), files.memoryCur)
	}

	return dir
}

func writeCgroupFile(tb testing.TB, path, content string) {
	tb.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		tb.Fatal(err)
	}
}
