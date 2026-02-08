# cgroupv2

A simple Go package for containers to read their own cgroup v2 resource usage as percentages of configured limits.

## Why?

When running inside a container, you often need to know your resource consumption relative to container limits (not host resources). Common use cases:

- **Backpressure**: Drop requests when CPU/memory approaches limits
- **Auto-scaling hints**: Report load metrics for scaling decisions
- **Resource monitoring**: Track actual container utilization

Existing packages like `gopsutil` read system-wide stats from `/proc`, which shows host-level metrics - not what your container is allowed to use. This package reads directly from cgroup v2 files to give you container-aware percentages.

## Requirements

- Linux with cgroup v2 (unified hierarchy)
- Kubernetes v1.25+ (cgroup v2 GA) or modern Linux distributions

## Installation

```bash
go get github.com/hromov/cgroupv2
```

## Usage

### Basic usage

```go
monitor := cgroupv2.NewMonitor()

// First call establishes CPU baseline (returns 0)
monitor.Stats()

time.Sleep(time.Second)

// Get full stats
stats := monitor.Stats()
fmt.Printf("CPU: %.1f%% (limit: %.2f cores)\n", stats.CPUPercent, stats.CPULimitCores)
fmt.Printf("Memory: %.1f%% (%d / %d bytes)\n",
    stats.MemoryPercent, stats.MemoryBytes, stats.MemoryLimitBytes)
```

### CPU percentage (requires delta calculation)

```go
monitor := cgroupv2.NewMonitor()

// First call establishes baseline (returns 0)
monitor.CPUPercent()

// Wait, then read actual percentage
time.Sleep(time.Second)
cpuPct := monitor.CPUPercent()
fmt.Printf("CPU: %.1f%%\n", cpuPct)
```

### Memory percentage

```go
monitor := cgroupv2.NewMonitor()
memPct := monitor.MemoryPercent()
fmt.Printf("Memory: %.1f%%\n", memPct)
```

### Check availability

```go
if !cgroupv2.Available() {
    log.Println("cgroup v2 not available, falling back to defaults")
}
```

### Custom cgroup path (for testing)

```go
monitor := cgroupv2.NewMonitor(
    cgroupv2.WithCgroupPath("/custom/cgroup/path"),
)
```

## Backpressure example

```go
type Backpressure struct {
    monitor   *cgroupv2.Monitor
    threshold float64
    overCount int
}

func (b *Backpressure) ShouldDrop() bool {
    stats := b.monitor.Stats()

    if stats.CPUPercent >= b.threshold || stats.MemoryPercent >= b.threshold {
        b.overCount++
    } else {
        b.overCount = 0
    }

    // Require 5 consecutive samples over threshold to avoid flapping
    return b.overCount >= 5
}
```

## How it works

The package reads directly from the cgroup v2 unified hierarchy mounted at `/sys/fs/cgroup`:

| Metric | Files |
|--------|-------|
| CPU limit | `/sys/fs/cgroup/cpu.max` (quota/period) |
| CPU usage | `/sys/fs/cgroup/cpu.stat` (usage_usec) |
| Memory limit | `/sys/fs/cgroup/memory.max` |
| Memory usage | `/sys/fs/cgroup/memory.current` |

CPU percentage is calculated as a delta between samples (like `top` or `htop`), which is why it requires a Monitor and returns 0 on the first call.

## Local development

When running outside a container (no cgroup limits), the package returns 0 for all values. This also applies when cgroup files are unavailable or unreadable. Use `Available()` to check if cgroup v2 is present.

> **Note**: CPU percentage can exceed 100% if your container has multiple CPU cores allocated (e.g., 200% means 2 full cores utilized).

## License

MIT
