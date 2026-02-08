package cgroupv2

// readMemory reads memory usage and limit from cgroup.
// Returns (percent, currentBytes, limitBytes, error).
func (m *Monitor) readMemory() (float64, uint64, uint64, error) {
	memMax, err := readFile(m.cgroupPath + "/memory.max")
	if err != nil {
		return 0, 0, 0, err
	}

	if memMax == unlimitedValue {
		return 0, 0, 0, nil
	}

	memLimit, err := parseUint64(memMax)
	if err != nil {
		return 0, 0, 0, nil
	}

	memCurrent, err := readFile(m.cgroupPath + "/memory.current")
	if err != nil {
		return 0, 0, memLimit, err
	}

	memBytes, err := parseUint64(memCurrent)
	if err != nil {
		return 0, 0, memLimit, nil
	}

	memPercent := float64(memBytes) / float64(memLimit) * 100

	return memPercent, memBytes, memLimit, nil
}
