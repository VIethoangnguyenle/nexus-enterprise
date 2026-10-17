package domain

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/teambition/rrule-go"
)

// weekdayNames are the Vietnamese names of the days, indexed by RRULE day (0 is
// Monday), the way a calendar week starts there.
var weekdayNames = [7]string{"thứ Hai", "thứ Ba", "thứ Tư", "thứ Năm", "thứ Sáu", "thứ Bảy", "Chủ nhật"}

// ordinalNames say which weekday of the month, for "the second Wednesday".
var ordinalNames = map[int]string{1: "đầu tiên", 2: "thứ hai", 3: "thứ ba", 4: "thứ tư", 5: "thứ năm", -1: "cuối cùng"}

// rruleDay is a time.Weekday as the RRULE index (Monday is 0).
func rruleDay(d time.Weekday) int { return (int(d) + 6) % 7 }

// RecurrenceText describes a recurrence rule in Vietnamese ("Hằng tuần vào thứ
// Tư"). start is the event's first start in its own zone: it fills in whatever
// the rule leaves to the start (the weekday, the day of the month). A rule that
// does not parse yields "".
func RecurrenceText(body string, start time.Time) string {
	opt, err := rrule.StrToROptionInLocation(normalizeRRule(body), start.Location())
	if err != nil {
		return ""
	}
	n := opt.Interval
	if n < 1 {
		n = 1
	}
	var text string
	switch opt.Freq {
	case rrule.DAILY:
		text = every(n, "Hằng ngày", "ngày")
	case rrule.WEEKLY:
		text = weeklyText(opt, n, start)
	case rrule.MONTHLY:
		text = every(n, "Hằng tháng", "tháng") + " vào " + monthlyDay(opt, start)
	case rrule.YEARLY:
		text = every(n, "Hằng năm", "năm") + " vào " + yearlyDay(opt, start)
	default:
		return ""
	}
	switch {
	case opt.Count > 0:
		text += fmt.Sprintf(", %d lần", opt.Count)
	case !opt.Until.IsZero():
		text += ", đến ngày " + opt.Until.In(start.Location()).Format("02/01/2006")
	}
	return text
}

// every is the unit of repetition: "Hằng ngày" for 1, "Mỗi 2 ngày" otherwise.
func every(n int, one, unit string) string {
	if n == 1 {
		return one
	}
	return fmt.Sprintf("Mỗi %d %s", n, unit)
}

func weeklyText(opt *rrule.ROption, n int, start time.Time) string {
	days := map[int]bool{}
	for _, w := range opt.Byweekday {
		days[w.Day()] = true
	}
	if len(days) == 0 {
		days[rruleDay(start.Weekday())] = true
	}
	var order []int
	for d := range days {
		order = append(order, d)
	}
	sort.Ints(order)
	if len(order) == 5 && order[0] == 0 && order[4] == 4 {
		if n == 1 {
			return "Mọi ngày trong tuần (thứ Hai đến thứ Sáu)"
		}
		return fmt.Sprintf("Mỗi %d tuần, từ thứ Hai đến thứ Sáu", n)
	}
	names := make([]string, len(order))
	for i, d := range order {
		names[i] = weekdayNames[d]
	}
	return every(n, "Hằng tuần", "tuần") + " vào " + joinList(names)
}

// joinList is "a", "a và b", "a, b và c".
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}
	return strings.Join(items[:len(items)-1], ", ") + " và " + items[len(items)-1]
}

// nthWeekday reads "the second Wednesday" from a rule: BYDAY=2WE, or BYDAY=WE
// with BYSETPOS=2. ok is false when the rule names no single nth weekday.
func nthWeekday(opt *rrule.ROption) (text string, ok bool) {
	if len(opt.Byweekday) != 1 {
		return "", false
	}
	w := opt.Byweekday[0]
	pos := w.N()
	if pos == 0 && len(opt.Bysetpos) == 1 {
		pos = opt.Bysetpos[0]
	}
	ord, known := ordinalNames[pos]
	if pos == 0 || !known {
		return "", false
	}
	return weekdayNames[w.Day()] + " " + ord, true
}

func monthDays(days []int) string {
	parts := make([]string, len(days))
	for i, d := range days {
		if d == -1 {
			parts[i] = "cuối cùng"
		} else {
			parts[i] = fmt.Sprint(d)
		}
	}
	return "ngày " + joinList(parts)
}

func monthlyDay(opt *rrule.ROption, start time.Time) string {
	if text, ok := nthWeekday(opt); ok {
		return text
	}
	if len(opt.Byweekday) > 0 {
		names := make([]string, len(opt.Byweekday))
		for i, w := range opt.Byweekday {
			names[i] = weekdayNames[w.Day()]
		}
		return "mọi " + joinList(names)
	}
	if len(opt.Bymonthday) > 0 {
		return monthDays(opt.Bymonthday)
	}
	return fmt.Sprintf("ngày %d", start.Day())
}

func yearlyDay(opt *rrule.ROption, start time.Time) string {
	month := int(start.Month())
	if len(opt.Bymonth) > 0 {
		month = opt.Bymonth[0]
	}
	if text, ok := nthWeekday(opt); ok {
		return fmt.Sprintf("%s của tháng %d", text, month)
	}
	day := start.Day()
	if len(opt.Bymonthday) > 0 {
		day = opt.Bymonthday[0]
	}
	return fmt.Sprintf("ngày %d tháng %d", day, month)
}
