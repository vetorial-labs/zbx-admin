package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"zbx-admin/internal/avail"
	"zbx-admin/internal/export"
	"zbx-admin/internal/report"
	"zbx-admin/internal/zabbix"
)

func newRelatorioCmd(flags *connFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "relatorio",
		Short: "Gera relatórios",
	}
	var host, group, period, from, to, outDir string
	disp := &cobra.Command{
		Use:   "disponibilidade",
		Short: "Exporta disponibilidade ICMP (UP/DOWN) em CSV e PDF",
		Example: `  zbx-admin relatorio disponibilidade --host BLANKEN --periodo mes-anterior
  zbx-admin relatorio disponibilidade --grupo CLIENTES --periodo personalizado --de 2026-09-01 --ate 2026-09-30`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runAvailability(cmd, flags, host, group, period, from, to, outDir)
		},
	}
	disp.Flags().StringVar(&host, "host", "", "Nome, nome técnico ou ID do host")
	disp.Flags().StringVar(&group, "grupo", "", "Nome ou ID do grupo de hosts")
	disp.Flags().StringVar(&period, "periodo", "mes-atual", "mes-atual, mes-anterior ou personalizado")
	disp.Flags().StringVar(&from, "de", "", "Início (data ou data-hora) do período personalizado")
	disp.Flags().StringVar(&to, "ate", "", "Fim do período personalizado (data inclusiva ou data-hora)")
	disp.Flags().StringVar(&outDir, "saida", ".", "Diretório dos CSV e do PDF")
	disp.MarkFlagsMutuallyExclusive("host", "grupo")
	disp.MarkFlagsOneRequired("host", "grupo")
	cmd.AddCommand(disp)
	return cmd
}

func runAvailability(cmd *cobra.Command, flags *connFlags, hostTerm, groupTerm, preset, fromRaw, toRaw, outDir string) error {
	cfg, err := loadSettings(cmd, flags)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return fmt.Errorf("fuso horário inválido: %s", cfg.Timezone)
	}
	now := time.Now().In(loc)
	period, capped, err := report.Resolve(loc, now, preset, fromRaw, toRaw)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	api, err := zabbix.Login(ctx, cfg.URL, cfg.Username, cfg.Password)
	if err != nil {
		return err
	}
	version, _ := api.Version(ctx)

	var (
		target string
		kind   string
		hosts  []zabbix.Host
	)
	if hostTerm != "" {
		host, err := pickHost(ctx, api, hostTerm)
		if err != nil {
			return err
		}
		target = host.Visible
		kind = "host"
		hosts = []zabbix.Host{host}
	} else {
		group, err := pickGroup(ctx, api, groupTerm)
		if err != nil {
			return err
		}
		list, err := api.HostsInGroup(ctx, group.ID)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return fmt.Errorf("nenhum host monitorado no grupo %s", group.Name)
		}
		target = group.Name
		kind = "grupo"
		hosts = list
	}
	rep, err := avail.Build(ctx, api, target, kind, hosts, period, now, cfg.URL, version, capped)
	if err != nil {
		return err
	}
	files, err := export.Write(outDir, rep)
	if err != nil {
		return err
	}
	printSummary(stdout(cmd), rep, files)
	return nil
}
