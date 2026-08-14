# CLAUDE.md — GitHub Contributions Widget

## Intuito do projeto

Widget de desktop, feito em Go, que mostra o gráfico de contribuições do GitHub
(estilo o quadriculado verde do perfil) numa janela própria sempre visível —
o objetivo é motivação visual pra codar todo dia.

**Este projeto é, antes de tudo, um exercício de fundamentos em Go.** Não é
pra ser resolvido do jeito mais rápido possível — é pra ir testando decisões
de arquitetura, errando, refatorando, e ganhando repertório. Não adianta me
dar (Claude) a solução pronta; o valor está em construir, mesmo que devagar.

Este projeto também serve de terreno de treino pro app centralizador de
notificações (Go + SQLite + Wails) que está nos planos — o padrão de
"poller em background + evento pro frontend" se repete lá.

## O que o app faz

- Autentica na API oficial do GitHub (GraphQL) com um Personal Access Token.
- Busca o calendário de contribuições do usuário (`contributionsCollection.contributionCalendar`).
- Fica rodando em background, consultando de tempos em tempos (polling —
  o GitHub não tem webhook de "nova contribuição").
- Quando detecta que o total subiu desde a última checagem, atualiza a
  janela na hora (algum feedback visual/notificação).
- Roda como janela própria (não depende de Rainmeter nem de terceiros tipo
  ghchart/wsrv — essas ferramentas foram usadas só como protótipo inicial
  pra validar a ideia).

## Decisões já tomadas

- **API:** GraphQL oficial do GitHub (`api.github.com/graphql`), campo
  `contributionsCollection`. Não existe endpoint REST pra isso — só o
  GraphQL expõe o calendário de contribuições. Token com escopo `read:user`
  basta.
- **Sem webhook de contribuição:** GitHub não dispara evento por
  contribuição. A saída é polling curto (ex.: a cada 1 min), o que ainda
  fica bem abaixo do limite de 5000 requisições/hora com token autenticado.
- **GUI:** Wails (Go no backend + HTML/CSS/JS no frontend), não Fyne nem
  Walk. Motivo: o frontend em HTML permite reaproveitar protótipos visuais
  feitos no Claude Design quase direto, sem reescrever em Go.
- **Janela estilo widget:** frameless, sempre visível, não é um app com
  menu/abas — é pra ficar "encostado" na área de trabalho.
- **Nunca commitar o token.** Fica em variável de ambiente ou arquivo local
  fora do versionamento (mesmo princípio de privacy-by-default que uso nos
  outros projetos).

## Design visual

Fica pra depois, prototipado separadamente no Claude Design. Este documento
não trata de layout/estética.S