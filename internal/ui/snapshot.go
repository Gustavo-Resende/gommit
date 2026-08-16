// Package ui traduz o domínio para a forma que o frontend consome.
//
// Ele existe pelo mesmo motivo que o pacote github existe: manter uma fronteira.
// O github traduz "o que vem de fora" para o domínio; o ui traduz o domínio para
// "o que vai para fora". No meio, o pacote contributions continua sem saber que
// existe JSON, HTTP ou janela.
//
// A alternativa seria mandar contributions.Calendar direto para o webview — o
// Wails serializa qualquer struct exportada. Foi descartada por três motivos:
//
//  1. Day.Date é um time.Time e sairia como "2026-08-16T00:00:00-03:00". O JS
//     teria que reparsear a data, e aí a armadilha de fuso que o domínio já
//     resolveu voltaria a valer no frontend.
//  2. Sem tags `json`, os campos chegam capitalizados (Total, Weeks) — feio de
//     consumir e frágil, porque um rename em Go quebraria o JS em silêncio.
//  3. Para pôr tags `json` no domínio seria preciso sujá-lo com uma preocupação
//     de transporte, que é exatamente o que a doc dele promete não fazer.
package ui

import (
	"time"

	"github.com/Gustavo-Resende/gommit/internal/contributions"
)

// dateLayout é o formato que sai para o frontend: só a data, sem hora nem fuso.
// É a mesma decisão do item 1 acima — o JS não precisa saber que existe fuso,
// porque tudo que ele faz com essa string é comparar e exibir.
const dateLayout = "2006-01-02"

// meses duplica os nomes que o pacote render já tem. É duplicação de propósito:
// o alternativo seria o ui importar o render (um pacote de terminal) ou criar um
// terceiro pacote só para sete strings. Se um dia virarem configuráveis ou
// traduzíveis, aí sim vale extrair — hoje seria acoplamento pago à toa.
var meses = [...]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

// Day é um quadradinho, pronto para virar uma célula do grid.
type Day struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
	Level int    `json:"level"`

	// Weekday é 0=domingo … 6=sábado, e é o que diz em qual das sete linhas a
	// célula entra. Vem calculado do Go de propósito: é a mesma lição que o
	// render.Grid aprendeu — a linha vem do dia da semana, nunca da posição do
	// dia dentro da semana, porque a primeira semana do período costuma ser
	// parcial e começar numa quarta-feira qualquer.
	Weekday int `json:"weekday"`
}

// Month é um rótulo da régua de meses, posicionado por coluna.
type Month struct {
	Label string `json:"label"`

	// Column é o índice da semana onde o mês começa. O frontend usa isso como
	// grid-column, então o valor precisa casar com o índice em Weeks.
	Column int `json:"column"`
}

// Snapshot é o estado inteiro que o frontend precisa para desenhar uma vez.
//
// É um retrato completo, não um delta: o frontend redesenha do zero a cada
// atualização. Com ~370 células isso é barato, e economiza todo o estado
// incremental que um diff exigiria dos dois lados.
type Snapshot struct {
	// Ready distingue "ainda não busquei" de "busquei e o resultado é zero".
	// Sem ele, a janela recém-aberta seria indistinguível de um ano sem nenhuma
	// contribuição — é o mesmo raciocínio do sentinela `vazia` no render.
	Ready bool `json:"ready"`

	Total int `json:"total"`

	// Changed dispara a animação no quadrado de hoje.
	Changed bool `json:"changed"`

	// Error é a falha do último ciclo, quando houve. Vem junto com os dados
	// antigos em vez de substituí-los: numa queda de rede o grid continua na
	// tela e o erro aparece de canto, que é o comportamento certo para uma
	// janela que fica aberta o dia todo.
	Error string `json:"error,omitempty"`

	// Today é ponteiro para poder ser nulo — o calendário pode não cobrir a
	// data de hoje (fuso da conta do GitHub à frente do relógio local, virada
	// do dia entre dois ciclos). Nulo em JS é `null`, e o frontend consegue
	// distinguir "não sei" de "zero".
	Today *Day `json:"today,omitempty"`

	Weeks  [][]Day `json:"weeks"`
	Months []Month `json:"months"`
}

// FromCalendar monta o snapshot de um ciclo bem-sucedido.
//
// now entra como parâmetro em vez de a função chamar time.Now() por dentro:
// é o que torna o resultado determinístico e o teste possível sem congelar o
// relógio do processo.
func FromCalendar(cal contributions.Calendar, changed bool, now time.Time) Snapshot {
	snap := Snapshot{
		Ready:   true,
		Total:   cal.Total,
		Changed: changed,
		// make com capacidade conhecida: evita realocar a cada append.
		Weeks: make([][]Day, 0, len(cal.Weeks)),
	}

	for _, semana := range cal.Weeks {
		dias := make([]Day, 0, len(semana.Days))
		for _, d := range semana.Days {
			dias = append(dias, toDay(d))
		}
		snap.Weeks = append(snap.Weeks, dias)
	}

	if hoje, ok := cal.Today(now); ok {
		// Variável local e depois &: não dá para pegar o endereço do retorno de
		// uma função direto, e reaproveitar a variável do range seria pior —
		// em Go 1.22+ cada iteração tem a sua, mas depender disso é sutil demais.
		dia := toDay(hoje)
		snap.Today = &dia
	}

	snap.Months = mesesDe(cal.Weeks)

	return snap
}

// WithError devolve uma cópia do snapshot com a mensagem de erro anexada.
//
// Recebe e devolve por valor porque Snapshot é lido concorrentemente: quem
// chama guarda a cópia sob lock em vez de mutar o que já entregou ao frontend.
// Changed é zerado — um ciclo que falhou não é uma contribuição nova, e deixar
// o flag ligado faria a animação repetir a cada erro.
func (s Snapshot) WithError(err error) Snapshot {
	s.Error = err.Error()
	s.Changed = false
	return s
}

func toDay(d contributions.Day) Day {
	return Day{
		Date:    d.Date.Format(dateLayout),
		Count:   d.Count,
		Level:   int(d.Level),
		Weekday: int(d.Date.Weekday()),
	}
}

// mesesDe monta a régua de meses: um rótulo em cada coluna onde o mês vira.
func mesesDe(semanas []contributions.Week) []Month {
	var out []Month
	anterior := time.Month(0)
	proximaLivre := 0

	for coluna, semana := range semanas {
		if len(semana.Days) == 0 {
			continue
		}

		mes := semana.Days[0].Date.Month()
		if mes == anterior {
			continue
		}
		anterior = mes

		// A folga de 2 colunas evita dois rótulos colados, que é o que acontece
		// quando a primeira semana do período é parcial e cai no fim de um mês.
		// Mesma ideia do proximaLivre no render.Grid, só que aqui a unidade é
		// coluna do grid e não caractere.
		if coluna < proximaLivre {
			continue
		}

		out = append(out, Month{Label: meses[mes-1], Column: coluna})
		proximaLivre = coluna + 3
	}

	return out
}
