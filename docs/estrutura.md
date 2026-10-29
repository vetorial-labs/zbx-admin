# Estrutura do código e comandos

Este guia descreve como o `zbx-admin` está organizado e como alterar um comando existente ou criar um novo. O uso da CLI para quem vai gerar relatórios está no [README](../README.md).

O módulo Go é `zbx-admin`. Tudo em `internal/` só pode ser importado por este módulo.

## Mapa das pastas

```text
cmd/zbx-admin/main.go          processo: traduz o erro em código de saída
internal/cli/                  comandos cobra, flags, texto impresso
internal/config/               ~/.zbx-admin/config.yaml
internal/zabbix/               cliente JSON-RPC (login, busca, ICMP)
internal/resolve/              escolhe um único host ou grupo
internal/report/               período, horas e quedas (sem rede e sem disco)
internal/avail/                junta monitores e eventos num Report
internal/export/               grava CSV e PDF
.github/workflows/release.yml  go test e os binários de release
```

| Pacote | Responsabilidade | Não coloca aqui |
| --- | --- | --- |
| `cli` | `Use`, `Short`, flags, `RunE`, impressão | chamada JSON-RPC, conta de horas, escrita de PDF |
| `config` | ler e gravar a conexão | prompt de senha (isso fica na CLI) |
| `zabbix` | métodos da API em cima de `Client.call` | formatação do relatório |
| `resolve` | nome exato contra vários parciais | acesso à rede |
| `report` | `[From, To)`, segundos, centésimos de hora, slug | `os`, HTTP, cobra |
| `avail` | para cada host: evento, histórico ou tendência | formato do arquivo |
| `export` | `disponibilidade_...-resumo.csv`, `-quedas.csv` e `.pdf` | decisão de qual fonte usar |

`internal/report` não importa os outros pacotes do projeto. A conta de disponibilidade pode ser testada com instantes e amostras, sem subir um Zabbix falso. Quem precisa do Zabbix chama `report.AccountEvents`, `AccountHistory` ou `AccountTrends` já com os dados em mãos.

## Do comando até o arquivo

`relatorio disponibilidade` é o caminho mais longo. Os outros comandos param antes.

1. `cmd/zbx-admin/main.go` chama `cli.Execute()`.
2. `newRoot` em `internal/cli/cli.go` monta a cobra e registra `config`, `host`, `grupo` e `relatorio`.
3. `runAvailability` em `internal/cli/report_cmd.go` lê a configuração, resolve o período, autentica e escolhe o host ou o grupo.
4. `avail.Build` pede os itens `icmpping`, os triggers e os eventos. Sem trigger, cai no histórico ou na tendência.
5. `export.Write` grava os três arquivos. `printSummary` escreve o resumo no stdout do comando.

```text
flag --host / --grupo
        |
        v
  pickHost / pickGroup          resolve.Pick  (exit 2 se ambíguo)
        |
        v
  zabbix.ICMPMonitors + Incidents
        |
        v
  report.AccountEvents          ou AccountHistory / AccountTrends
        |
        v
  export.Write                  CSV + PDF
```

A fonte de cada host fica em `HostResult.Source`: `evento`, `historico`, `tendencia` ou `sem_item`. Host `sem_item` entra na lista e fica fora da soma.

## Comandos que já existem

O registro único está em `newRoot`:

```go
root.AddCommand(newConfigCmd(flagsPtr))
root.AddCommand(newHostCmd(flagsPtr))
root.AddCommand(newGrupoCmd(flagsPtr))
root.AddCommand(newRelatorioCmd(flagsPtr))
```

| Comando | Construtor | Arquivo |
| --- | --- | --- |
| `config set`, `config show` | `newConfigCmd` | `internal/cli/config_cmd.go` |
| `host search` | `newHostCmd` | `internal/cli/search_cmd.go` |
| `grupo search` | `newGrupoCmd` | `internal/cli/search_cmd.go` |
| `relatorio disponibilidade` | `newRelatorioCmd` | `internal/cli/report_cmd.go` |

Não há subcomando `version`. A cobra imprime `zbx-admin version ...` com `--version`. O texto vem de `internal/cli/version.go` (`var Version = "dev"`). O workflow de release troca esse valor com `-X zbx-admin/internal/cli.Version=...`.

## Peças que todo comando reutiliza

`connFlags` guarda os ponteiros das flags persistentes `--url`, `--user` e `--password`, declaradas na raiz. O construtor do comando recebe `*connFlags` para enxergar o que o usuário passou naquela execução.

`loadSettings` lê `~/.zbx-admin/config.yaml` e, se a flag mudou, cobre URL, usuário ou senha. Uma URL sem caminho ganha `/api_jsonrpc.php`. Fuso vazio vira `America/Sao_Paulo`. Falta qualquer um dos três e o erro pede `zbx-admin config set`.

`connect` faz `loadSettings` e `zabbix.Login`. O token fica no `Client`, só na memória. O timeout da API é 120 segundos. Falha de login que mencione senha ou login vira `usuário ou senha rejeitados pelo Zabbix`, sem ecoar a senha.

`stdout(cmd)` é `cmd.OutOrStdout()`. Testes trocam essa saída com `root.SetOut`. Imprimir em `os.Stdout` direto foge do teste.

`pickHost` e `pickGroup` tratam um termo só de dígitos como ID. Nos outros casos buscam e chamam `resolve.Pick`. Um nome exato, sem diferenciar maiúsculas, vence vários parciais. Vários parciais sem um exato único devolvem `*resolve.AmbiguousError`.

`SilenceUsage = true` fica no começo do `RunE`, depois que a cobra já validou as flags. Erro de flag continua mostrando a ajuda. Erro de API, de busca ou de período sai só com a mensagem.

## Códigos de saída

`main` não imprime o erro de novo. A cobra já imprime, porque `SilenceErrors` está desligado.

| Situação | Código |
| --- | --- |
| comando concluiu | 0 |
| uso, configuração, API, alvo inexistente | 1 |
| busca ambígua (`*resolve.AmbiguousError`) | 2 |

Para o código 2 valer, devolva o `*resolve.AmbiguousError` ou embrulhe com `%w`. `fmt.Errorf("...: %v", err)` perde o tipo e vira saída 1. `resolve.Pick` já devolve o ponteiro certo. `pickHost` e `pickGroup` repassam esse erro sem reescrever.

Nenhum comando chama `os.Exit`.

## Alterar um comando que já existe

1. Abra o arquivo da tabela acima.
2. Mude `Short`, `Long` ou `Example` se o texto de `zbx-admin <comando> --help` precisar mudar. A ajuda é gerada desses campos.
3. Flag nova fica no comando que a usa, com variável declarada dentro do construtor, como `--periodo` em `newRelatorioCmd`. Flag de todos os comandos entra em `root.PersistentFlags()` dentro de `newRoot`.
4. Leia a flag com `cmd.Flags().Changed` só quando o valor ausente deve preservar o que já estava salvo. `--timezone` em `config set` funciona assim. Flags com default (`--periodo`, `--saida`) podem ser lidas direto.
5. A lógica que não é texto de terminal vai para o pacote da tabela. Um filtro novo de trigger mora em `internal/zabbix`. Uma regra nova de hora mora em `internal/report`.
6. Cubra a mudança com teste no mesmo pacote. `go test ./...` é o que o workflow roda.

Dois casos que já aparecem no código:

- `--host` e `--grupo` são mutuamente exclusivos e um deles é obrigatório: `MarkFlagsMutuallyExclusive` e `MarkFlagsOneRequired` em `newRelatorioCmd`.
- O preset do período aceita o nome em português e o alias em inglês (`mes-atual` / `current-month`, `mes-anterior` / `previous-month`, `personalizado` / `custom`). O `switch` está em `report.Resolve`. Um preset novo entra nesse `switch` e em `internal/report/period_test.go`.

A senha não aparece em `config show`, em log nem em mensagem de erro. `config show` imprime `configurada` ou `ausente`.

## Criar um comando novo

O encaixe mínimo é um construtor que devolve `*cobra.Command`, registrado em `newRoot`. Comando que fala com o Zabbix segue `host search`: silencia a ajuda de uso, conecta, chama o cliente e escreve com `stdout(cmd)`.

Exemplo: um `host get <termo>` que resolve um único host e imprime o ID. O arquivo pode ser `internal/cli/host_get_cmd.go`, ou o subcomando pode nascer dentro de `newHostCmd` se for pequeno. Os dois jeitos valem. Subcomando no construtor que já existe evita um `AddCommand` extra na raiz.

```go
package cli

import (
    "fmt"
    "text/tabwriter"

    "github.com/spf13/cobra"
)

func newHostGetCmd(flags *connFlags) *cobra.Command {
    return &cobra.Command{
        Use:   "get <termo>",
        Short: "Mostra um host pelo nome ou pelo ID",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            cmd.SilenceUsage = true
            api, err := connect(cmd, flags)
            if err != nil {
                return err
            }
            host, err := pickHost(cmd.Context(), api, args[0])
            if err != nil {
                return err
            }
            w := tabwriter.NewWriter(stdout(cmd), 0, 0, 2, ' ', 0)
            fmt.Fprintln(w, "ID\tSTATUS\tNOME TÉCNICO\tNOME")
            fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", host.ID, host.StatusLabel(), host.Technical, host.Visible)
            return w.Flush()
        },
    }
}
```

Para pendurá-lo em `host`, no fim de `newHostCmd`:

```go
cmd.AddCommand(search, newHostGetCmd(flags))
```

A invocação fica `zbx-admin host get BLANKEN`. Busca ambígua continua saindo com código 2, porque `pickHost` devolve `*resolve.AmbiguousError`.

Um comando de primeiro nível, irmão de `host` e `relatorio`, ganha o próprio construtor e uma linha em `newRoot`:

```go
root.AddCommand(newMeuCmd(flagsPtr))
```

Passe `flagsPtr`. Sem ele, `--url`, `--user` e `--password` daquela execução não chegam no comando.

### Quando o comando precisa de uma chamada nova na API

`Client.call` não é exportado. O método novo fica no pacote `zabbix`, no mesmo estilo de `Version`:

```go
func (c *Client) Algo(ctx context.Context, id string) (MeuTipo, error) {
    var out MeuTipo
    params := map[string]any{"hostids": []string{id}}
    if err := c.call(ctx, "algum.metodo", params, &out); err != nil {
        return MeuTipo{}, err
    }
    return out, nil
}
```

O comando só vê `Algo`. JSON de resposta da API fica em tipos não exportados do pacote (`hostDTO` é o modelo). A busca por nome é substring e não liga `searchWildcardsEnabled`. Com esse parâmetro ligado, o Zabbix exige `*` no termo e a busca atual passa a voltar vazia.

Teste o método com `httptest`, como `internal/zabbix/icmp_test.go` e `internal/cli/cli_test.go`. O teste da CLI monta a raiz, aponta a saída para um buffer e passa os argumentos:

```go
func TestHostGet(t *testing.T) {
    // srv := httptest.NewServer(...) respondendo user.login e host.get
    t.Setenv("HOME", t.TempDir())
    var out bytes.Buffer
    root := newRoot()
    root.SetOut(&out)
    root.SetErr(&out)
    root.SetArgs([]string{
        "--url", srv.URL,
        "--user", "thiago",
        "--password", "segredo",
        "host", "get", "10",
    })
    if err := root.Execute(); err != nil {
        t.Fatal(err)
    }
}
```

`TestReportCommandWritesFiles` já mostra um servidor falso que despacha por `req.Method` (`user.login`, `host.get`, `item.get`, `event.get`). Copie esse desenho quando o comando autentica.

`newRoot` não é exportado. O teste fica em `package cli`, não em `package cli_test`.

### Quando o comando gera outro relatório

Separe as três camadas no mesmo desenho da disponibilidade:

- regra pura em `internal/report` (ou num arquivo novo desse pacote), com teste de tabela;
- busca dos dados em `internal/zabbix`;
- orquestração num pacote pequeno, no estilo de `avail.Build`;
- arquivo em `internal/export`, recebendo um `report.Report` já pronto.

`export.Write` hoje grava sempre os três arquivos da disponibilidade ICMP. Outro relatório precisa da própria função de escrita, para não misturar colunas. Horas exibidas passam por `report.SplitHours` e `report.FormatCenti` quando UP e DOWN precisam fechar com o período na soma das linhas.

O PDF usa DejaVu embutido em `internal/export/fonts/` para acentos. `fonts/LICENSE` acompanha as fontes. `SetCompression(false)` deixa o texto em UTF-16BE dentro do arquivo. Um teste que procura uma frase no PDF compara esses bytes, como `bytesContain` em `csv_test.go`.

## Onde mexer em cada tipo de mudança

| Pedido | Onde |
| --- | --- |
| Texto de ajuda, flag, coluna no terminal | `internal/cli` |
| Conexão gravada, permissão do arquivo, normalização da URL | `internal/config` |
| Outro método JSON-RPC, outro critério de item ou trigger | `internal/zabbix` |
| Desempate de nome ou ID | `internal/resolve` |
| Preset de período, arredondamento, slug do arquivo | `internal/report` |
| Ordem das fontes ICMP (evento, histórico, tendência) | `internal/avail` |
| Coluna de CSV ou página do PDF | `internal/export` |
| Binário publicado, Go do CI | `.github/workflows/release.yml` |

Mensagens vistas pelo usuário ficam em português, no mesmo tom das que já existem (`nenhum host encontrado para %q`, `fuso horário inválido`).

## Verificação

```sh
go test ./...
go build -o zbx-admin ./cmd/zbx-admin
./zbx-admin <comando> --help
```

O binário `./zbx-admin` e os relatórios `disponibilidade_*` estão no `.gitignore`. Configuração e senha ficam só em `~/.zbx-admin/`.
