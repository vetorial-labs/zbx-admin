// Package config lê e grava ~/.zbx-admin/config.yaml.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	DirName  = ".zbx-admin"
	FileName = "config.yaml"
	// DefaultTimezone é o fuso do Zabbix de referência (America/Sao_Paulo).
	DefaultTimezone = "America/Sao_Paulo"
)

// File é a configuração persistida. A senha fica só neste arquivo, modo 0600.
type File struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Timezone string `yaml:"timezone"`
}

// Dir devolve $HOME/.zbx-admin.
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar o diretório home: %w", err)
	}
	return filepath.Join(home, DirName), nil
}

// Path devolve o caminho do config.yaml.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

// Load lê a configuração. Arquivo ausente devolve uma struct vazia.
func Load() (File, error) {
	path, err := Path()
	if err != nil {
		return File{}, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return File{}, nil
		}
		return File{}, fmt.Errorf("não foi possível ler %s: %w", path, err)
	}
	var cfg File
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return File{}, fmt.Errorf("arquivo de configuração inválido (%s): %w", path, err)
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Timezone = strings.TrimSpace(cfg.Timezone)
	return cfg, nil
}

// Save grava a configuração com diretório 0700 e arquivo 0600.
func Save(cfg File) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("não foi possível criar %s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("não foi possível ajustar a permissão de %s: %w", dir, err)
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Timezone = strings.TrimSpace(cfg.Timezone)
	if cfg.Timezone == "" {
		cfg.Timezone = DefaultTimezone
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("não foi possível serializar a configuração: %w", err)
	}
	path := filepath.Join(dir, FileName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("não foi possível gravar %s: %w", path, err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("não foi possível ajustar a permissão de %s: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("não foi possível gravar %s: %w", path, err)
	}
	return nil
}

// NormalizeURL aceita um host ou a URL completa da API JSON-RPC.
func NormalizeURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("informe a URL do Zabbix")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("URL inválida: %s", raw)
	}
	if u.Path == "" || u.Path == "/" {
		u.Path = "/api_jsonrpc.php"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
