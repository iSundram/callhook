// Package callwindow keeps calls inside polite local hours. A courtesy
// redial at 2 AM loses customers; this gate defers instead.
package callwindow

import (
	"time"
)

// Default open hours, in the recipient's local time.
const (
	OpenHour  = 9
	CloseHour = 20
)

// regionTZ maps a recipient region to its default timezone. Approximate —
// one zone per country; production systems should store the customer's
// actual timezone and pass it as the session TZ override.
var regionTZ = map[string]string{
	"US": "America/New_York",
	"CA": "America/Toronto",
	"GB": "Europe/London",
	"IE": "Europe/Dublin",
	"DE": "Europe/Berlin",
	"FR": "Europe/Paris",
	"NL": "Europe/Amsterdam",
	"ES": "Europe/Madrid",
	"IT": "Europe/Rome",
	"SG": "Asia/Singapore",
	"MY": "Asia/Kuala_Lumpur",
	"IN": "Asia/Kolkata",
	"AE": "Asia/Dubai",
	"SA": "Asia/Riyadh",
	"AU": "Australia/Sydney",
	"NZ": "Pacific/Auckland",
	"JP": "Asia/Tokyo",
	"KR": "Asia/Seoul",
	"BR": "America/Sao_Paulo",
	"MX": "America/Mexico_City",
	"ZA": "Africa/Johannesburg",
}

// tz resolves the timezone for a session: explicit override first, then the
// region default, then UTC.
func tz(region, override string) *time.Location {
	name := override
	if name == "" {
		name = regionTZ[region]
	}
	if name == "" {
		name = "UTC"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// IsOpen reports whether a call may be placed to this region right now.
func IsOpen(region, tzOverride string, now time.Time) bool {
	local := now.In(tz(region, tzOverride))
	h := local.Hour()
	if local.Weekday() == time.Saturday || local.Weekday() == time.Sunday {
		return false // conservative default: weekdays only
	}
	return h >= OpenHour && h < CloseHour
}

// NextOpen returns when the window next opens for this region. Always in
// the future relative to now.
func NextOpen(region, tzOverride string, now time.Time) time.Time {
	loc := tz(region, tzOverride)
	local := now.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), OpenHour, 0, 0, 0, loc)
	for !day.After(local) || !isWeekday(day) {
		day = day.AddDate(0, 0, 1)
		day = time.Date(day.Year(), day.Month(), day.Day(), OpenHour, 0, 0, 0, loc)
	}
	return day
}

func isWeekday(t time.Time) bool {
	wd := t.Weekday()
	return wd != time.Saturday && wd != time.Sunday
}
