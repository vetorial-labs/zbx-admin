package report

import (
	"testing"
	"time"
)

func testLoc(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestCurrentMonthIsPartial(t *testing.T) {
	loc := testLoc(t)
	now := time.Date(2026, 10, 29, 15, 4, 0, 0, loc)
	p, capped, err := Resolve(loc, now, "mes-atual", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if capped {
		t.Fatal("mês atual não deve ser marcado como cortado")
	}
	wantFrom := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	if !p.From.Equal(wantFrom) || !p.To.Equal(now) {
		t.Fatalf("período %s até %s", p.From, p.To)
	}
	if p.Label != "Mês atual" {
		t.Fatalf("rótulo %s", p.Label)
	}
}

func TestPreviousMonthWrapsYear(t *testing.T) {
	loc := testLoc(t)
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, loc)
	p, _, err := Resolve(loc, now, "mes-anterior", "", "")
	if err != nil {
		t.Fatal(err)
	}
	wantFrom := time.Date(2025, 12, 1, 0, 0, 0, 0, loc)
	wantTo := time.Date(2026, 1, 1, 0, 0, 0, 0, loc)
	if !p.From.Equal(wantFrom) || !p.To.Equal(wantTo) {
		t.Fatalf("período %s até %s", p.From, p.To)
	}
	if p.Seconds() != 31*24*3600 {
		t.Fatalf("segundos %d", p.Seconds())
	}
	start, end := p.FileDates()
	if start != "2025-12-01" || end != "2025-12-31" {
		t.Fatalf("arquivo %s %s", start, end)
	}
}

func TestCustomDateIsInclusive(t *testing.T) {
	loc := testLoc(t)
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, loc)
	p, capped, err := Resolve(loc, now, "personalizado", "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if capped {
		t.Fatal("setembro não está no futuro")
	}
	wantTo := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	if !p.From.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, loc)) || !p.To.Equal(wantTo) {
		t.Fatalf("período %s até %s", p.From, p.To)
	}
}

func TestCustomDatetimeAndFutureCap(t *testing.T) {
	loc := testLoc(t)
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, loc)
	p, capped, err := Resolve(loc, now, "custom", "2026-10-29T08:00", "2026-10-29T18:00")
	if err != nil {
		t.Fatal(err)
	}
	if !capped {
		t.Fatal("o fim da tarde ainda não aconteceu")
	}
	if !p.To.Equal(now) {
		t.Fatalf("fim %s", p.To)
	}
	if !p.From.Equal(time.Date(2026, 10, 29, 8, 0, 0, 0, loc)) {
		t.Fatalf("início %s", p.From)
	}
}

func TestCustomRequiresBounds(t *testing.T) {
	loc := testLoc(t)
	_, _, err := Resolve(loc, time.Now(), "personalizado", "", "")
	if err == nil {
		t.Fatal("esperava erro")
	}
}

func TestFuturePeriodErrors(t *testing.T) {
	loc := testLoc(t)
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, loc)
	_, _, err := Resolve(loc, now, "personalizado", "2026-11-01", "2026-11-30")
	if err == nil {
		t.Fatal("esperava erro")
	}
}
