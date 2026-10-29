package report

import (
	"fmt"
	"strings"
	"time"
)

// Period é um intervalo [From, To). To é exclusivo.
type Period struct {
	From  time.Time
	To    time.Time
	Label string
}

// Seconds devolve a duração do período.
func (p Period) Seconds() int64 {
	return int64(p.To.Sub(p.From).Seconds())
}

// Display formata o intervalo no fuso do próprio período.
func (p Period) Display() string {
	return p.From.Format("02/01/2006 15:04:05") + " até " + p.To.Format("02/01/2006 15:04:05")
}

// FileDates devolve as datas de início e a última data inclusa, para o nome do arquivo.
func (p Period) FileDates() (string, string) {
	end := p.To.Add(-time.Second)
	return p.From.Format("2006-01-02"), end.Format("2006-01-02")
}

// Resolve interpreta o preset e, no período personalizado, --de e --ate.
// Se o fim estiver no futuro, ele é limitado a now e capped fica verdadeiro.
func Resolve(loc *time.Location, now time.Time, preset, fromRaw, toRaw string) (Period, bool, error) {
	if loc == nil {
		return Period{}, false, fmt.Errorf("fuso horário ausente")
	}
	now = now.In(loc)
	var from, to time.Time
	var label string
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "", "mes-atual", "current-month":
		from, to = currentMonth(now)
		label = "Mês atual"
	case "mes-anterior", "previous-month":
		from, to = previousMonth(now)
		label = "Mês anterior"
	case "personalizado", "custom":
		if strings.TrimSpace(fromRaw) == "" || strings.TrimSpace(toRaw) == "" {
			return Period{}, false, fmt.Errorf("período personalizado exige --de e --ate")
		}
		var err error
		from, err = parseBound(fromRaw, loc, false)
		if err != nil {
			return Period{}, false, fmt.Errorf("início inválido: %w", err)
		}
		to, err = parseBound(toRaw, loc, true)
		if err != nil {
			return Period{}, false, fmt.Errorf("fim inválido: %w", err)
		}
		label = "Período personalizado"
	default:
		return Period{}, false, fmt.Errorf("período %q desconhecido (use mes-atual, mes-anterior ou personalizado)", preset)
	}
	capped := false
	if to.After(now) {
		to = now
		capped = true
	}
	if !to.After(from) {
		return Period{}, false, fmt.Errorf("o período está vazio ou no futuro")
	}
	return Period{From: from, To: to, Label: label}, capped, nil
}

func currentMonth(now time.Time) (time.Time, time.Time) {
	loc := now.Location()
	start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	return start, now
}

func previousMonth(now time.Time) (time.Time, time.Time) {
	loc := now.Location()
	startThis := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	startPrev := startThis.AddDate(0, -1, 0)
	return startPrev, startThis
}

// parseBound aceita data (2006-01-02), data-hora local (2006-01-02T15:04) ou RFC3339.
// Uma data sozinha em end vira o início do dia seguinte, para incluir o dia inteiro.
func parseBound(raw string, loc *time.Location, end bool) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.ParseInLocation("2006-01-02", raw, loc); err == nil {
		if end {
			return t.AddDate(0, 0, 1), nil
		}
		return t, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t, nil
		}
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.In(loc), nil
	}
	return time.Time{}, fmt.Errorf("use 2006-01-02 ou 2006-01-02T15:04")
}
