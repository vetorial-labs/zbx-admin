package zabbix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"zbx-admin/internal/report"
)

func TestChooseTriggerPrefersEnabledUnavailable(t *testing.T) {
	id := chooseTrigger([]triggerDTO{
		{TriggerID: "1", Description: "High ICMP ping loss", Status: "0"},
		{TriggerID: "2", Description: "{HOST.NAME} is unavailable by ICMP", Status: "0"},
		{TriggerID: "3", Description: "Unavailable by ICMP ping", Status: "1"},
	})
	if id != "2" {
		t.Fatalf("trigger %s", id)
	}
	id = chooseTrigger([]triggerDTO{
		{TriggerID: "3", Description: "Unavailable by ICMP", Status: "1"},
	})
	if id != "3" {
		t.Fatalf("trigger desativado %s", id)
	}
}

func TestIncidentsIncludePriorOutageAndRecovery(t *testing.T) {
	prev := eventPageSize
	eventPageSize = 10
	t.Cleanup(func() { eventPageSize = prev })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		params, _ := req.Params.(map[string]any)
		var result any
		switch req.Method {
		case "event.get":
			if ids := stringList(params["eventids"]); len(ids) > 0 {
				result = []map[string]string{}
				for _, id := range ids {
					switch id {
					case "r1":
						result = append(result.([]map[string]string), map[string]string{"eventid": "r1", "clock": "1200"})
					case "r2":
						result = append(result.([]map[string]string), map[string]string{"eventid": "r2", "clock": "1600"})
					}
				}
				break
			}
			order, _ := params["sortorder"].(string)
			if order == "DESC" {
				result = []map[string]string{
					{"eventid": "b", "objectid": "10", "clock": "500", "r_eventid": "r1", "value": "1"},
					{"eventid": "old", "objectid": "10", "clock": "100", "r_eventid": "r0", "value": "1"},
				}
			} else {
				result = []map[string]string{
					{"eventid": "a", "objectid": "10", "clock": "1500", "r_eventid": "r2", "value": "1"},
				}
			}
		default:
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": result, "id": req.ID})
	}))
	defer srv.Close()

	c := &Client{url: srv.URL, token: "tok", http: srv.Client()}
	got, err := c.Incidents(context.Background(), []string{"10"}, 1000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	incs := got["10"]
	if len(incs) != 2 {
		t.Fatalf("incidentes %+v", incs)
	}
	down, outages := report.AccountEvents(1000, 2000, incs)
	if down != 300 || len(outages) != 2 {
		t.Fatalf("down %d outages %+v", down, outages)
	}
}

func TestEventsPageSplitsWhenFull(t *testing.T) {
	prev := eventPageSize
	eventPageSize = 1
	t.Cleanup(func() { eventPageSize = prev })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		params, _ := req.Params.(map[string]any)
		ids := stringList(params["objectids"])
		order, _ := params["sortorder"].(string)
		from := numberParam(params["time_from"])
		till := numberParam(params["time_till"])
		var rows []map[string]string
		if order == "ASC" {
			for _, id := range ids {
				const clock int64 = 1500
				if from > 0 && clock < from {
					continue
				}
				if till > 0 && clock > till {
					continue
				}
				rows = append(rows, map[string]string{
					"eventid": "e" + id, "objectid": id, "clock": "1500", "r_eventid": "0", "value": "1",
				})
			}
			if len(rows) > eventPageSize {
				rows = rows[:eventPageSize]
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "result": rows, "id": req.ID})
	}))
	defer srv.Close()

	c := &Client{url: srv.URL, token: "tok", http: srv.Client()}
	got, err := c.Incidents(context.Background(), []string{"1", "2"}, 1000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got["1"]) != 1 || len(got["2"]) != 1 {
		t.Fatalf("incidentes %+v", got)
	}
	if !got["1"][0].Open || !got["2"][0].Open {
		t.Fatal("problemas sem recuperação deveriam seguir abertos")
	}
}

func TestLoginHidesPassword(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		params, _ := req.Params.(map[string]any)
		if params["password"] != "segredo-unico" {
			t.Errorf("senha não enviada")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"error": map[string]any{
				"code": -32500, "message": "Application error.",
				"data": "Login name or password is incorrect.",
			},
			"id": 1,
		})
	}))
	defer srv.Close()
	_, err := Login(context.Background(), srv.URL, "thiago", "segredo-unico")
	if err == nil {
		t.Fatal("esperava erro")
	}
	if got := err.Error(); got != "usuário ou senha rejeitados pelo Zabbix" {
		t.Fatalf("erro %s", got)
	}
}

func numberParam(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	default:
		return 0
	}
}

func stringList(v any) []string {
	switch ids := v.(type) {
	case []any:
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			s, _ := id.(string)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
