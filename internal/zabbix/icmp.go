package zabbix

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"zbx-admin/internal/report"
)

// eventPageSize é o tamanho da página de event.get. Testes reduzem este valor.
var eventPageSize = 800

// Monitor liga um host ao item icmpping e, quando existe, ao trigger de indisponibilidade.
type Monitor struct {
	HostID    string
	ItemID    string
	TriggerID string
	History   string
}

type itemDTO struct {
	ItemID    string       `json:"itemid"`
	HostID    string       `json:"hostid"`
	Key       string       `json:"key_"`
	History   string       `json:"history"`
	Status    string       `json:"status"`
	ValueType string       `json:"value_type"`
	Triggers  []triggerDTO `json:"triggers"`
}

type triggerDTO struct {
	TriggerID   string `json:"triggerid"`
	Description string `json:"description"`
	Status      string `json:"status"`
}

type eventDTO struct {
	EventID  string `json:"eventid"`
	ObjectID string `json:"objectid"`
	Clock    string `json:"clock"`
	REventID string `json:"r_eventid"`
	Value    string `json:"value"`
}

type historyDTO struct {
	ItemID string `json:"itemid"`
	Clock  string `json:"clock"`
	Value  string `json:"value"`
}

type trendDTO struct {
	ItemID   string `json:"itemid"`
	Clock    string `json:"clock"`
	Num      string `json:"num"`
	ValueAvg string `json:"value_avg"`
}

// ICMPMonitors devolve o item icmpping de cada host, indexado por hostid.
func (c *Client) ICMPMonitors(ctx context.Context, hostIDs []string) (map[string]Monitor, error) {
	found := map[string][]itemDTO{}
	for _, chunk := range chunks(hostIDs, 100) {
		var rows []itemDTO
		err := c.call(ctx, "item.get", map[string]any{
			"output":         []string{"itemid", "hostid", "name", "key_", "history", "status", "value_type"},
			"hostids":        chunk,
			"filter":         map[string]string{"key_": "icmpping"},
			"selectTriggers": []string{"triggerid", "description", "status"},
		}, &rows)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			found[row.HostID] = append(found[row.HostID], row)
		}
	}
	out := make(map[string]Monitor, len(found))
	for hostID, items := range found {
		it, ok := ChooseItem(items)
		if !ok {
			continue
		}
		out[hostID] = Monitor{
			HostID:    hostID,
			ItemID:    it.ItemID,
			TriggerID: chooseTrigger(it.Triggers),
			History:   it.History,
		}
	}
	return out, nil
}

// ChooseItem escolhe um item icmpping do host, de preferência habilitado e com trigger.
func ChooseItem(items []itemDTO) (itemDTO, bool) {
	if len(items) == 0 {
		return itemDTO{}, false
	}
	sort.SliceStable(items, func(i, j int) bool {
		return rankItem(items[i]) < rankItem(items[j])
	})
	return items[0], true
}

func rankItem(it itemDTO) int {
	score := 2
	if it.Status == "0" {
		score = 0
	}
	if chooseTrigger(it.Triggers) == "" {
		score++
	}
	return score
}

// chooseTrigger devolve o trigger de indisponibilidade ICMP ligado ao item.
func chooseTrigger(trigs []triggerDTO) string {
	type cand struct {
		id      string
		n       int
		enabled bool
	}
	var cs []cand
	for _, t := range trigs {
		if !strings.Contains(strings.ToLower(t.Description), "unavailable by icmp") {
			continue
		}
		cs = append(cs, cand{id: t.TriggerID, n: len(t.Description), enabled: t.Status == "" || t.Status == "0"})
	}
	if len(cs) == 0 {
		return ""
	}
	sort.Slice(cs, func(i, j int) bool {
		if cs[i].enabled != cs[j].enabled {
			return cs[i].enabled
		}
		return cs[i].n < cs[j].n
	})
	return cs[0].id
}

// Incidents devolve os problemas ICMP que cruzam [from, to), agrupados por trigger.
// A busca inclui o último problema anterior ao período, para quedas que começaram antes.
func (c *Client) Incidents(ctx context.Context, triggerIDs []string, from, to int64) (map[string][]report.Incident, error) {
	out := map[string][]report.Incident{}
	if len(triggerIDs) == 0 || to <= from {
		return out, nil
	}
	events, err := c.eventsOverlapping(ctx, triggerIDs, from, to)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return out, nil
	}
	recoveries := map[string]int64{}
	var recoveryIDs []string
	seenRecovery := map[string]bool{}
	for _, ev := range events {
		if ev.REventID != "" && ev.REventID != "0" && !seenRecovery[ev.REventID] {
			seenRecovery[ev.REventID] = true
			recoveryIDs = append(recoveryIDs, ev.REventID)
		}
	}
	if len(recoveryIDs) > 0 {
		recoveries, err = c.recoveryClocks(ctx, recoveryIDs)
		if err != nil {
			return nil, err
		}
	}
	seenEvent := map[string]bool{}
	for _, ev := range events {
		if seenEvent[ev.EventID] {
			continue
		}
		seenEvent[ev.EventID] = true
		start, err := strconv.ParseInt(ev.Clock, 10, 64)
		if err != nil {
			continue
		}
		inc := report.Incident{Start: start}
		if ev.REventID == "" || ev.REventID == "0" {
			inc.Open = true
		} else if end, ok := recoveries[ev.REventID]; ok && end > 0 {
			inc.End = end
		} else {
			inc.Open = true
		}
		if !inc.Open && inc.End <= from {
			continue
		}
		if inc.Start >= to {
			continue
		}
		out[ev.ObjectID] = append(out[ev.ObjectID], inc)
	}
	return out, nil
}

func (c *Client) eventsOverlapping(ctx context.Context, triggerIDs []string, from, to int64) ([]eventDTO, error) {
	inWindow, err := c.eventsInRange(ctx, triggerIDs, from, to)
	if err != nil {
		return nil, err
	}
	before, err := c.latestBefore(ctx, triggerIDs, from)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var all []eventDTO
	for _, ev := range inWindow {
		if seen[ev.EventID] {
			continue
		}
		seen[ev.EventID] = true
		all = append(all, ev)
	}
	for _, ev := range before {
		if seen[ev.EventID] {
			continue
		}
		seen[ev.EventID] = true
		all = append(all, ev)
	}
	return all, nil
}

// eventsInRange traz os problemas que começaram dentro do período.
func (c *Client) eventsInRange(ctx context.Context, triggerIDs []string, from, to int64) ([]eventDTO, error) {
	if len(triggerIDs) == 0 {
		return nil, nil
	}
	rows, err := c.fetchEvents(ctx, triggerIDs, from, to, "ASC", eventPageSize)
	if err != nil {
		return nil, err
	}
	if len(rows) < eventPageSize {
		return rows, nil
	}
	if len(triggerIDs) > 1 {
		mid := len(triggerIDs) / 2
		left, err := c.eventsInRange(ctx, triggerIDs[:mid], from, to)
		if err != nil {
			return nil, err
		}
		right, err := c.eventsInRange(ctx, triggerIDs[mid:], from, to)
		if err != nil {
			return nil, err
		}
		return append(left, right...), nil
	}
	return c.pageOneTrigger(ctx, triggerIDs[0], from, to)
}

// pageOneTrigger pagina um único trigger pelo relógio quando a página enche.
func (c *Client) pageOneTrigger(ctx context.Context, triggerID string, from, to int64) ([]eventDTO, error) {
	var all []eventDTO
	seen := map[string]bool{}
	cursor := from
	for {
		rows, err := c.fetchEvents(ctx, []string{triggerID}, cursor, to, "ASC", eventPageSize)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		var maxClock int64
		added := 0
		for _, row := range rows {
			clock, _ := strconv.ParseInt(row.Clock, 10, 64)
			if clock > maxClock {
				maxClock = clock
			}
			if seen[row.EventID] {
				continue
			}
			seen[row.EventID] = true
			all = append(all, row)
			added++
		}
		if len(rows) < eventPageSize {
			break
		}
		if maxClock <= cursor {
			cursor++
		} else if added == 0 {
			cursor = maxClock + 1
		} else {
			cursor = maxClock
		}
		if cursor > to {
			break
		}
	}
	return all, nil
}

// latestBefore devolve, por trigger, o problema mais recente que começou antes de before.
func (c *Client) latestBefore(ctx context.Context, triggerIDs []string, before int64) ([]eventDTO, error) {
	if len(triggerIDs) == 0 || before <= 0 {
		return nil, nil
	}
	rows, err := c.fetchEvents(ctx, triggerIDs, 0, before-1, "DESC", eventPageSize)
	if err != nil {
		return nil, err
	}
	latest := map[string]eventDTO{}
	var order []string
	for _, row := range rows {
		if _, ok := latest[row.ObjectID]; ok {
			continue
		}
		latest[row.ObjectID] = row
		order = append(order, row.ObjectID)
	}
	if len(rows) < eventPageSize {
		out := make([]eventDTO, 0, len(order))
		for _, id := range order {
			out = append(out, latest[id])
		}
		return out, nil
	}
	var missing []string
	for _, id := range triggerIDs {
		if _, ok := latest[id]; !ok {
			missing = append(missing, id)
		}
	}
	out := make([]eventDTO, 0, len(order))
	for _, id := range order {
		out = append(out, latest[id])
	}
	if len(missing) == 0 {
		return out, nil
	}
	rest, err := c.latestBefore(ctx, missing, before)
	if err != nil {
		return nil, err
	}
	return append(out, rest...), nil
}

func (c *Client) fetchEvents(ctx context.Context, triggerIDs []string, from, to int64, order string, limit int) ([]eventDTO, error) {
	if len(triggerIDs) == 0 {
		return nil, nil
	}
	params := map[string]any{
		"output":    []string{"eventid", "objectid", "clock", "r_eventid", "value"},
		"objectids": triggerIDs,
		"source":    0,
		"object":    0,
		"value":     1,
		"sortfield": "clock",
		"sortorder": order,
		"limit":     limit,
	}
	if from > 0 {
		params["time_from"] = from
	}
	if to > 0 {
		params["time_till"] = to
	}
	var rows []eventDTO
	if err := c.call(ctx, "event.get", params, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

func (c *Client) recoveryClocks(ctx context.Context, eventIDs []string) (map[string]int64, error) {
	out := make(map[string]int64, len(eventIDs))
	for _, chunk := range chunks(eventIDs, 200) {
		var rows []eventDTO
		err := c.call(ctx, "event.get", map[string]any{
			"output":   []string{"eventid", "clock"},
			"eventids": chunk,
		}, &rows)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			clock, err := strconv.ParseInt(row.Clock, 10, 64)
			if err != nil {
				continue
			}
			out[row.EventID] = clock
		}
	}
	return out, nil
}

// History devolve o estado anterior a from e os pontos dentro de [from, to).
func (c *Client) History(ctx context.Context, itemID string, from, to int64) (*bool, []report.Sample, error) {
	var initial *bool
	before, err := c.historyPoints(ctx, itemID, 0, from, "DESC", 1)
	if err != nil {
		return nil, nil, err
	}
	if len(before) == 1 && before[0].Clock < from {
		down := !before[0].Up
		initial = &down
	}
	var samples []report.Sample
	cursor := from
	for {
		points, err := c.historyPoints(ctx, itemID, cursor, to, "ASC", eventPageSize)
		if err != nil {
			return nil, nil, err
		}
		if len(points) == 0 {
			break
		}
		var maxClock int64
		for _, p := range points {
			if p.Clock >= to {
				continue
			}
			if p.Clock > maxClock {
				maxClock = p.Clock
			}
			samples = append(samples, p)
		}
		if len(points) < eventPageSize {
			break
		}
		if maxClock <= cursor {
			cursor++
		} else {
			cursor = maxClock
		}
		if cursor >= to {
			break
		}
	}
	return initial, samples, nil
}

func (c *Client) historyPoints(ctx context.Context, itemID string, from, to int64, order string, limit int) ([]report.Sample, error) {
	params := map[string]any{
		"output":    []string{"itemid", "clock", "value"},
		"history":   3,
		"itemids":   []string{itemID},
		"sortfield": "clock",
		"sortorder": order,
		"limit":     limit,
	}
	if from > 0 {
		params["time_from"] = from
	}
	if to > 0 {
		params["time_till"] = to
	}
	var rows []historyDTO
	if err := c.call(ctx, "history.get", params, &rows); err != nil {
		return nil, err
	}
	out := make([]report.Sample, 0, len(rows))
	for _, row := range rows {
		clock, err := strconv.ParseInt(row.Clock, 10, 64)
		if err != nil {
			continue
		}
		out = append(out, report.Sample{Clock: clock, Up: row.Value != "0"})
	}
	return out, nil
}

// Trends devolve as médias horárias que cruzam [from, to).
func (c *Client) Trends(ctx context.Context, itemID string, from, to int64) ([]report.Trend, error) {
	// Uma hora que começa antes de from ainda cobre o início do período.
	queryFrom := from - 3600
	if queryFrom < 0 {
		queryFrom = 0
	}
	var rows []trendDTO
	err := c.call(ctx, "trend.get", map[string]any{
		"output":    []string{"itemid", "clock", "num", "value_min", "value_avg", "value_max"},
		"itemids":   []string{itemID},
		"time_from": queryFrom,
		"time_till": to,
	}, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]report.Trend, 0, len(rows))
	for _, row := range rows {
		clock, err := strconv.ParseInt(row.Clock, 10, 64)
		if err != nil {
			continue
		}
		avg, err := strconv.ParseFloat(row.ValueAvg, 64)
		if err != nil {
			continue
		}
		num, _ := strconv.Atoi(row.Num)
		out = append(out, report.Trend{Clock: clock, Avg: avg, Num: num})
	}
	return out, nil
}

func chunks(ids []string, size int) [][]string {
	if size <= 0 || len(ids) == 0 {
		return nil
	}
	var out [][]string
	for len(ids) > 0 {
		n := size
		if n > len(ids) {
			n = len(ids)
		}
		out = append(out, ids[:n])
		ids = ids[n:]
	}
	return out
}
