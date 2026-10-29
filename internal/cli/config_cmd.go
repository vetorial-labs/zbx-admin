package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"zbx-admin/internal/config"
)

func newConfigCmd(flags *connFlags) *cobra.Command {
	var timezone string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Mostra ou grava a conexão em ~/.zbx-admin",
	}
	set := &cobra.Command{
		Use:   "set",
		Short: "Grava URL, usuário, senha e fuso",
		Long: `Grava a conexão em ~/.zbx-admin/config.yaml.

O diretório fica com permissão 0700 e o arquivo com 0600.
Se --password for omitido e ainda não houver senha salva, a CLI pergunta no terminal.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runConfigSet(cmd, flags, timezone)
		},
	}
	set.Flags().StringVar(&timezone, "timezone", "", "Fuso dos períodos (padrão America/Sao_Paulo)")
	show := &cobra.Command{
		Use:   "show",
		Short: "Mostra a configuração, sem a senha",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			return runConfigShow(cmd)
		},
	}
	cmd.AddCommand(set, show)
	return cmd
}

func runConfigSet(cmd *cobra.Command, flags *connFlags, timezone string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if flags.changed(cmd, "url") {
		cfg.URL = *flags.url
	}
	if flags.changed(cmd, "user") {
		cfg.Username = *flags.user
	}
	if flags.changed(cmd, "password") {
		if strings.TrimSpace(*flags.password) == "" {
			return fmt.Errorf("senha vazia")
		}
		cfg.Password = *flags.password
	}
	if cmd.Flags().Changed("timezone") {
		cfg.Timezone = timezone
	}
	if cfg.URL == "" || cfg.Username == "" {
		return fmt.Errorf("informe --url e --user")
	}
	if cfg.Password == "" {
		cfg.Password, err = readPassword()
		if err != nil {
			return err
		}
	}
	cfg.URL, err = config.NormalizeURL(cfg.URL)
	if err != nil {
		return err
	}
	if cfg.Timezone == "" {
		cfg.Timezone = config.DefaultTimezone
	}
	if _, err := time.LoadLocation(cfg.Timezone); err != nil {
		return fmt.Errorf("fuso horário inválido: %s", cfg.Timezone)
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	path, err := config.Path()
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout(cmd), "Configuração salva em %s\n", path)
	return nil
}

func runConfigShow(cmd *cobra.Command) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	path, err := config.Path()
	if err != nil {
		return err
	}
	if cfg.URL == "" && cfg.Username == "" {
		return fmt.Errorf("não há configuração em %s", path)
	}
	senha := "ausente"
	if cfg.Password != "" {
		senha = "configurada"
	}
	tz := cfg.Timezone
	if tz == "" {
		tz = config.DefaultTimezone
	}
	fmt.Fprintf(stdout(cmd), "arquivo: %s\nurl: %s\nusuario: %s\nsenha: %s\nfuso: %s\n", path, cfg.URL, cfg.Username, senha, tz)
	return nil
}
