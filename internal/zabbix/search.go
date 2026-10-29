package zabbix

import (
	"context"
	"strconv"
	"strings"
)

// Host é um host do Zabbix.
type Host struct {
	ID        string
	Technical string
	Visible   string
	Status    string
}

// Monitored indica host habilitado.
func (h Host) Monitored() bool {
	return h.Status == "0"
}

// StatusLabel traduz o status do host.
func (h Host) StatusLabel() string {
	if h.Monitored() {
		return "monitorado"
	}
	return "desativado"
}

// Group é um grupo de hosts.
type Group struct {
	ID             string
	Name           string
	MonitoredHosts int
}

type hostDTO struct {
	HostID string `json:"hostid"`
	Host   string `json:"host"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

func (d hostDTO) host() Host {
	name := d.Name
	if name == "" {
		name = d.Host
	}
	return Host{ID: d.HostID, Technical: d.Host, Visible: name, Status: d.Status}
}

type groupDTO struct {
	GroupID string `json:"groupid"`
	Name    string `json:"name"`
}

// SearchHosts procura por nome visível ou nome técnico.
func (c *Client) SearchHosts(ctx context.Context, term string) ([]Host, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, errEmpty("host")
	}
	var rows []hostDTO
	err := c.call(ctx, "host.get", map[string]any{
		"output":      []string{"hostid", "host", "name", "status"},
		"search":      map[string]string{"name": term, "host": term},
		"searchByAny": true,
		"sortfield":   "name",
		"sortorder":   "ASC",
	}, &rows)
	if err != nil {
		return nil, err
	}
	return hostsFrom(rows), nil
}

// HostByID busca um host pelo ID. O segundo retorno é false quando ele não existe.
func (c *Client) HostByID(ctx context.Context, id string) (Host, bool, error) {
	var rows []hostDTO
	err := c.call(ctx, "host.get", map[string]any{
		"output":  []string{"hostid", "host", "name", "status"},
		"hostids": []string{id},
	}, &rows)
	if err != nil {
		return Host{}, false, err
	}
	if len(rows) == 0 {
		return Host{}, false, nil
	}
	return rows[0].host(), true, nil
}

// SearchGroups procura grupos pelo nome.
func (c *Client) SearchGroups(ctx context.Context, term string) ([]Group, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return nil, errEmpty("grupo")
	}
	var rows []groupDTO
	err := c.call(ctx, "hostgroup.get", map[string]any{
		"output":    []string{"groupid", "name"},
		"search":    map[string]string{"name": term},
		"sortfield": "name",
		"sortorder": "ASC",
	}, &rows)
	if err != nil {
		return nil, err
	}
	out := make([]Group, 0, len(rows))
	for _, row := range rows {
		n, err := c.CountMonitored(ctx, row.GroupID)
		if err != nil {
			return nil, err
		}
		out = append(out, Group{ID: row.GroupID, Name: row.Name, MonitoredHosts: n})
	}
	return out, nil
}

// GroupByID busca um grupo pelo ID.
func (c *Client) GroupByID(ctx context.Context, id string) (Group, bool, error) {
	var rows []groupDTO
	err := c.call(ctx, "hostgroup.get", map[string]any{
		"output":   []string{"groupid", "name"},
		"groupids": []string{id},
	}, &rows)
	if err != nil {
		return Group{}, false, err
	}
	if len(rows) == 0 {
		return Group{}, false, nil
	}
	n, err := c.CountMonitored(ctx, rows[0].GroupID)
	if err != nil {
		return Group{}, false, err
	}
	return Group{ID: rows[0].GroupID, Name: rows[0].Name, MonitoredHosts: n}, true, nil
}

// CountMonitored conta os hosts habilitados do grupo.
func (c *Client) CountMonitored(ctx context.Context, groupID string) (int, error) {
	var raw jsonNumber
	err := c.call(ctx, "host.get", map[string]any{
		"groupids":        []string{groupID},
		"monitored_hosts": true,
		"countOutput":     true,
	}, &raw)
	if err != nil {
		return 0, err
	}
	return raw.Int(), nil
}

// HostsInGroup devolve os hosts monitorados do grupo, em ordem de nome.
func (c *Client) HostsInGroup(ctx context.Context, groupID string) ([]Host, error) {
	var rows []hostDTO
	err := c.call(ctx, "host.get", map[string]any{
		"output":          []string{"hostid", "host", "name", "status"},
		"groupids":        []string{groupID},
		"monitored_hosts": true,
		"sortfield":       "name",
		"sortorder":       "ASC",
	}, &rows)
	if err != nil {
		return nil, err
	}
	return hostsFrom(rows), nil
}

func hostsFrom(rows []hostDTO) []Host {
	out := make([]Host, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.host())
	}
	return out
}

// jsonNumber aceita o countOutput, que o Zabbix manda como string ou número.
type jsonNumber string

func (n *jsonNumber) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	*n = jsonNumber(s)
	return nil
}

func (n jsonNumber) Int() int {
	v, err := strconv.Atoi(string(n))
	if err != nil {
		return 0
	}
	return v
}

type emptyTerm string

func (e emptyTerm) Error() string { return string(e) }

func errEmpty(kind string) error {
	return emptyTerm("informe o " + kind + " para a busca")
}
