package annote

import (
	"time"
)

func ParseNotWellformedTime(input string) time.Time {
	// we try incresingly less specific formats until something matches
	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02Z",
		"2006-01-02",
		"2006-01",
		"2006",
		"2006-1-2",
		"2006-1",
		"01/02/06",
		"1/2/06",
		"January 2, 2006",
		"January 2006",
		"Jan-06",
	}

	for _, f := range formats {
		result, err := time.Parse(f, input)
		if err == nil {
			return result
		}
	}
	return time.Time{}
}
