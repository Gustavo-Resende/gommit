package main

import (
	"context"
	"sync"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/poller"
	"github.com/Gustavo-Resende/gommit/internal/ui"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// eventCalendar é o nome do evento que o frontend escuta. Constante porque o
// nome aparece nos dois lados da fronteira: errar a string aqui não dá erro de
// compilação nenhum, só uma janela que nunca atualiza.
const eventCalendar = "calendar:update"

// App é a ponte entre o canal do poller e o webview.
//
// Ele não tem regra de negócio: só consome updates, traduz para snapshot e
// avisa o frontend. Toda a lógica continua nos pacotes internal/, que seguem
// sem saber que o Wails existe.
type App struct {
	// poller é nil quando o app subiu sem credencial válida. Nesse caso a
	// janela abre mostrando o erro em vez de não abrir — ver comentário no
	// main.go.
	poller *poller.Poller

	// login vem da config e nunca muda, então fica fora do Snapshot: mandar a
	// mesma string a cada ciclo seria repetir dado imutável num evento que
	// existe para carregar o que mudou. O frontend pega uma vez, no load.
	login string

	// ctx é o contexto do Wails, entregue no startup. Guardá-lo é o padrão da
	// v2: as funções do runtime (EventsEmit, Quit) precisam dele para saber a
	// qual aplicação estão falando.
	ctx context.Context

	// mu protege last, que é o primeiro estado de fato compartilhado do
	// projeto: a goroutine do poller escreve, e as chamadas vindas do
	// JavaScript leem, cada uma na sua goroutine.
	//
	// Note que o lastTotal/primed lá dentro do poller continuam sem mutex, e
	// isso segue correto — só a goroutine do Start toca neles. Sincronização é
	// para estado compartilhado, não para todo estado mutável.
	mu   sync.RWMutex
	last ui.Snapshot
}

// NewApp devolve o App. Um poller nil é válido e significa "subiu com erro":
// a janela abre, mostra a mensagem e não consulta nada.
func NewApp(p *poller.Poller, login string, falhaInicial error) *App {
	app := &App{poller: p, login: login}
	if falhaInicial != nil {
		app.last = app.last.WithError(falhaInicial)
	}
	return app
}

// startup roda antes de o frontend carregar. É o gancho que o Wails chama para
// entregar o contexto da aplicação.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	if a.poller == nil {
		return
	}

	// A goroutine é necessária: o startup precisa retornar para o Wails seguir
	// abrindo a janela. Bloquear aqui no range travaria o app antes de aparecer
	// qualquer coisa na tela.
	go a.consume(ctx)
}

// consume drena o canal do poller até ele fechar (o que acontece quando o ctx
// da aplicação é cancelado, no fechamento da janela).
func (a *App) consume(ctx context.Context) {
	for update := range a.poller.Start(ctx) {
		a.mu.Lock()
		if update.Err != nil {
			// Anexa o erro ao último estado bom em vez de substituí-lo: numa
			// queda de rede o grid continua desenhado e o erro aparece de
			// canto. Para uma janela que fica aberta o dia inteiro, apagar
			// tudo por causa de dois segundos sem Wi-Fi seria péssimo.
			a.last = a.last.WithError(update.Err)
		} else {
			a.last = ui.FromCalendar(update.Calendar, update.Changed, time.Now())
		}
		// Cópia local para soltar o lock antes de emitir. Nunca chame código de
		// fora segurando um mutex — o EventsEmit atravessa para o webview, e o
		// que acontece do outro lado não é problema que se resolva sob lock.
		//
		// A cópia compartilha o array por trás dos slices com a.last, e isso é
		// seguro porque ninguém muta um Snapshot depois de montado: cada ciclo
		// constrói slices novos, e o WithError só mexe em campos escalares.
		snap := a.last
		a.mu.Unlock()

		wruntime.EventsEmit(ctx, eventCalendar, snap)
	}
}

// Snapshot devolve o estado atual. É chamada pelo frontend ao carregar.
//
// Ela existe por causa de uma corrida real: o startup roda antes de o webview
// existir, então o primeiro EventsEmit sai para uma plateia vazia. Sem esta
// função, a janela abriria em branco e ficaria assim até o tick seguinte — um
// minuto inteiro de widget vazio.
func (a *App) Snapshot() ui.Snapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.last
}

// Login devolve o usuário do GitHub que está sendo acompanhado.
//
// É chamada uma vez, no load. Fica fora do Snapshot de propósito: o snapshot
// carrega o que muda a cada ciclo, e o login não muda nunca.
func (a *App) Login() string {
	return a.login
}

// Minimize manda a janela para a barra de tarefas.
//
// Existe pelo mesmo motivo que o Quit: sem barra de título, os botões que o
// sistema daria de graça viram responsabilidade do frontend.
func (a *App) Minimize() {
	wruntime.WindowMinimise(a.ctx)
}

// Quit fecha a aplicação.
//
// Obrigatória, não conveniência: a janela é frameless e always-on-top, ou seja,
// não tem o X da barra de título e fica por cima de tudo. Sem este botão a
// única saída seria o Gerenciador de Tarefas.
func (a *App) Quit() {
	wruntime.Quit(a.ctx)
}
