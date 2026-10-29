// Package avail monta o relatório de disponibilidade ICMP.
package avail

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"zbx-admin/internal/report"
	"zbx-admin/internal/zabbix"
)

// Build calcula a disponibilidade dos hosts no período.
func Build(ctx context.Context, api *zabbix.Client, targetName, kind string, hosts []zabbix.Host, period report.Period, now time.Time, serverURL, version string, capped bool) (report.Report, error) {
	if len(hosts) == 0 {
		return report.Report{}, fmt.Errorf("nenhum host para o relatório")
	}
	ids := make([]string, 0, len(hosts))
	for _, h := range hosts {
		ids = append(ids, h.ID)
	}
	monitors, err := api.ICMPMonitors(ctx, ids)
	if err != nil {
		return report.Report{}, err
	}
	var triggerIDs []string
	for _, m := range monitors {
		if m.TriggerID != "" {
			triggerIDs = append(triggerIDs, m.TriggerID)
		}
	}
	incidents, err := api.Incidents(ctx, triggerIDs, period.From.Unix(), period.To.Unix())
	if err != nil {
		return report.Report{}, err
	}

	from := period.From.Unix()
	to := period.To.Unix()
	span := period.Seconds()
	results := make([]report.HostResult, 0, len(hosts))
	for _, host := range hosts {
		mon, ok := monitors[host.ID]
		if !ok || mon.ItemID == "" {
			results = append(results, report.HostResult{
				ID:        host.ID,
				Technical: host.Technical,
				Visible:   host.Visible,
				Source:    report.SourceNone,
			})
			continue
		}
		if mon.TriggerID != "" {
			down, outages := report.AccountEvents(from, to, incidents[mon.TriggerID])
			results = append(results, report.NewHostResult(host.ID, host.Technical, host.Visible, report.SourceEvent, span, down, outages))
			continue
		}
		result, err := fallback(ctx, api, host, mon, from, to, span, now)
		if err != nil {
			return report.Report{}, err
		}
		results = append(results, result)
	}
	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Visible) < strings.ToLower(results[j].Visible)
	})
	return report.Report{
		Target:    targetName,
		Kind:      kind,
		Period:    period,
		Generated: now,
		ServerURL: serverURL,
		Version:   version,
		Capped:    capped,
		Hosts:     results,
	}, nil
}

func fallback(ctx context.Context, api *zabbix.Client, host zabbix.Host, mon zabbix.Monitor, from, to, span int64, now time.Time) (report.HostResult, error) {
	retention := report.ParseRetention(mon.History)
	if report.HistoryCovers(retention, time.Unix(from, 0), now) {
		initial, samples, err := api.History(ctx, mon.ItemID, from, to)
		if err != nil {
			return report.HostResult{}, err
		}
		if initial != nil || len(samples) > 0 {
			down, outages := report.AccountHistory(from, to, initial, samples)
			return report.NewHostResult(host.ID, host.Technical, host.Visible, report.SourceHistory, span, down, outages), nil
		}
	}
	trends, err := api.Trends(ctx, mon.ItemID, from, to)
	if err != nil {
		return report.HostResult{}, err
	}
	down, outages := report.AccountTrends(from, to, trends)
	return report.NewHostResult(host.ID, host.Technical, host.Visible, report.SourceTrend, span, down, outages), nil
}
