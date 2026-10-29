package cli

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"zbx-admin/internal/resolve"
	"zbx-admin/internal/zabbix"
)

func newHostCmd(flags *connFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Busca hosts",
	}
	search := &cobra.Command{
		Use:   "search <termo>",
		Short: "Procura hosts pelo nome visível ou nome técnico",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			api, err := connect(cmd, flags)
			if err != nil {
				return err
			}
			hosts, err := api.SearchHosts(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(hosts) == 0 {
				return fmt.Errorf("nenhum host encontrado para %q", args[0])
			}
			w := tabwriter.NewWriter(stdout(cmd), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tSTATUS\tNOME TÉCNICO\tNOME")
			for _, h := range hosts {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.ID, h.StatusLabel(), h.Technical, h.Visible)
			}
			return w.Flush()
		},
	}
	cmd.AddCommand(search)
	return cmd
}

func newGrupoCmd(flags *connFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "grupo",
		Short: "Busca grupos de hosts",
	}
	search := &cobra.Command{
		Use:   "search <termo>",
		Short: "Procura grupos de hosts pelo nome",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			api, err := connect(cmd, flags)
			if err != nil {
				return err
			}
			groups, err := api.SearchGroups(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if len(groups) == 0 {
				return fmt.Errorf("nenhum grupo encontrado para %q", args[0])
			}
			w := tabwriter.NewWriter(stdout(cmd), 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tHOSTS\tNOME")
			for _, g := range groups {
				fmt.Fprintf(w, "%s\t%d\t%s\n", g.ID, g.MonitoredHosts, g.Name)
			}
			return w.Flush()
		},
	}
	cmd.AddCommand(search)
	return cmd
}

func connect(cmd *cobra.Command, flags *connFlags) (*zabbix.Client, error) {
	cfg, err := loadSettings(cmd, flags)
	if err != nil {
		return nil, err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	return zabbix.Login(ctx, cfg.URL, cfg.Username, cfg.Password)
}

func pickHost(ctx context.Context, api *zabbix.Client, term string) (zabbix.Host, error) {
	if digits(term) {
		host, ok, err := api.HostByID(ctx, term)
		if err != nil {
			return zabbix.Host{}, err
		}
		if ok {
			return host, nil
		}
	}
	list, err := api.SearchHosts(ctx, term)
	if err != nil {
		return zabbix.Host{}, err
	}
	cands := make([]resolve.Candidate, len(list))
	for i, h := range list {
		cands[i] = resolve.Candidate{ID: h.ID, Primary: h.Visible, Secondary: h.Technical}
	}
	picked, err := resolve.Pick("host", term, cands)
	if err != nil {
		return zabbix.Host{}, err
	}
	for _, h := range list {
		if h.ID == picked.ID {
			return h, nil
		}
	}
	return zabbix.Host{}, fmt.Errorf("nenhum host encontrado para %q", term)
}

func pickGroup(ctx context.Context, api *zabbix.Client, term string) (zabbix.Group, error) {
	if digits(term) {
		group, ok, err := api.GroupByID(ctx, term)
		if err != nil {
			return zabbix.Group{}, err
		}
		if ok {
			return group, nil
		}
	}
	list, err := api.SearchGroups(ctx, term)
	if err != nil {
		return zabbix.Group{}, err
	}
	cands := make([]resolve.Candidate, len(list))
	for i, g := range list {
		cands[i] = resolve.Candidate{ID: g.ID, Primary: g.Name}
	}
	picked, err := resolve.Pick("grupo", term, cands)
	if err != nil {
		return zabbix.Group{}, err
	}
	for _, g := range list {
		if g.ID == picked.ID {
			return g, nil
		}
	}
	return zabbix.Group{}, fmt.Errorf("nenhum grupo encontrado para %q", term)
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
