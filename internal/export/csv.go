package export

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"zbx-admin/internal/report"
)

// Files são os três arquivos de um relatório.
type Files struct {
	Resumo string
	Quedas string
	PDF    string
}

// Write grava o CSV de resumo, o CSV de quedas e o PDF no diretório.
func Write(dir string, rep report.Report) (Files, error) {
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Files{}, fmt.Errorf("não foi possível criar %s: %w", dir, err)
	}
	base := report.BaseName(rep.Target, rep.Period)
	out := Files{
		Resumo: filepath.Join(dir, base+"-resumo.csv"),
		Quedas: filepath.Join(dir, base+"-quedas.csv"),
		PDF:    filepath.Join(dir, base+".pdf"),
	}
	if err := writeFile(out.Resumo, func(w io.Writer) error { return writeResumo(w, rep) }); err != nil {
		return Files{}, err
	}
	if err := writeFile(out.Quedas, func(w io.Writer) error { return writeQuedas(w, rep) }); err != nil {
		return Files{}, err
	}
	if err := writePDF(out.PDF, rep); err != nil {
		return Files{}, err
	}
	return out, nil
}

func writeFile(path string, fn func(io.Writer) error) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("não foi possível criar %s: %w", path, err)
	}
	defer f.Close()
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	return fn(f)
}

func writeResumo(w io.Writer, rep report.Report) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"host", "host_tecnico", "horas_up", "horas_down", "horas_contabilizadas",
		"disponibilidade_pct", "quedas", "fonte",
	}); err != nil {
		return err
	}
	var up, down, accounted int64
	var monitored int
	period := rep.Period.Seconds()
	for _, h := range rep.Hosts {
		if !h.HasICMP {
			if err := cw.Write([]string{h.Visible, h.Technical, "", "", "", "", "0", report.SourceNone}); err != nil {
				return err
			}
			continue
		}
		u, d, a := report.SplitHours(period, h.DownSeconds)
		up += u
		down += d
		accounted += a
		monitored++
		if err := cw.Write([]string{
			h.Visible,
			h.Technical,
			report.FormatCenti(u),
			report.FormatCenti(d),
			report.FormatCenti(a),
			report.Percent(u, a),
			strconv.Itoa(len(h.Outages)),
			h.Source,
		}); err != nil {
			return err
		}
	}
	if monitored > 0 {
		if err := cw.Write([]string{
			"TOTAL",
			"",
			report.FormatCenti(up),
			report.FormatCenti(down),
			report.FormatCenti(accounted),
			report.Percent(up, accounted),
			"",
			"",
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func writeQuedas(w io.Writer, rep report.Report) error {
	cw := csv.NewWriter(w)
	loc := rep.Period.From.Location()
	if err := cw.Write([]string{"host", "inicio", "fim", "duracao_horas", "em_andamento", "resolucao"}); err != nil {
		return err
	}
	for _, h := range rep.Hosts {
		for _, o := range h.Outages {
			ongoing := "nao"
			if o.Open {
				ongoing = "sim"
			}
			if err := cw.Write([]string{
				h.Visible,
				report.FormatClock(loc, o.Start),
				report.FormatClock(loc, o.End),
				report.FormatHours(o.Seconds),
				ongoing,
				o.Resolution,
			}); err != nil {
				return err
			}
		}
	}
	cw.Flush()
	return cw.Error()
}
