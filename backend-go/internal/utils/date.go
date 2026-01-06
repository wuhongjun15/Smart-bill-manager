package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var dateYMDRegex = regexp.MustCompile(`(\d{4})\D+(\d{1,2})\D+(\d{1,2})`)

// NormalizeDateYMD extracts a "YYYY-MM-DD" date from a free-form date string.
// Returns empty string if no valid date can be found.
func NormalizeDateYMD(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return ""
	}

	// Fast-path: already ISO-like prefix.
	if len(s) >= 10 {
		p := s[:10]
		if len(p) == 10 {
			if p[4] == '-' && p[7] == '-' {
				if y, m, d, ok := parseYMDParts(p[0:4], p[5:7], p[8:10]); ok {
					return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
				}
			}
			if p[4] == '/' && p[7] == '/' {
				if y, m, d, ok := parseYMDParts(p[0:4], p[5:7], p[8:10]); ok {
					return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
				}
			}
		}
	}

	// Generic: YYYY<sep>MM<sep>DD (supports 年/月/日, spaces, etc).
	if m := dateYMDRegex.FindStringSubmatch(s); len(m) == 4 {
		if y, mo, d, ok := parseYMDParts(m[1], m[2], m[3]); ok {
			return fmt.Sprintf("%04d-%02d-%02d", y, mo, d)
		}
	}

	return ""
}

func parseYMDParts(yStr, mStr, dStr string) (int, int, int, bool) {
	y, err1 := strconv.Atoi(strings.TrimSpace(yStr))
	mo, err2 := strconv.Atoi(strings.TrimSpace(mStr))
	d, err3 := strconv.Atoi(strings.TrimSpace(dStr))
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, 0, 0, false
	}
	if y < 1000 || y > 9999 || mo < 1 || mo > 12 || d < 1 || d > 31 {
		return 0, 0, 0, false
	}
	t := time.Date(y, time.Month(mo), d, 0, 0, 0, 0, time.UTC)
	if t.Year() != y || int(t.Month()) != mo || t.Day() != d {
		return 0, 0, 0, false
	}
	return y, mo, d, true
}
