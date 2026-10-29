package export

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-pdf/fpdf"
	"zbx-admin/internal/report"
)

const (
	fontName = "dejavu"
	margin   = 10.0
	lineH    = 4.2
)

// writePDF gera o relatório em A4 paisagem.
func writePDF(path string, rep report.Report) error {
	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetCompression(false)
	pdf.SetMargins(margin, 12, margin)
	pdf.SetAutoPageBreak(false, 0)
	pdf.AddUTF8FontFromBytes(fontName, "", fontRegular)
	pdf.AddUTF8FontFromBytes(fontName, "B", fontBold)
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("não foi possível carregar a fonte do PDF: %w", err)
	}
	pdf.AliasNbPages("")
	pdf.SetFooterFunc(func() {
		pdf.SetY(-10)
		pdf.SetFont(fontName, "", 8)
		pdf.SetTextColor(80, 80, 80)
		pdf.CellFormat(0, 6, fmt.Sprintf("Página %d/{nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})
	pdf.AddPage()

	d := &doc{pdf: pdf, rep: rep}
	d.title()
	d.section("Resumo")
	d.table(summaryCols(), summaryRows(rep))
	d.gap()
	d.section("Períodos de DOWN")
	if rows := outageRows(rep); len(rows) == 0 {
		d.note("Nenhuma queda no período.")
	} else {
		d.table(outageCols(), rows)
		if hasHourResolution(rep) {
			d.note("Resolução hora: a duração é a fração indisponível da hora, pela média da tendência.")
		}
	}
	if missing := rep.WithoutICMP(); len(missing) > 0 {
		d.gap()
		d.section("Sem monitoramento ICMP")
		d.note("Estes hosts ficam fora do tempo contabilizado.")
		for _, h := range missing {
			d.note("• " + h.Visible)
		}
	}
	d.gap()
	d.totals()
	if err := pdf.Error(); err != nil {
		return fmt.Errorf("não foi possível montar o PDF: %w", err)
	}
	if err := pdf.OutputFileAndClose(path); err != nil {
		return fmt.Errorf("não foi possível gravar %s: %w", path, err)
	}
	return nil
}

type doc struct {
	pdf *fpdf.Fpdf
	rep report.Report
}

type column struct {
	title string
	width float64
	align string
}

func (d *doc) title() {
	pdf := d.pdf
	pdf.SetTextColor(20, 20, 20)
	pdf.SetFont(fontName, "B", 16)
	pdf.CellFormat(0, 8, "Relatório de disponibilidade ICMP", "", 1, "L", false, 0, "")
	pdf.SetFont(fontName, "", 9)
	version := d.rep.Version
	if version != "" {
		version = "  ·  Zabbix " + version
	}
	pdf.CellFormat(0, 5, "Origem: "+d.rep.ServerURL+version, "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 5, "Alvo: "+d.rep.Target, "", 1, "L", false, 0, "")
	loc := d.rep.Period.From.Location().String()
	pdf.CellFormat(0, 5, "Período: "+d.rep.Period.Label+"  ·  "+d.rep.Period.Display()+"  ("+loc+")", "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 5, "Gerado em: "+d.rep.Generated.In(d.rep.Period.From.Location()).Format("02/01/2006 15:04:05"), "", 1, "L", false, 0, "")
	if d.rep.Capped {
		pdf.CellFormat(0, 5, "O fim do período foi limitado ao momento da geração.", "", 1, "L", false, 0, "")
	}
	pdf.Ln(2)
}

func (d *doc) runningHeader() {
	pdf := d.pdf
	pdf.SetFont(fontName, "B", 10)
	pdf.SetTextColor(20, 20, 20)
	pdf.CellFormat(0, 6, "Disponibilidade ICMP — "+d.rep.Target, "", 1, "L", false, 0, "")
	pdf.Ln(1)
}

func (d *doc) section(text string) {
	d.ensure(8, false)
	d.pdf.SetFont(fontName, "B", 12)
	d.pdf.SetTextColor(20, 20, 20)
	d.pdf.CellFormat(0, 7, text, "", 1, "L", false, 0, "")
}

func (d *doc) note(text string) {
	d.ensure(6, false)
	d.pdf.SetFont(fontName, "", 9)
	d.pdf.SetTextColor(40, 40, 40)
	d.pdf.MultiCell(0, 4.5, text, "", "L", false)
}

func (d *doc) gap() {
	d.pdf.Ln(3)
}

func (d *doc) totals() {
	up, down, accounted, hosts := d.rep.DisplayTotals()
	lines := []string{
		"Duração do período: " + report.FormatHours(d.rep.Period.Seconds()) + " h (" + report.FormatHMS(d.rep.Period.Seconds()) + ")",
		"Hosts com ICMP: " + strconv.Itoa(hosts),
		"Tempo contabilizado: " + report.FormatCenti(accounted) + " h",
		"Horas UP: " + report.FormatCenti(up) + " h",
		"Horas DOWN: " + report.FormatCenti(down) + " h",
		"Disponibilidade: " + report.Percent(up, accounted) + "%",
	}
	h := 4 + float64(len(lines))*5
	d.ensure(h, false)
	pdf := d.pdf
	pdf.SetFillColor(235, 242, 236)
	pdf.SetFont(fontName, "B", 11)
	pdf.SetTextColor(20, 20, 20)
	y := pdf.GetY()
	pdf.Rect(margin, y, d.width(), h, "F")
	pdf.SetXY(margin+2, y+2)
	for _, line := range lines {
		if strings.HasPrefix(line, "Tempo contabilizado") {
			pdf.SetFont(fontName, "B", 11)
		} else {
			pdf.SetFont(fontName, "", 10)
		}
		pdf.CellFormat(d.width()-4, 5, line, "", 1, "L", false, 0, "")
		pdf.SetX(margin + 2)
	}
}

func hasHourResolution(rep report.Report) bool {
	for _, host := range rep.Hosts {
		for _, outage := range host.Outages {
			if outage.Resolution == report.ResolutionHour {
				return true
			}
		}
	}
	return false
}

func summaryCols() []column {
	return []column{
		{"Host", 107, "L"},
		{"Horas UP", 28, "R"},
		{"Horas DOWN", 30, "R"},
		{"Contabilizado", 36, "R"},
		{"Disp. %", 28, "R"},
		{"Quedas", 22, "R"},
		{"Fonte", 26, "L"},
	}
}

func summaryRows(rep report.Report) [][]string {
	var rows [][]string
	for _, h := range rep.Hosts {
		if !h.HasICMP {
			continue
		}
		up, down, accounted := report.SplitHours(rep.Period.Seconds(), h.DownSeconds)
		rows = append(rows, []string{
			h.Visible,
			report.FormatCenti(up),
			report.FormatCenti(down),
			report.FormatCenti(accounted),
			report.Percent(up, accounted),
			strconv.Itoa(len(h.Outages)),
			h.Source,
		})
	}
	return rows
}

func outageCols() []column {
	return []column{
		{"Host", 90, "L"},
		{"Início", 46, "L"},
		{"Fim", 46, "L"},
		{"Horas", 28, "R"},
		{"Duração", 25, "R"},
		{"Em andamento", 20, "L"},
		{"Resolução", 22, "L"},
	}
}

func outageRows(rep report.Report) [][]string {
	loc := rep.Period.From.Location()
	var rows [][]string
	for _, h := range rep.Hosts {
		for _, o := range h.Outages {
			ongoing := "não"
			if o.Open {
				ongoing = "sim"
			}
			rows = append(rows, []string{
				h.Visible,
				report.FormatClock(loc, o.Start),
				report.FormatClock(loc, o.End),
				report.FormatHours(o.Seconds),
				report.FormatHMS(o.Seconds),
				ongoing,
				o.Resolution,
			})
		}
	}
	return rows
}

func (d *doc) table(cols []column, rows [][]string) {
	if len(rows) == 0 {
		d.note("Nenhum host com monitoramento ICMP.")
		return
	}
	d.headerRow(cols)
	for _, row := range rows {
		h := d.rowHeight(cols, row, false)
		if d.pdf.GetY()+h > d.limitY() {
			d.pdf.AddPage()
			d.runningHeader()
			d.headerRow(cols)
		}
		d.drawRow(cols, row, h, false)
	}
}

func (d *doc) headerRow(cols []column) {
	texts := make([]string, len(cols))
	for i, c := range cols {
		texts[i] = c.title
	}
	h := d.rowHeight(cols, texts, true)
	if d.pdf.GetY()+h > d.limitY() {
		d.pdf.AddPage()
		d.runningHeader()
	}
	d.drawRow(cols, texts, h, true)
}

func (d *doc) rowHeight(cols []column, texts []string, header bool) float64 {
	style := ""
	size := 8.0
	if header {
		style = "B"
		size = 8
	}
	d.pdf.SetFont(fontName, style, size)
	maxLines := 1
	for i, text := range texts {
		n := len(d.pdf.SplitLines([]byte(text), cols[i].width-2))
		if n > maxLines {
			maxLines = n
		}
	}
	return float64(maxLines)*lineH + 1.2
}

func (d *doc) drawRow(cols []column, texts []string, h float64, header bool) {
	pdf := d.pdf
	if header {
		pdf.SetFillColor(226, 232, 240)
		pdf.SetFont(fontName, "B", 8)
	} else {
		pdf.SetFillColor(255, 255, 255)
		pdf.SetFont(fontName, "", 8)
	}
	pdf.SetTextColor(20, 20, 20)
	pdf.SetDrawColor(180, 180, 180)
	x := margin
	y := pdf.GetY()
	for i, c := range cols {
		pdf.Rect(x, y, c.width, h, "FD")
		pdf.SetXY(x+1, y+0.6)
		pdf.MultiCell(c.width-2, lineH, texts[i], "", c.align, false)
		x += c.width
	}
	pdf.SetXY(margin, y+h)
}

func (d *doc) ensure(h float64, redrawTable bool) {
	if d.pdf.GetY()+h <= d.limitY() {
		return
	}
	d.pdf.AddPage()
	d.runningHeader()
	_ = redrawTable
}

func (d *doc) limitY() float64 {
	_, pageH := d.pdf.GetPageSize()
	return pageH - 14
}

func (d *doc) width() float64 {
	w, _ := d.pdf.GetPageSize()
	return w - 2*margin
}
