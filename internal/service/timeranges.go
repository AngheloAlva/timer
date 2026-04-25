package service

import "time"

// StartOfDay returns midnight (00:00:00.000) on the same calendar day as t,
// in t's location. Used as the lower bound for `--today` filters.
func StartOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// StartOfISOWeek returns Monday 00:00 of the ISO week that contains t,
// in t's location. ISO weeks start on Monday.
func StartOfISOWeek(t time.Time) time.Time {
	sod := StartOfDay(t)
	// time.Weekday: Sunday=0, Monday=1, ..., Saturday=6.
	// We want Monday as day 1 and Sunday as day 7 → offset by -(weekday-1).
	wd := int(sod.Weekday())
	if wd == 0 {
		wd = 7
	}
	return sod.AddDate(0, 0, -(wd - 1))
}
