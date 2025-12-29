package services

import (
	"fmt"
	"strings"
	"time"
)

const (
	assignSrcAuto    = "auto"
	assignSrcManual  = "manual"
	assignSrcBlocked = "blocked"

	assignStateAssigned = "assigned"
	assignStateNoMatch  = "no_match"
	assignStateOverlap  = "overlap"
	assignStateBlocked  = "blocked"
)

func loadLocationOrUTC(name string) *time.Location {
	name = strings.TrimSpace(name)
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

func parseRFC3339ToUTC(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	// RFC3339Nano accepts both with/without fractional seconds.
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func parsePaymentTimeToUTC(s string, defaultLoc *time.Location) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty transaction_time")
	}

	// Prefer explicit offsets if present.
	if t, err := parseRFC3339ToUTC(s); err == nil {
		return t, nil
	}

	if defaultLoc == nil {
		defaultLoc = time.UTC
	}

	dateTimeLayouts := []string{
		// Common OCR formats (no timezone).
		"2006-01-02 15:04:05",
		"2006-1-2 15:04:05",
		"2006-01-2 15:04:05",
		"2006-1-02 15:04:05",

		// Sometimes with slashes.
		"2006/01/02 15:04:05",
		"2006/1/2 15:04:05",

		// Sometimes date and time are concatenated.
		"2006-01-0215:04:05",
		"2006-1-215:04:05",

		// Sometimes with explicit offsets but no 'T'.
		"2006-01-02 15:04:05-07:00",
		"2006-1-2 15:04:05-07:00",
	}
	for _, layout := range dateTimeLayouts {
		if t, err := time.ParseInLocation(layout, s, defaultLoc); err == nil {
			return t.UTC(), nil
		}
	}

	dateOnlyLayouts := []string{
		"2006-01-02",
		"2006-1-2",
		"2006/01/02",
		"2006/1/2",
	}
	for _, layout := range dateOnlyLayouts {
		if t, err := time.ParseInLocation(layout, s, defaultLoc); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported transaction_time format: %q", s)
}

func unixMilli(t time.Time) int64 {
	return t.UTC().UnixNano() / int64(time.Millisecond)
}
