package errors

import (
	"testing"
)

func TestIndexNth(t *testing.T) {
	s := "testing \nString \nabc testing, abc\n again"
	sep := "\n"

	for _, s := range []struct {
		str string
		sep string
		index int
		expected int
	} {
		{s, sep, 0, -1},
		{s, sep, 1, 8},
		{s, sep, 2, 16},
		{s, sep, 3, 33},
		{s, sep, 4, -2},
		{s, "\t", 1, -1},
		{"", "", 1, -1},
		{"", sep, 1, -1},
	} {
		actual := IndexNth(s.str, s.sep, s.index)
		if actual != s.expected {
			t.Errorf("index nth of [%q] by [%q] not return properly: expected: [%d], actual: [%d]", s.str, s.sep, s.expected, actual)
		}
	}
}
