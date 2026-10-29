package cli

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"zbx-admin/internal/report"
)

func TestConfigSetShowHidesPassword(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var out bytes.Buffer
	root := newRoot()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"config", "set", "--url", "zabbix.example", "--user", "thiago", "--password", "segredo"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Configuração salva") {
		t.Fatalf("saída %s", out.String())
	}

	out.Reset()
	root = newRoot()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"config", "show"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if strings.Contains(text, "segredo") {
		t.Fatalf("a senha apareceu: %s", text)
	}
	if !strings.Contains(text, "senha: configurada") || !strings.Contains(text, "usuario: thiago") {
		t.Fatalf("show %s", text)
	}
	if !strings.Contains(text, "/api_jsonrpc.php") {
		t.Fatalf("url não normalizada: %s", text)
	}
}

func TestReportCommandWritesFiles(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().In(loc)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	prevStart := monthStart.AddDate(0, -1, 0)
	outageStart := time.Date(prevStart.Year(), prevStart.Month(), 2, 10, 0, 0, 0, loc)
	outageEnd := outageStart.Add(time.Hour)
	period, _, err := report.Resolve(loc, now, "mes-anterior", "", "")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
			ID     int            `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var result any
		switch req.Method {
		case "user.login":
			result = "token"
		case "apiinfo.version":
			result = "6.4.3"
		case "host.get":
			result = []map[string]string{{
				"hostid": "10", "host": "roteador-teste", "name": "Roteador Teste", "status": "0",
			}}
		case "item.get":
			result = []any{map[string]any{
				"itemid": "99", "hostid": "10", "key_": "icmpping", "history": "1w",
				"status": "0", "value_type": "3",
				"triggers": []map[string]string{{
					"triggerid": "77", "description": "Roteador Teste is unavailable by ICMP", "status": "0",
				}},
			}}
		case "event.get":
			result = eventsFor(req.Params, outageStart.Unix(), outageEnd.Unix(), period.From.Unix(), period.To.Unix())
		default:
			http.Error(w, req.Method, http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result, "id": req.ID})
	}))
	defer srv.Close()

	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	var out bytes.Buffer
	root := newRoot()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--url", srv.URL, "--user", "thiago", "--password", "segredo",
		"relatorio", "disponibilidade", "--host", "10", "--periodo", "mes-anterior", "--saida", dir,
	})
	root.SetContext(context.Background())
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "Tempo contabilizado: "+report.FormatHours(period.Seconds())+" h") {
		t.Fatalf("resumo:\n%s", text)
	}
	if !strings.Contains(text, "Horas DOWN: 1.00 h") {
		t.Fatalf("down:\n%s", text)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("arquivos %d", len(entries))
	}
	var resumoPath string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "-resumo.csv") {
			resumoPath = dir + "/" + e.Name()
		}
	}
	raw, err := os.ReadFile(resumoPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(raw), "\uFEFF"))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][3] != "1.00" || rows[1][4] != report.FormatHours(period.Seconds()) || rows[1][7] != "evento" {
		t.Fatalf("csv %v", rows)
	}
}

func eventsFor(params map[string]any, start, end, periodFrom, periodTo int64) any {
	if ids, ok := params["eventids"].([]any); ok && len(ids) > 0 {
		return []map[string]string{{"eventid": "r1", "clock": itoa(end)}}
	}
	order, _ := params["sortorder"].(string)
	if order == "DESC" {
		return []map[string]string{}
	}
	from := int64(0)
	if v, ok := params["time_from"].(float64); ok {
		from = int64(v)
	}
	if start < from || start >= periodTo || start < periodFrom {
		return []map[string]string{}
	}
	return []map[string]string{{
		"eventid": "e1", "objectid": "77", "clock": itoa(start), "r_eventid": "r1", "value": "1",
	}}
}

func itoa(n int64) string {
	return strconv.FormatInt(n, 10)
}
