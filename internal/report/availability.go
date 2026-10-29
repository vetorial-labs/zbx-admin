package report

import (
	"math"
	"sort"
	"time"
)

const (
	SourceEvent   = "evento"
	SourceHistory = "historico"
	SourceTrend   = "tendencia"
	SourceNone    = "sem_item"

	ResolutionSeconds = "segundos"
	ResolutionHour    = "hora"
)

// Incident é um problema ICMP ainda não recortado no período.
// End vale 0 quando o problema segue aberto.
type Incident struct {
	Start int64
	End   int64
	Open  bool
}

// Sample é um ponto do item icmpping.
type Sample struct {
	Clock int64
	Up    bool
}

// Trend é a média horária do icmpping. Clock é o início da hora.
type Trend struct {
	Clock int64
	Avg   float64
	Num   int
}

// Outage é um período de DOWN já recortado.
// Seconds é o tempo contabilizado como DOWN. Em tendência horária ele pode
// ser menor que End-Start, porque a hora entra pela fração média.
type Outage struct {
	Start      int64
	End        int64
	Seconds    int64
	Open       bool
	Resolution string
}

// AccountEvents recorta, funde e soma os incidentes dentro de [from, to).
func AccountEvents(from, to int64, incidents []Incident) (int64, []Outage) {
	if to <= from {
		return 0, nil
	}
	raw := make([]Outage, 0, len(incidents))
	for _, inc := range incidents {
		start := inc.Start
		end := inc.End
		if inc.Open || end == 0 {
			end = to
		}
		if end <= from || start >= to || end <= start {
			continue
		}
		if start < from {
			start = from
		}
		if end > to {
			end = to
		}
		if end <= start {
			continue
		}
		raw = append(raw, Outage{
			Start:      start,
			End:        end,
			Seconds:    end - start,
			Open:       inc.Open,
			Resolution: ResolutionSeconds,
		})
	}
	merged := mergeOutages(raw)
	return sumSeconds(merged, to-from), merged
}

// AccountHistory monta os intervalos em que o icmpping ficou em 0.
// initialDown é o último valor conhecido antes de from. nil significa que
// o estado anterior é desconhecido e o período começa como UP.
func AccountHistory(from, to int64, initialDown *bool, samples []Sample) (int64, []Outage) {
	if to <= from {
		return 0, nil
	}
	pts := make([]Sample, 0, len(samples))
	for _, s := range samples {
		if s.Clock < from || s.Clock >= to {
			continue
		}
		pts = append(pts, s)
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].Clock < pts[j].Clock })

	down := false
	if initialDown != nil {
		down = *initialDown
	}
	var start int64
	if down {
		start = from
	}
	raw := make([]Outage, 0)
	for _, p := range pts {
		if !p.Up && !down {
			down = true
			start = p.Clock
			continue
		}
		if p.Up && down {
			raw = append(raw, outageSeconds(start, p.Clock, false))
			down = false
		}
	}
	if down {
		raw = append(raw, outageSeconds(start, to, true))
	}
	merged := mergeOutages(raw)
	return sumSeconds(merged, to-from), merged
}

// AccountTrends converte médias horárias em tempo DOWN.
// Cada hora com média menor que 1 vira uma linha de resolução horária.
func AccountTrends(from, to int64, trends []Trend) (int64, []Outage) {
	if to <= from {
		return 0, nil
	}
	raw := make([]Outage, 0, len(trends))
	for _, tr := range trends {
		if tr.Num <= 0 {
			continue
		}
		hourStart := tr.Clock
		hourEnd := tr.Clock + 3600
		if hourEnd <= from || hourStart >= to {
			continue
		}
		if hourStart < from {
			hourStart = from
		}
		if hourEnd > to {
			hourEnd = to
		}
		span := hourEnd - hourStart
		if span <= 0 {
			continue
		}
		frac := 1 - tr.Avg
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		downSec := int64(math.Round(float64(span) * frac))
		if downSec <= 0 {
			continue
		}
		if downSec > span {
			downSec = span
		}
		raw = append(raw, Outage{
			Start:      hourStart,
			End:        hourEnd,
			Seconds:    downSec,
			Resolution: ResolutionHour,
		})
	}
	sort.Slice(raw, func(i, j int) bool { return raw[i].Start < raw[j].Start })
	return sumSeconds(raw, to-from), raw
}

// HistoryCovers diz se o histórico retido ainda cobre o início do período.
// A margem de uma hora absorve o atraso da limpeza do Zabbix.
func HistoryCovers(retention time.Duration, from, now time.Time) bool {
	if retention <= 0 {
		return false
	}
	earliest := now.Add(-retention).Add(time.Hour)
	return !from.Before(earliest)
}

// ParseRetention entende os prazos do Zabbix, como 7d, 1w e 365d.
func ParseRetention(raw string) time.Duration {
	raw = trimLower(raw)
	if raw == "" || raw == "0" {
		return 0
	}
	i := 0
	for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0
	}
	n := 0
	for _, c := range raw[:i] {
		n = n*10 + int(c-'0')
	}
	switch raw[i:] {
	case "", "s":
		return time.Duration(n) * time.Second
	case "m":
		return time.Duration(n) * time.Minute
	case "h":
		return time.Duration(n) * time.Hour
	case "d":
		return time.Duration(n) * 24 * time.Hour
	case "w":
		return time.Duration(n) * 7 * 24 * time.Hour
	default:
		return 0
	}
}

func outageSeconds(start, end int64, open bool) Outage {
	return Outage{
		Start:      start,
		End:        end,
		Seconds:    end - start,
		Open:       open,
		Resolution: ResolutionSeconds,
	}
}

func mergeOutages(in []Outage) []Outage {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool {
		if in[i].Start == in[j].Start {
			return in[i].End < in[j].End
		}
		return in[i].Start < in[j].Start
	})
	out := make([]Outage, 0, len(in))
	cur := in[0]
	for _, next := range in[1:] {
		if next.Start <= cur.End {
			if next.End > cur.End {
				cur.End = next.End
			}
			cur.Open = cur.Open || next.Open
			cur.Seconds = cur.End - cur.Start
			if next.Resolution == ResolutionHour {
				cur.Resolution = ResolutionHour
			}
			continue
		}
		out = append(out, cur)
		cur = next
	}
	out = append(out, cur)
	return out
}

func sumSeconds(outages []Outage, limit int64) int64 {
	var down int64
	for _, o := range outages {
		down += o.Seconds
	}
	if down < 0 {
		return 0
	}
	if limit >= 0 && down > limit {
		return limit
	}
	return down
}

func trimLower(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c = c - 'A' + 'a'
		}
		b = append(b, c)
	}
	return string(b)
}
