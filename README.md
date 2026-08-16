# gommit

Widget de desktop em Go que mostra o gráfico de contribuições do GitHub — aquele
quadriculado verde do perfil — numa janela própria, sempre visível na área de
trabalho.

A ideia é simples: motivação visual pra codar todo dia. O quadrado de hoje
começa apagado; ele só acende quando você commita.

> **Status:** funcionando. A janela abre, desenha o calendário e pulsa o quadrado
> de hoje quando o total sobe. O que falta agora é acabamento — ícone próprio,
> lembrar a posição da janela, distribuição.

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

## Estrutura

| Pacote | Responsabilidade |
| --- | --- |
| `internal/config` | Resolve as credenciais. Lê o `.env` como conveniência e o ambiente como fonte de verdade. |
| `internal/github` | Fala GraphQL com a API e **traduz** a resposta para os tipos de domínio. |
| `internal/contributions` | O domínio: `Calendar`, `Week`, `Day`, `Level`. Só tipos e regras que dependem deles mesmos. |
| `internal/poller` | Goroutine de background que consulta em intervalos e emite `Update` num canal. |
| `internal/render` | Desenha o calendário como texto para o terminal. |
| `internal/ui` | Traduz o domínio para o `Snapshot` JSON que o frontend consome. |
| `cmd/probe` | Sonda de terminal: liga tudo e imprime. Serve pra depurar o backend sem GUI. |
| `main.go` + `app.go` | A janela: única parte do projeto que conhece o Wails. |
| `frontend/dist` | HTML/CSS/JS do widget. |

A regra que organiza tudo isso: **nenhum pacote `internal/` sabe que existe
terminal ou janela.** Quem decide o destino do dado é quem está na ponta.

A prova de que a regra está sendo cumprida é a existência do `cmd/probe`: a
sonda de terminal e a janela importam exatamente os mesmos pacotes, e nenhum
deles precisou de uma linha diferente para servir aos dois.

Os pacotes `github` e `ui` são as duas fronteiras, simétricas: um traduz o que
vem de fora **para** o domínio, o outro traduz o domínio **para** o que vai para
fora. No meio, `contributions` não sabe que existe JSON, HTTP ou janela.

Duas consequências práticas dessa regra, que valem como referência:

- `poller.Fetcher` é uma interface declarada **no consumidor**, não no pacote
  `github` que a implementa. O poller não importa o `github`, e nos testes um
  fetcher falso substitui a rede inteira.
- `Start()` devolve um `<-chan Update` (canal só de leitura) fechado no
  cancelamento do `context`. Quem consome só faz `for range` — não precisa saber
  de goroutine, ticker nem sinal de parada.

## Decisões de arquitetura

| Decisão | Escolha | Motivo |
| --- | --- | --- |
| Fonte dos dados | GraphQL oficial (`api.github.com/graphql`) | Não existe endpoint REST pro calendário de contribuições — só o GraphQL expõe `contributionsCollection`. |
| Atualização | Polling curto (ex.: 1 min) | O GitHub **não** dispara webhook por contribuição. Com token autenticado o limite é 5000 req/h, então 1 req/min sobra folgado. |
| GUI | [Wails](https://wails.io/) **v2** (Go no backend + HTML/CSS/JS no frontend) | Permite reaproveitar protótipos visuais quase direto, sem reescrever layout em Go. Descartados: Fyne e Walk. O v3 ficou de fora por estar em beta — a API de janela única do v2 já cobre um widget. |
| Frontend | HTML/CSS/JS puro, sem npm | O Wails injeta o runtime em `window.runtime`, então não há import a resolver. Mantém a promessa de zero dependências do projeto. |
| Formato da janela | Frameless, sempre visível | É um widget "encostado" na área de trabalho, não um app com menu e abas. |
| Comunicação Go → tela | Evento (`EventsEmit`) + cache lido por método ligado | O evento cobre as atualizações; o cache cobre a primeira, porque o poller começa antes de o webview existir. Só o evento deixaria a janela em branco por um ciclo. |
| Token | Variável de ambiente / arquivo local fora do versionamento | Privacy by default. O token **nunca** entra no repositório. |

### Sobre o protótipo inicial

A ideia foi validada antes com Rainmeter + serviços de terceiros
(`ghchart` / `wsrv`) só pra ver se o conceito se sustentava. O app final não
depende de nenhum deles: fala direto com a API oficial e desenha a própria
janela.

## Configuração

### Requisitos

- [Go](https://go.dev/dl/) 1.25+
- Um Personal Access Token do GitHub com escopo `read:user`
- **Só para a janela:** [Wails CLI](https://wails.io/docs/gettingstarted/installation)
  e, no Windows, o [WebView2 Runtime](https://developer.microsoft.com/microsoft-edge/webview2/)
  (já vem no Windows 11). A sonda de terminal não precisa de nenhum dos dois.

### Variáveis

O app precisa de **duas** variáveis:

| Variável | O que é |
| --- | --- |
| `GITHUB_TOKEN` | Personal Access Token com escopo `read:user`. |
| `GITHUB_LOGIN` | Seu usuário do GitHub — é de quem o calendário será buscado. |

Gere o token em **Settings → Developer settings → Personal access tokens**. O
escopo `read:user` é suficiente — não precisa de acesso a repositórios.

**Jeito mais simples:** copie o [`.env.example`](.env.example) para `.env` na raiz
do projeto e preencha. O `.env` está no `.gitignore`.

```
GITHUB_TOKEN=seu_personal_access_token
GITHUB_LOGIN=seu_login_do_github
```

O app procura o `.env` em três lugares, nesta ordem — o primeiro que existir
vence:

1. O diretório atual (é o caso do `go run` e do `wails dev`).
2. O diretório do executável (o `.env` viaja junto do `.exe`).
3. `%AppData%\gommit\.env` — o lugar para deixar a credencial de uma vez e não
   precisar copiar nada a cada build.

A ordem importa porque um widget é aberto por atalho, e aí o "diretório atual" é
qualquer coisa. Sem os itens 2 e 3, o `.exe` funcionaria no terminal e falharia
no clique.

Ou exporte no ambiente, se preferir:

```powershell
# PowerShell (sessão atual)
$env:GITHUB_TOKEN = "seu_token_aqui"
$env:GITHUB_LOGIN = "seu_login"

# PowerShell (permanente, usuário atual)
[Environment]::SetEnvironmentVariable("GITHUB_TOKEN", "seu_token_aqui", "User")
```

```bash
# bash / zsh
export GITHUB_TOKEN="seu_token_aqui"
export GITHUB_LOGIN="seu_login"
```

> **O ambiente vence o arquivo.** Se a variável já estiver definida na sessão, o
> `.env` é ignorado para ela — é a regra do 12-factor, e serve pra sobrescrever
> um valor numa execução só. A armadilha é a mesma coisa de cabeça pra baixo: se
> sobrou um `$env:GITHUB_TOKEN` velho na sessão, mexer no `.env` não muda nada.

> ⚠️ **Nunca commite o token.** O `.gitignore` já cobre `.env` e arquivos de
> credencial locais, mas a regra vale independente disso.

## Como rodar

```bash
go test ./...          # a suíte inteira
go run ./cmd/probe     # sonda de terminal: imprime o grid e fica escutando
```

A sonda desenha o calendário na primeira consulta e depois só redesenha quando
algo muda — nos ciclos normais imprime uma linha de sinal de vida. Ctrl+C encerra
limpo, sem cortar uma requisição no meio.

Ela usa cor de 24 bits, que o Windows Terminal entende. Em console antigo, ou
quando você redireciona a saída pra arquivo, defina `NO_COLOR` (qualquer valor) e
a intensidade vira densidade de caractere:

```powershell
$env:NO_COLOR = "1"; go run ./cmd/probe
```

E a janela:

```bash
wails dev              # janela com recarga automática — é o modo do dia a dia
wails build            # gera build/bin/gommit.exe
```

> ⚠️ **`go run .` não abre a janela** — e o pior é que ele não reclama: sai com
> sucesso, sem imprimir nada. O Wails v2 esconde a implementação de verdade atrás
> de build tags, e sem elas o `wails.Run` vira um stub que retorna na hora.
>
> Se precisar rodar sem a CLI (num depurador, por exemplo), as tags são
> obrigatórias:
>
> ```bash
> go run -tags desktop,production .
> ```
>
> A sonda em `cmd/probe` não tem essa pegadinha: ela é Go puro, e `go run
> ./cmd/probe` funciona direto.

O frontend fica em [`frontend/dist/`](frontend/dist/) e é HTML/CSS/JS puro — **sem
npm, sem bundler, sem `node_modules`**. O Wails injeta o runtime em
`window.runtime` e os métodos ligados em `window.go.main.App`, então não há nada
a resolver em tempo de build. Por isso `wails.json` tem `frontend:install` e
`frontend:build` vazios, e por isso `frontend/dist/` **não** está no
`.gitignore`: ali é código-fonte, não artefato.

## A janela

Sem barra de título, tudo que o sistema daria de graça vira responsabilidade
nossa:

| Elemento | O que faz |
| --- | --- |
| Corpo da janela | Alça de arrasto (`--wails-draggable: drag`). Botões e o painel de ajustes precisam dizer `no-drag`, senão o clique vira arrasto. |
| Passar o mouse num quadrado | Mostra a data e a contagem daquele dia. |
| ⚙ | Abre os ajustes: opacidade de 25% a 100%, guardada no `localStorage`. |
| − | Minimiza (`App.Minimize`). A janela mantém entrada na barra de tarefas, então volta com um clique. |
| × | Fecha (`App.Quit`). **Obrigatório**: always-on-top sem barra de título só morreria pelo Gerenciador de Tarefas. |

A opacidade é o alpha do fundo da janela, não um filtro sobre tudo: o texto e os
quadradinhos continuam sólidos enquanto só o fundo fica vazado. Isso exige três
coisas ligadas ao mesmo tempo, e faltando qualquer uma o resultado é um
retângulo preto:

1. `WebviewIsTransparent: true` — o webview passa a respeitar alpha 0.
2. `WindowIsTranslucent: true` — a janela por baixo dele deixa passar.
3. `BackgroundColour` com `A: 0` — senão a cor opaca da janela tapa tudo.

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

- [x] Cliente GraphQL: autenticação e busca do calendário
- [x] Modelagem dos tipos de domínio (calendário, semana, dia)
- [x] Leitura de credenciais via `.env` + ambiente
- [x] Poller em background com intervalo configurável
- [x] Detecção de mudança no total de contribuições
- [x] Renderização do grid no terminal (sonda de depuração)
- [x] Tratamento de erro por ciclo — uma falha de rede não derruba o poller
- [x] Janela Wails frameless e always-on-top
- [x] Grid renderizado em HTML/CSS
- [x] Feedback visual quando uma contribuição nova é detectada
- [x] Resolver o `.env` fora do diretório atual (pra rodar por atalho)
- [x] Tooltip com data e contagem ao passar o mouse
- [x] Opacidade ajustável e botão de minimizar
- [ ] Ícone próprio no lugar do padrão do Wails
- [ ] Lembrar a posição da janela entre execuções
- [ ] Tratamento de rate limit
- [ ] Distribuição (instalador / início automático com o Windows)

## Design visual

Prototipado à parte no Claude Design. Layout e estética não são tratados nesta
documentação.
