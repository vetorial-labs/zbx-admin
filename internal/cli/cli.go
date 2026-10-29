// Package cli é a interface de linha de comando do zbx-admin.
package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"zbx-admin/internal/config"
)

// Execute roda a CLI com os argumentos do processo.
func Execute() error {
	return newRoot().Execute()
}

func newRoot() *cobra.Command {
	var flagURL, flagUser, flagPassword string
	root := &cobra.Command{
		Use:           "zbx-admin",
		Short:         "Relatórios de disponibilidade ICMP no Zabbix",
		Version:       Version,
		SilenceUsage:  false,
		SilenceErrors: false,
	}
	flags := root.PersistentFlags()
	flags.StringVar(&flagURL, "url", "", "URL do Zabbix (host ou endpoint da API)")
	flags.StringVar(&flagUser, "user", "", "Usuário da API")
	flags.StringVar(&flagPassword, "password", "", "Senha da API")

	flagsPtr := &connFlags{url: &flagURL, user: &flagUser, password: &flagPassword}
	root.AddCommand(newConfigCmd(flagsPtr))
	root.AddCommand(newHostCmd(flagsPtr))
	root.AddCommand(newGrupoCmd(flagsPtr))
	root.AddCommand(newRelatorioCmd(flagsPtr))
	return root
}

type connFlags struct {
	url      *string
	user     *string
	password *string
}

func (f *connFlags) changed(cmd *cobra.Command, name string) bool {
	return cmd.Flags().Changed(name)
}

func readPassword() (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("informe a senha com --password")
	}
	fmt.Fprint(os.Stderr, "Senha: ")
	raw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("não foi possível ler a senha: %w", err)
	}
	if len(raw) == 0 {
		return "", fmt.Errorf("senha vazia")
	}
	return string(raw), nil
}

func loadSettings(cmd *cobra.Command, flags *connFlags) (config.File, error) {
	cfg, err := config.Load()
	if err != nil {
		return config.File{}, err
	}
	if flags.changed(cmd, "url") {
		cfg.URL = *flags.url
	}
	if flags.changed(cmd, "user") {
		cfg.Username = *flags.user
	}
	if flags.changed(cmd, "password") {
		if strings.TrimSpace(*flags.password) == "" {
			return config.File{}, fmt.Errorf("senha vazia")
		}
		cfg.Password = *flags.password
	}
	if cfg.URL == "" || cfg.Username == "" || cfg.Password == "" {
		return config.File{}, fmt.Errorf("configure a conexão com `zbx-admin config set --url ... --user ... --password`")
	}
	cfg.URL, err = config.NormalizeURL(cfg.URL)
	if err != nil {
		return config.File{}, err
	}
	if cfg.Timezone == "" {
		cfg.Timezone = config.DefaultTimezone
	}
	return cfg, nil
}

func stdout(cmd *cobra.Command) io.Writer {
	return cmd.OutOrStdout()
}
