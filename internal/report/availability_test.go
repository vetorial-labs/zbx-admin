package report

import (
	"testing"
	"time"
)

func TestAccountEventsClipsMergesAndSums(t *testing.T) {
	const from, to int64 = 1000, 5000
	down, outages := AccountEvents(from, to, []Incident{
		{Start: 900, End: 1500},
		{Start: 1400, End: 1800},
		{Start: 2000, End: 2500},
		{Start: 4000, Open: true},
		{Start: 100, End: 200},
	})
	if down != 2300 {
		t.Fatalf("down %d", down)
	}
	up := (to - from) - down
	if up+down != to-from {
		t.Fatalf("soma %d + %d", up, down)
	}
	if len(outages) != 3 {
		t.Fatalf("intervalos %+v", outages)
	}
	if !outages[2].Open || outages[2].Start != 4000 || outages[2].End != 5000 {
		t.Fatalf("aberto %+v", outages[2])
	}
	if outages[0].Open {
		t.Fatal("incidente encerrado não segue em andamento")
	}
	if outages[0].Seconds != 800 {
		t.Fatalf("primeiro intervalo %d", outages[0].Seconds)
	}
}

func TestAccountEventsClosedAfterPeriodIsNotOngoing(t *testing.T) {
	_, outages := AccountEvents(100, 200, []Incident{{Start: 150, End: 400}})
	if len(outages) != 1 {
		t.Fatalf("intervalos %+v", outages)
	}
	if outages[0].Open || outages[0].End != 200 || outages[0].Seconds != 50 {
		t.Fatalf("recorte %+v", outages[0])
	}
}

func TestAccountHistory(t *testing.T) {
	initial := true
	down, outages := AccountHistory(0, 100, &initial, []Sample{
		{Clock: 50, Up: true},
		{Clock: 60, Up: false},
		{Clock: 90, Up: true},
	})
	if down != 80 {
		t.Fatalf("down %d", down)
	}
	if len(outages) != 2 || outages[1].Open {
		t.Fatalf("intervalos %+v", outages)
	}
	up := int64(100) - down
	if up+down != 100 {
		t.Fatalf("soma %d", up+down)
	}
}

func TestAccountHistoryUnknownStartStaysUp(t *testing.T) {
	down, outages := AccountHistory(0, 100, nil, []Sample{
		{Clock: 10, Up: true},
		{Clock: 40, Up: false},
	})
	if down != 60 || len(outages) != 1 || !outages[0].Open {
		t.Fatalf("down %d intervalos %+v", down, outages)
	}
}

func TestAccountTrendsPartialHour(t *testing.T) {
	down, outages := AccountTrends(0, 7200, []Trend{
		{Clock: 0, Avg: 0, Num: 30},
		{Clock: 3600, Avg: 0.75, Num: 30},
		{Clock: 7200, Avg: 0, Num: 30},
	})
	if down != 4500 {
		t.Fatalf("down %d", down)
	}
	if len(outages) != 2 || outages[1].Resolution != ResolutionHour || outages[1].Seconds != 900 {
		t.Fatalf("intervalos %+v", outages)
	}
	if 7200-down+down != 7200 {
		t.Fatal("soma divergente")
	}
}

func TestParseRetentionAndCoverage(t *testing.T) {
	if ParseRetention("1w") != 7*24*time.Hour {
		t.Fatalf("1w = %s", ParseRetention("1w"))
	}
	if ParseRetention("7d") != 7*24*time.Hour {
		t.Fatalf("7d = %s", ParseRetention("7d"))
	}
	now := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	retention := 7 * 24 * time.Hour
	if HistoryCovers(retention, now.Add(-2*24*time.Hour), now) != true {
		t.Fatal("dois dias cabem em uma semana")
	}
	if HistoryCovers(retention, now.Add(-20*24*time.Hour), now) {
		t.Fatal("vinte dias não cabem no histórico de uma semana")
	}
}
