// Package poller consulta o calendário de tempos em tempos e avisa quando o
// total de contribuições muda.
//
// Este é o padrão que se repete no app de notificações: uma goroutine em
// background produzindo eventos num canal, e quem consome (terminal hoje,
// janela depois) sem saber de onde vieram.
package poller

import (
	"context"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/contributions"
)

// Fetcher é tudo que o poller precisa saber sobre a origem dos dados.
//
// A interface é declarada aqui, no consumidor, e não no pacote github que a
// implementa — é assim que se faz em Go. O ganho é concreto: o poller não
// importa o pacote github, e nos testes você passa um Fetcher falso que
// devolve um calendário fixo, sem tocar na rede (ver poller_test.go).
type Fetcher interface {
	FetchCalendar(ctx context.Context, login string) (contributions.Calendar, error)
}

// Update é o que sai do canal a cada ciclo.
type Update struct {
	Calendar contributions.Calendar

	// Changed diz se o total subiu em relação ao ciclo anterior. É o gatilho
	// do feedback visual.
	Changed bool

	// Err carrega a falha do ciclo, quando houve. A alternativa seria matar o
	// poller no primeiro erro — péssimo para um widget que fica aberto o dia
	// todo, já que uma queda de Wi-Fi de dois segundos encerraria o app.
	// Aqui o ciclo falha, avisa, e o próximo tenta de novo.
	Err error
}

// Poller guarda o estado entre os ciclos.
type Poller struct {
	fetcher  Fetcher
	login    string
	interval time.Duration

	// lastTotal é o total visto no ciclo anterior; primed indica que já houve
	// um ciclo com o que comparar. Só a goroutine do Start mexe nos dois, e
	// por isso não precisa de mutex — a regra vale enquanto continuar assim.
	lastTotal int
	primed    bool
}

// New devolve um Poller que consulta login a cada interval.
func New(fetcher Fetcher, login string, interval time.Duration) *Poller {
	return &Poller{
		fetcher:  fetcher,
		login:    login,
		interval: interval,
	}
}

// Start dispara a goroutine de polling e devolve o canal por onde os updates
// saem. O canal é fechado quando ctx é cancelado — é isso que faz o
// `for range` de quem consome terminar sozinho, sem precisar de nenhum outro
// sinal combinado.
//
// O retorno é <-chan (canal só de leitura) para que ninguém de fora consiga
// escrever nem fechar o canal por engano. Quem cria o canal é quem fecha.
func (p *Poller) Start(ctx context.Context) <-chan Update {
	updates := make(chan Update)

	go func() {
		defer close(updates)

		ticker := time.NewTicker(p.interval)
		// Sem o Stop o ticker continua agendando disparos depois que a
		// goroutine morre, segurando memória à toa.
		defer ticker.Stop()

		// O ticker só dispara depois do primeiro intervalo. Sem esta consulta
		// inicial, a janela abriria vazia e ficaria assim por um minuto.
		if !p.emit(ctx, updates) {
			return
		}

		for {
			// select bloqueia até que UM dos casos esteja pronto. É o que
			// permite esperar o próximo tick e o cancelamento ao mesmo tempo,
			// sem consumir CPU e sem atrasar o encerramento.
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !p.emit(ctx, updates) {
					return
				}
			}
		}
	}()

	return updates
}

// emit faz uma consulta e entrega o resultado no canal.
// Devolve false quando o contexto foi cancelado e o laço deve terminar.
func (p *Poller) emit(ctx context.Context, updates chan<- Update) bool {
	update, err := p.check(ctx)
	if err != nil {
		// Se o erro veio do próprio cancelamento, não é uma falha para
		// reportar — é o encerramento normal.
		if ctx.Err() != nil {
			return false
		}
		update = Update{Err: err}
	}

	// O canal não tem buffer: este envio bloqueia até alguém receber. Se o
	// consumidor sumir sem cancelar o ctx, a goroutine ficaria travada aqui
	// para sempre — um vazamento clássico. O segundo caso do select é o que
	// garante a saída.
	select {
	case updates <- update:
		return true
	case <-ctx.Done():
		return false
	}
}

// check faz uma consulta e monta o Update comparando com o ciclo anterior.
//
// Está separado do laço de propósito: assim dá para testar a regra do "mudou
// ou não" sem esperar ticker nenhum.
func (p *Poller) check(ctx context.Context) (Update, error) {
	cal, err := p.fetcher.FetchCalendar(ctx, p.login)
	if err != nil {
		return Update{}, err
	}

	// A comparação é `>` e não `!=`: o total pode cair legitimamente (um repo
	// privado deixa de contar, o período de um ano desliza e perde um dia
	// antigo). Isso não é uma contribuição nova e não deve piscar a janela.
	//
	// O primed evita o falso positivo do primeiro ciclo, quando lastTotal
	// ainda é 0 e qualquer total pareceria um aumento.
	changed := p.primed && cal.Total > p.lastTotal

	p.lastTotal = cal.Total
	p.primed = true

	return Update{Calendar: cal, Changed: changed}, nil
}
