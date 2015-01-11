package errors

import "strings"

// TODO(jasonparekh) TECHDEBT import cycle on miscutil

// Returns the index of the Nth sep or -1 if not found or -2 if too few found
func IndexNth(s, sep string, n int) int {
	count := 0
	lastIdx := -1
	for idx := -1; (count == 0 || idx >= 0) && count < n; {
		substrIdx := strings.Index(s[idx+1:], sep)
		if substrIdx == -1 && count >= 1 {
			return -2
		}
		idx = substrIdx + idx + 1
		count++
		lastIdx = idx
	}

	return lastIdx
}
