# gommit

Widget de desktop em Go que mostra o gráfico de contribuições do GitHub — aquele
quadriculado verde do perfil — numa janela própria, sempre visível na área de
trabalho.

A ideia é simples: motivação visual pra codar todo dia. O quadrado de hoje
começa apagado; ele só acende quando você commita.

> **Status:** em construção. Este repositório está no começo — o que existe aqui
> por enquanto é a documentação de intenção e as decisões de arquitetura já
> tomadas.

## Por que este projeto existe

Antes de ser um widget, este é um **exercício de fundamentos em Go**. O objetivo
não é chegar no resultado pelo caminho mais curto, e sim testar decisões de
arquitetura, errar, refatorar e ganhar repertório no processo.

Ele também serve de terreno de treino para um projeto maior nos planos: um app
centralizador de notificações (Go + SQLite + Wails). O padrão central —
*poller rodando em background + evento disparado pro frontend* — é exatamente o
mesmo nos dois.

## Como funciona

```
  PAT (env var)
       │
       ▼
  ┌──────────────┐   polling    ┌─────────────────────┐
  │  cliente     │ ───────────► │  GitHub GraphQL API │
  │  GraphQL     │ ◄─────────── │  contributions      │
  └──────┬───────┘   calendário └─────────────────────┘
         │
         │ total subiu desde a última checagem?
         ▼
  ┌──────────────┐   evento     ┌─────────────────────┐
  │  poller      │ ───────────► │  janela Wails       │
  │  (goroutine) │              │  (HTML/CSS/JS)      │
  └──────────────┘              └─────────────────────┘
```

1. Autentica na API do GitHub com um Personal Access Token.
2. Busca o calendário de contribuições
   (`contributionsCollection.contributionCalendar`).
3. Fica rodando em background, consultando de tempos em tempos.
4. Quando detecta que o total subiu, atualiza a janela na hora, com algum
   feedback visual.

## Decisões de arquitetura

| Decisão | Escolha | Motivo |
| --- | --- | --- |
| Fonte dos dados | GraphQL oficial (`api.github.com/graphql`) | Não existe endpoint REST pro calendário de contribuições — só o GraphQL expõe `contributionsCollection`. |
| Atualização | Polling curto (ex.: 1 min) | O GitHub **não** dispara webhook por contribuição. Com token autenticado o limite é 5000 req/h, então 1 req/min sobra folgado. |
| GUI | [Wails](https://wails.io/) (Go no backend + HTML/CSS/JS no frontend) | Permite reaproveitar protótipos visuais quase direto, sem reescrever layout em Go. Descartados: Fyne e Walk. |
| Formato da janela | Frameless, sempre visível | É um widget "encostado" na área de trabalho, não um app com menu e abas. |
| Token | Variável de ambiente / arquivo local fora do versionamento | Privacy by default. O token **nunca** entra no repositório. |

### Sobre o protótipo inicial

A ideia foi validada antes com Rainmeter + serviços de terceiros
(`ghchart` / `wsrv`) só pra ver se o conceito se sustentava. O app final não
depende de nenhum deles: fala direto com a API oficial e desenha a própria
janela.

## Configuração

### Requisitos

- [Go](https://go.dev/dl/) 1.25+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)
- Um Personal Access Token do GitHub com escopo `read:user`

### Token

Gere o token em **Settings → Developer settings → Personal access tokens**. O
escopo `read:user` é suficiente — não precisa de acesso a repositórios.

Exponha o token como variável de ambiente:

```powershell
# PowerShell (sessão atual)
$env:GITHUB_TOKEN = "seu_token_aqui"

# PowerShell (permanente, usuário atual)
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "seu_token_aqui", "User")
```

```bash
# bash / zsh
export GITHUB_TOKEN="seu_token_aqui"
```

> ⚠️ **Nunca commite o token.** O `.gitignore` já cobre `.env` e arquivos de
> credencial locais, mas a regra vale independente disso.

### A query

O coração do app é esta consulta:

```graphql
query($login: String!) {
  user(login: $login) {
    contributionsCollection {
      contributionCalendar {
        totalContributions
        weeks {
          contributionDays {
            date
            contributionCount
            contributionLevel
          }
        }
      }
    }
  }
}
```

`totalContributions` é o número comparado a cada ciclo de polling para detectar
uma contribuição nova. `contributionLevel` é o que define a intensidade do verde
de cada quadradinho.

## Roadmap

- [ ] Cliente GraphQL: autenticação e busca do calendário
- [ ] Modelagem dos tipos de domínio (calendário, semana, dia)
- [ ] Poller em background com intervalo configurável
- [ ] Detecção de mudança no total de contribuições
- [ ] Janela Wails frameless e always-on-top
- [ ] Renderização do grid de contribuições
- [ ] Feedback visual quando uma contribuição nova é detectada
- [ ] Tratamento de erro e rate limit
- [ ] Build e distribuição

## Design visual

Prototipado à parte no Claude Design. Layout e estética não são tratados nesta
documentação.
