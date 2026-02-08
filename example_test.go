package cgroupv2_test

import (
	"fmt"
	"time"

	"github.com/hromov/cgroupv2"
)

func Example() {
	// Check if cgroup v2 is available
	if !cgroupv2.Available() {
		fmt.Println("cgroup v2 not available")
		return
	}

	// Create a monitor for resource tracking
	monitor := cgroupv2.NewMonitor()

	// First call establishes baseline
	monitor.Stats()

	// Wait for some CPU activity
	time.Sleep(100 * time.Millisecond)

	// Get full stats
	stats := monitor.Stats()

	fmt.Printf("CPU: %.1f%% of %.2f cores\n", stats.CPUPercent, stats.CPULimitCores)
	fmt.Printf("Memory: %.1f%% (%d bytes of %d limit)\n",
		stats.MemoryPercent, stats.MemoryBytes, stats.MemoryLimitBytes)
}

func ExampleMonitor_MemoryPercent() {
	monitor := cgroupv2.NewMonitor()
	pct := monitor.MemoryPercent()
	fmt.Printf("Memory usage: %.1f%%\n", pct)
}

func ExampleMonitor_CPUPercent() {
	monitor := cgroupv2.NewMonitor()

	// First call returns 0 (baseline)
	_ = monitor.CPUPercent()

	// Do some work
	time.Sleep(100 * time.Millisecond)

	// Subsequent calls return actual percentage
	pct := monitor.CPUPercent()
	fmt.Printf("CPU usage: %.1f%%\n", pct)
}

func ExampleWithCgroupPath() {
	// Use custom cgroup path (useful for testing)
	monitor := cgroupv2.NewMonitor(
		cgroupv2.WithCgroupPath("/custom/cgroup/path"),
	)
	_ = monitor
}
