package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRecurrenceText(t *testing.T) {
	loc := time.FixedZone("ICT", 7*3600)
	wed := time.Date(2026, 10, 14, 9, 0, 0, 0, loc) // a Wednesday, the 14th
	for _, c := range []struct {
		rule, want string
	}{
		{"FREQ=DAILY", "Hằng ngày"},
		{"FREQ=WEEKLY", "Hằng tuần vào thứ Tư"},
		{"FREQ=WEEKLY;BYDAY=MO,WE,FR", "Hằng tuần vào thứ Hai, thứ Tư và thứ Sáu"},
		{"FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", "Mọi ngày trong tuần (thứ Hai đến thứ Sáu)"},
		{"FREQ=WEEKLY;INTERVAL=2", "Mỗi 2 tuần vào thứ Tư"},
		{"FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,TU,WE,TH,FR", "Mỗi 2 tuần, từ thứ Hai đến thứ Sáu"},
		{"FREQ=DAILY;INTERVAL=3", "Mỗi 3 ngày"},
		{"FREQ=MONTHLY;BYDAY=2WE", "Hằng tháng vào thứ Tư thứ hai"},
		{"FREQ=MONTHLY;BYDAY=WE;BYSETPOS=2", "Hằng tháng vào thứ Tư thứ hai"},
		{"FREQ=MONTHLY;BYDAY=-1FR", "Hằng tháng vào thứ Sáu cuối cùng"},
		{"FREQ=MONTHLY", "Hằng tháng vào ngày 14"},
		{"FREQ=MONTHLY;BYMONTHDAY=1,15", "Hằng tháng vào ngày 1 và 15"},
		{"FREQ=MONTHLY;BYMONTHDAY=-1", "Hằng tháng vào ngày cuối cùng"},
		{"FREQ=MONTHLY;INTERVAL=3", "Mỗi 3 tháng vào ngày 14"},
		{"FREQ=YEARLY", "Hằng năm vào ngày 14 tháng 10"},
		{"FREQ=YEARLY;BYMONTH=3;BYMONTHDAY=8", "Hằng năm vào ngày 8 tháng 3"},
		{"FREQ=YEARLY;BYMONTH=11;BYDAY=4TH", "Hằng năm vào thứ Năm thứ tư của tháng 11"},
		{"FREQ=DAILY;COUNT=10", "Hằng ngày, 10 lần"},
		{"FREQ=WEEKLY;UNTIL=20261231", "Hằng tuần vào thứ Tư, đến ngày 31/12/2026"},
		{"RRULE:FREQ=WEEKLY;UNTIL=20261231T170000Z", "Hằng tuần vào thứ Tư, đến ngày 01/01/2027"},
		{"nonsense", ""},
	} {
		assert.Equal(t, c.want, RecurrenceText(c.rule, wed), c.rule)
	}
}
