package wowplayed

import (
	"math"
	"sort"
	"time"

	"github.com/knightsofeternity/kfire-server/internal/store"
)

// PresenceWeeks is how far back the typical week looks.
const PresenceWeeks = 8

// Slot is one hour of the typical week. Weekday 0 is Monday.
type Slot struct {
	Weekday int     `json:"weekday"`
	Hour    int     `json:"hour"`
	Average float64 `json:"average"`
}

// TypicalWeek averages the hourly presence by weekday (Monday first) and
// hour. An hour nobody watched (nil) is left out of its average, and a cell
// with no data at all stays nil. best holds the n highest cells, ties broken
// by the earlier slot of the week.
func TypicalWeek(days []store.WowGuildPresenceDay, n int) (week [7][24]*float64, best []Slot) {
	var sum, cnt [7][24]int
	for _, d := range days {
		wd := (int(d.Day.Weekday()) + 6) % 7
		for h, v := range d.Hours {
			if v != nil {
				sum[wd][h] += *v
				cnt[wd][h]++
			}
		}
	}
	var slots []Slot
	for wd := range 7 {
		for h := range 24 {
			if cnt[wd][h] == 0 {
				continue
			}
			avg := math.Round(float64(sum[wd][h])/float64(cnt[wd][h])*10) / 10
			week[wd][h] = &avg
			if avg > 0 {
				slots = append(slots, Slot{Weekday: wd, Hour: h, Average: avg})
			}
		}
	}
	sort.SliceStable(slots, func(i, j int) bool { return slots[i].Average > slots[j].Average })
	if len(slots) > n {
		slots = slots[:n]
	}
	if slots == nil {
		slots = []Slot{}
	}
	return week, slots
}

// PresenceSince is the first day the typical week reads, as of now.
func PresenceSince(now time.Time) time.Time {
	d := now.UTC()
	return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -7*PresenceWeeks+1)
}
