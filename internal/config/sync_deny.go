package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// syncDenyRange is a daily UTC window [start, end) in minutes from midnight.
// If start > end, the denied range crosses midnight (e.g. 22:00–06:00).
type syncDenyRange struct {
	startMin int
	endMin   int
	enabled  bool
}

func ParseSyncDenyRangeUTC(s string) (*syncDenyRange, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return &syncDenyRange{}, nil
	}
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("SYNC_DENY_RANGE_UTC must look like 22:00-06:00, got %q", s)
	}
	a, err := parseHHMM(parts[0])
	if err != nil {
		return nil, err
	}
	b, err := parseHHMM(parts[1])
	if err != nil {
		return nil, err
	}
	return &syncDenyRange{startMin: a, endMin: b, enabled: true}, nil
}

func parseHHMM(s string) (int, error) {
	s = strings.TrimSpace(s)
	chunks := strings.Split(s, ":")
	if len(chunks) != 2 {
		return 0, fmt.Errorf("invalid time %q (want HH:MM)", s)
	}
	h, err := strconv.Atoi(chunks[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(chunks[1])
	if err != nil {
		return 0, err
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("time out of range: %q", s)
	}
	return h*60 + m, nil
}

func (w *syncDenyRange) blocksUTC(t time.Time) bool {
	if w == nil || !w.enabled {
		return false
	}
	now := t.UTC()
	cur := now.Hour()*60 + now.Minute()
	if w.startMin < w.endMin {
		return cur >= w.startMin && cur < w.endMin
	}
	if w.startMin > w.endMin {
		return cur >= w.startMin || cur < w.endMin
	}
	return false
}
