package export

import (
	"bytes"
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"zbx-admin/internal/report"
)

func TestWriteCSVAndPDF(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	to := time.Date(2026, 10, 1, 0, 0, 0, 0, loc)
	rep := report.Report{
		Target:    "BLANKEN ASSESSORIA",
		Kind:      "host",
		Period:    report.Period{From: from, To: to, Label: "Mês anterior"},
		Generated: time.Date(2026, 10, 29, 11, 0, 0, 0, loc),
		ServerURL: "https://zabbix.example/api_jsonrpc.php",
		Version:   "6.4.3",
		Hosts: []report.HostResult{
			report.NewHostResult("10", "blanken", "BLANKEN ASSESSORIA", report.SourceEvent, int64(to.Sub(from).Seconds()), 3600, []report.Outage{{
				Start:      from.Add(10 * time.Hour).Unix(),
				End:        from.Add(11 * time.Hour).Unix(),
				Seconds:    3600,
				Resolution: report.ResolutionSeconds,
			}}),
			{
				ID: "11", Technical: "sem", Visible: "Sem ICMP", Source: report.SourceNone,
			},
		},
	}
	dir := t.TempDir()
	files, err := Write(dir, rep)
	if err != nil {
		t.Fatal(err)
	}
	resumo := readCSV(t, files.Resumo)
	if resumo[0][0] != "host" || resumo[0][4] != "horas_contabilizadas" {
		t.Fatalf("cabeçalho %v", resumo[0])
	}
	if resumo[1][0] != "BLANKEN ASSESSORIA" || resumo[1][3] != "1.00" || resumo[1][4] != "720.00" || resumo[1][7] != "evento" {
		t.Fatalf("resumo %v", resumo[1])
	}
	if resumo[2][7] != "sem_item" {
		t.Fatalf("sem item %v", resumo[2])
	}
	if resumo[3][0] != "TOTAL" || resumo[3][4] != "720.00" || resumo[3][3] != "1.00" {
		t.Fatalf("total %v", resumo[3])
	}
	quedas := readCSV(t, files.Quedas)
	if len(quedas) != 2 || quedas[1][4] != "nao" || quedas[1][5] != "segundos" {
		t.Fatalf("quedas %v", quedas)
	}
	raw, err := os.ReadFile(files.PDF)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), "%PDF") {
		t.Fatal("PDF sem cabeçalho")
	}
	if len(raw) < 2000 {
		t.Fatalf("PDF pequeno: %d", len(raw))
	}
	if !bytesContain(raw, "Tempo contabilizado") {
		t.Fatal("o PDF não contém o resumo de tempo contabilizado")
	}
	if filepath.Base(files.PDF) != "disponibilidade_blanken_assessoria_2026-09-01_2026-09-30.pdf" {
		t.Fatalf("nome %s", files.PDF)
	}
}

func readCSV(t *testing.T, path string) [][]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	text = strings.TrimPrefix(text, "\uFEFF")
	rows, err := csv.NewReader(strings.NewReader(text)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func bytesContain(raw []byte, phrase string) bool {
	if strings.Contains(string(raw), phrase) {
		return true
	}
	// A fonte UTF-8 grava o texto como UTF-16BE dentro do PDF.
	var encoded []byte
	for _, r := range phrase {
		encoded = append(encoded, byte(r>>8), byte(r))
	}
	return bytes.Contains(raw, encoded)
}
