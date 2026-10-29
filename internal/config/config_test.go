package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveAndLoadRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg := File{
		URL:      "https://zabbix.example/api_jsonrpc.php",
		Username: "thiago",
		Password: "segredo",
		Timezone: "America/Sao_Paulo",
	}
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}

	dir := filepath.Join(home, DirName)
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("diretório %o, esperado 0700", info.Mode().Perm())
	}
	path := filepath.Join(dir, FileName)
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("arquivo %o, esperado 0600", info.Mode().Perm())
	}

	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Fatalf("lido %+v, gravado %+v", got, cfg)
	}
}

func TestLoadMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got != (File{}) {
		t.Fatalf("esperado vazio, veio %+v", got)
	}
}

func TestNormalizeURL(t *testing.T) {
	got, err := NormalizeURL("zabbix.vetorial.net")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://zabbix.vetorial.net/api_jsonrpc.php" {
		t.Fatalf("url %s", got)
	}
	got, err = NormalizeURL("https://zabbix.vetorial.net/api_jsonrpc.php")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://zabbix.vetorial.net/api_jsonrpc.php" {
		t.Fatalf("url cheia %s", got)
	}
	if _, err := NormalizeURL(""); err == nil {
		t.Fatal("URL vazia deveria falhar")
	}
}
