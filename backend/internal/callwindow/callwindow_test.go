package callwindow

import (
	"testing"
	"time"
)

func TestIsOpenRespectsLocalHours(t *testing.T) {
	// 2026-09-07 is a Monday. 10:00 UTC = 15:30 IST (open) and 06:00 EDT (closed).
	mondayMorningUTC := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
	if !IsOpen("IN", "", mondayMorningUTC) {
		t.Error("10:00 UTC should be inside 9-20 IST window")
	}
	if IsOpen("US", "", mondayMorningUTC) {
		t.Error("10:00 UTC is 06:00 in New York — should be closed")
	}
}

func TestIsOpenWeekendClosed(t *testing.T) {
	// Saturday midday UTC — weekday rule should defer regardless of hour.
	saturday := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	if IsOpen("IN", "", saturday) {
		t.Error("weekends should be closed by default")
	}
}

func TestIsOpenTZOverride(t *testing.T) {
	// 03:00 UTC = 08:30 in Kolkata (closed) but 04:00 in New York (closed too).
	// Use an override that makes it open: 20:00 UTC = 01:00 next day Kolkata (closed),
	// but as America/Los_Angeles it's 13:00 (open).
	night := time.Date(2026, 9, 7, 20, 0, 0, 0, time.UTC)
	if IsOpen("IN", "America/Los_Angeles", night) {
		t.Log("override respected: LA daytime while IN default would be closed")
	} else {
		t.Error("TZ override should be honored — 20:00 UTC is 13:00 in Los Angeles")
	}
}

func TestNextOpenFromEvening(t *testing.T) {
	// Monday 21:00 New York → next open is Tuesday 09:00 New York.
	mondayNight := time.Date(2026, 9, 7, 21, 0, 0, 0, time.UTC)
	next := NextOpen("US", "", mondayNight)
	ny, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	want := time.Date(2026, 9, 8, 9, 0, 0, 0, ny)
	if !next.Equal(want) {
		t.Errorf("NextOpen = %s, want %s", next, want)
	}
}

func TestNextOpenFromSaturday(t *testing.T) {
	// Saturday → Monday 09:00 local.
	sat := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	next := NextOpen("IN", "", sat)
	kolkata, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	want := time.Date(2026, 9, 7, 9, 0, 0, 0, kolkata)
	if !next.Equal(want) {
		t.Errorf("NextOpen = %s, want %s", next, want)
	}
}
