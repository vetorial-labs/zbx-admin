package report

import "testing"

func TestFormatHoursAndHMS(t *testing.T) {
	if FormatHours(4290) != "1.19" {
		t.Fatalf("horas %s", FormatHours(4290))
	}
	if FormatHMS(4290) != "1:11:30" {
		t.Fatalf("hms %s", FormatHMS(4290))
	}
	if Percent(710, 720) != "98.61" {
		t.Fatalf("pct %s", Percent(710, 720))
	}
}

func TestSplitHoursAddsUp(t *testing.T) {
	period := int64(720 * 3600)
	var up, down, acc int64
	for _, d0 := range []int64{11988, period, 264276, 0} {
		u, d, a := SplitHours(period, d0)
		if u+d != a || a != Centihours(period) {
			t.Fatalf("linha %d + %d != %d", u, d, a)
		}
		up += u
		down += d
		acc += a
	}
	if up+down != acc {
		t.Fatalf("total %d + %d != %d", up, down, acc)
	}
}

func TestSlug(t *testing.T) {
	got := Slug("BLANKEN ASSESSORIA E LOGISTICA LTDA - 400733")
	if got != "blanken_assessoria_e_logistica_ltda_400733" {
		t.Fatalf("slug %s", got)
	}
}
