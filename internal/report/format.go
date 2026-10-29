package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Centihours arredonda segundos para centésimos de hora.
func Centihours(seconds int64) int64 {
	if seconds < 0 {
		return -Centihours(-seconds)
	}
	return (seconds*100 + 1800) / 3600
}

// FormatCenti formata centésimos de hora com duas casas.
func FormatCenti(centi int64) string {
	sign := ""
	if centi < 0 {
		sign = "-"
		centi = -centi
	}
	return fmt.Sprintf("%s%d.%02d", sign, centi/100, centi%100)
}

// FormatHours formata segundos em horas decimais com duas casas.
func FormatHours(seconds int64) string {
	return FormatCenti(Centihours(seconds))
}

// SplitHours reparte o período em UP e DOWN já arredondados, de modo que a soma feche.
func SplitHours(periodSeconds, downSeconds int64) (up, down, accounted int64) {
	accounted = Centihours(periodSeconds)
	down = Centihours(downSeconds)
	if down < 0 {
		down = 0
	}
	if down > accounted {
		down = accounted
	}
	up = accounted - down
	return up, down, accounted
}

// FormatHMS formata segundos como H:MM:SS.
func FormatHMS(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := seconds / 3600
	m := (seconds % 3600) / 60
	s := seconds % 60
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

// Percent formata a disponibilidade com duas casas.
func Percent(up, total int64) string {
	if total <= 0 {
		return "0.00"
	}
	return strconv.FormatFloat(100*float64(up)/float64(total), 'f', 2, 64)
}

// FormatClock formata um unix time no fuso do relatório.
func FormatClock(loc *time.Location, unix int64) string {
	if loc == nil {
		loc = time.Local
	}
	return time.Unix(unix, 0).In(loc).Format("02/01/2006 15:04:05")
}

// Slug produz um trecho seguro para nome de arquivo.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prev := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			prev = false
			continue
		}
		if !prev {
			b.WriteByte('_')
			prev = true
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 60 {
		out = strings.Trim(out[:60], "_")
	}
	if out == "" {
		return "relatorio"
	}
	return out
}

// BaseName é o prefixo dos arquivos do relatório.
func BaseName(title string, p Period) string {
	start, end := p.FileDates()
	return "disponibilidade_" + Slug(title) + "_" + start + "_" + end
}
