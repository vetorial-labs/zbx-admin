package cli

import (
	"fmt"
	"io"

	"zbx-admin/internal/export"
	"zbx-admin/internal/report"
)

func printSummary(w io.Writer, rep report.Report, files export.Files) {
	up, down, accounted, hosts := rep.DisplayTotals()
	fmt.Fprintf(w, "Relatório de disponibilidade ICMP\n")
	fmt.Fprintf(w, "Alvo: %s\n", rep.Target)
	fmt.Fprintf(w, "Período: %s (%s)\n", rep.Period.Display(), rep.Period.From.Location())
	if rep.Capped {
		fmt.Fprintln(w, "O fim do período foi limitado ao momento da geração.")
	}
	fmt.Fprintf(w, "Hosts com ICMP: %d\n", hosts)
	fmt.Fprintf(w, "Duração do período: %s h\n", report.FormatHours(rep.Period.Seconds()))
	fmt.Fprintf(w, "Tempo contabilizado: %s h\n", report.FormatCenti(accounted))
	fmt.Fprintf(w, "Horas UP: %s h\n", report.FormatCenti(up))
	fmt.Fprintf(w, "Horas DOWN: %s h\n", report.FormatCenti(down))
	fmt.Fprintf(w, "Disponibilidade: %s%%\n", report.Percent(up, accounted))
	if missing := rep.WithoutICMP(); len(missing) > 0 {
		fmt.Fprintf(w, "Sem monitoramento ICMP: %d\n", len(missing))
	}
	fmt.Fprintf(w, "CSV: %s\n", files.Resumo)
	fmt.Fprintf(w, "CSV: %s\n", files.Quedas)
	fmt.Fprintf(w, "PDF: %s\n", files.PDF)
}
