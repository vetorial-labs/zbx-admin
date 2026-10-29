# zbx-admin

CLI em Go para consultar um Zabbix e exportar a disponibilidade ICMP dos hosts. Para um host ou um grupo, o programa calcula o tempo UP e DOWN no período pedido e grava o resultado em CSV e PDF.

A disponibilidade vem dos eventos do trigger de indisponibilidade ICMP (`unavailable by ICMP`), o mesmo estado que o Zabbix mostra como problema. Cada queda entra com início, fim e duração. Host sem item `icmpping` aparece no relatório e fica fora da soma.

## Requisitos

- Zabbix 6.4 com a API JSON-RPC
- Go 1.27 ou superior, para compilar a partir do código

## Instalação

Os binários de cada versão estão em [Releases](https://github.com/vetorial-labs/zbx-admin/releases).

- `zbx-admin-linux-amd64` para Linux x86_64
- `zbx-admin-macos-arm64` para macOS Apple Silicon

```sh
curl -fsSL -o zbx-admin https://github.com/vetorial-labs/zbx-admin/releases/download/v0.1.0/zbx-admin-macos-arm64
chmod +x zbx-admin
./zbx-admin --version
```

No Linux, troque o nome do arquivo por `zbx-admin-linux-amd64`. Confira o `SHA256SUMS` publicado junto com a versão.

## Build

```sh
git clone https://github.com/vetorial-labs/zbx-admin.git
cd zbx-admin
go test ./...
go build -o zbx-admin ./cmd/zbx-admin
```

`zbx-admin --version` responde `zbx-admin version dev` num build local. As versões publicadas recebem o nome da tag.

O workflow [Build e release](.github/workflows/release.yml) testa o código em cada push na `main`. Uma tag `v*` também compila os dois binários e abre o GitHub Release:

```sh
git tag v0.1.1
git push origin v0.1.1
```

## Configuração

A conexão fica em `~/.zbx-admin/config.yaml`. O diretório é criado com permissão `0700` e o arquivo com `0600`.

```sh
zbx-admin config set \
  --url https://zabbix.exemplo.net \
  --user operador \
  --password 'senha'
```

A URL pode ser só o host. O programa acrescenta `/api_jsonrpc.php`. Sem `--password`, a senha é pedida no terminal quando ainda não existe uma salva.

```sh
zbx-admin config show
```

`config show` lista URL, usuário e fuso, e omite a senha. O fuso padrão é `America/Sao_Paulo`. Para mudar:

```sh
zbx-admin config set --timezone America/Sao_Paulo
```

`--url`, `--user` e `--password` também podem ser passados em qualquer comando. Nesse caso valem só para aquela execução.

## Uso

Busca de hosts, pelo nome visível ou pelo nome técnico:

```sh
zbx-admin host search BLANKEN
```

Busca de grupos. A coluna HOSTS é a quantidade de hosts monitorados:

```sh
zbx-admin grupo search CLIENTES
```

Relatório de um host. O período padrão é o mês atual, do dia 1 às 00:00 até o momento da geração:

```sh
zbx-admin relatorio disponibilidade --host BLANKEN
```

Mês anterior fechado:

```sh
zbx-admin relatorio disponibilidade --host BLANKEN --periodo mes-anterior
```

Grupo inteiro, um único par de CSV e um PDF:

```sh
zbx-admin relatorio disponibilidade --grupo CLIENTES --periodo mes-anterior --saida ./relatorios
```

Período personalizado. Uma data sozinha em `--ate` inclui o dia inteiro. Data e hora usam o formato `2006-01-02T15:04`:

```sh
zbx-admin relatorio disponibilidade \
  --host 10775 \
  --periodo personalizado \
  --de 2026-09-01 \
  --ate 2026-09-30 \
  --saida ./relatorios
```

`--host` e `--grupo` aceitam o nome ou o ID numérico. Se a busca encontrar mais de um resultado e nenhum nome exato, o comando lista os candidatos e termina com código 2. Host ou grupo inexistente termina com código 1.

Se o fim do período ainda não aconteceu, a conta vai só até o instante da geração.

## Arquivos gerados

O nome segue `disponibilidade_<alvo>_<inicio>_<fim>`.

- `...-resumo.csv` traz uma linha por host: horas UP, horas DOWN, tempo contabilizado, disponibilidade e quantidade de quedas. A última linha é o TOTAL.
- `...-quedas.csv` traz cada período DOWN, com início, fim, duração, se ainda está em andamento e a resolução (`segundos` ou `hora`).
- `....pdf` repete o resumo, o detalhe das quedas e o tempo contabilizado.

As horas usam duas casas decimais. Em cada host, UP + DOWN é o tempo contabilizado. No grupo, o total é a soma das horas dos hosts monitorados por ICMP.

Quando o host não tem o trigger de indisponibilidade, o programa usa o histórico do item `icmpping` se o período ainda estiver dentro da retenção. Fora dela, usa a tendência horária. A coluna `fonte` indica `evento`, `historico`, `tendencia` ou `sem_item`.

## Código

A organização dos pacotes e o passo a passo para alterar ou criar um comando estão em [docs/estrutura.md](docs/estrutura.md).
