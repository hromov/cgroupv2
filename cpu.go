package cgroupv2

import (
	"strings"
	"time"
)

// readCPU reads CPU usage and calculates percentage of container limit.
// Returns (percent, limitCores, error).
func (m *Monitor) readCPU() (float64, float64, error) {
	// Read CPU limit from cpu.max (format: "quota period" or "max period")
	cpuMax, err := readFile(m.cgroupPath + "/cpu.max")
	if err != nil {
		return 0, 0, err
	}

	parts := strings.Fields(cpuMax)
	if len(parts) != 2 {
		return 0, 0, nil
	}

	if parts[0] == unlimitedValue {
		return 0, 0, nil
	}

	quota, err := parseInt64(parts[0])
	if err != nil || quota <= 0 {
		return 0, 0, nil
	}

	period, err := parseInt64(parts[1])
	if err != nil {
		return 0, 0, nil
	}

	if period == 0 {
		return 0, 0, nil
	}

	cpuLimitCores := float64(quota) / float64(period)

	cpuStat, err := readFile(m.cgroupPath + "/cpu.stat")
	if err != nil {
		return 0, cpuLimitCores, err
	}

	usageUsec := parseCPUStatUsage(cpuStat)
	now := time.Now()

	// First sample - establish baseline
	if !m.hasBaseline {
		m.lastCPUUsageUsec = usageUsec
		m.lastCPUSampleTime = now
		m.hasBaseline = true
		return 0, cpuLimitCores, nil
	}

	// Handle counter reset (container restart, cgroup reset)
	if usageUsec < m.lastCPUUsageUsec {
		m.lastCPUUsageUsec = usageUsec
		m.lastCPUSampleTime = now
		return 0, cpuLimitCores, nil
	}

	usageDelta := float64(usageUsec - m.lastCPUUsageUsec)
	timeDelta := now.Sub(m.lastCPUSampleTime)
	if timeDelta == 0 {
		return 0, cpuLimitCores, nil
	}

	// CPU% = (CPU microseconds used / elapsed microseconds) / limit cores * 100
	elapsedUsec := float64(timeDelta.Microseconds())
	coresUsed := usageDelta / elapsedUsec
	cpuPercent := (coresUsed / cpuLimitCores) * 100

	m.lastCPUUsageUsec = usageUsec
	m.lastCPUSampleTime = now

	return cpuPercent, cpuLimitCores, nil
}

func parseCPUStatUsage(content string) uint64 {
	for _, line := range strings.Split(content, "\n") {
		if !strings.HasPrefix(line, "usage_usec ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		val, err := parseUint64(parts[1])
		if err != nil {
			continue
		}
		return val
	}
	return 0
}
