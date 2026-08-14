package render

import (
	"strings"
	"testing"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/contributions"
)

func dia(d int, nivel contributions.Level) contributions.Day {
	return contributions.Day{
		Date:  time.Date(2026, time.August, d, 0, 0, 0, 0, time.Local),
		Count: int(nivel),
		Level: nivel,
	}
}

// linhas devolve o grid quebrado, sem a quebra final, para as asserções.
func linhas(out string) []string {
	return strings.Split(strings.TrimRight(out, "\n"), "\n")
}

func TestGridPoeODiaNaLinhaDoDiaDaSemana(t *testing.T) {
	// 12/08/2026 é uma quarta-feira: linha 3 do grid. O dia está sozinho na
	// semana, no índice 0 do slice — se o código usasse a posição no slice em
	// vez do Weekday(), ele cairia na linha 0 e este teste pegaria.
	cal := contributions.Calendar{
		Weeks: []contributions.Week{{Days: []contributions.Day{dia(12, contributions.LevelFourthQuartile)}}},
	}

	ls := linhas(Grid(cal, StylePlain))

	// +1 porque a primeira linha é o cabeçalho de meses.
	const linhaQuarta = 3 + 1
	if !strings.ContainsRune(ls[linhaQuarta], '█') {
		t.Errorf("a quarta-feira devia estar na linha %d, veio %q", linhaQuarta, ls[linhaQuarta])
	}

	for i := 1; i <= 7; i++ {
		if i == linhaQuarta {
			continue
		}
		if strings.ContainsRune(ls[i], '█') {
			t.Errorf("linha %d (%q) nao devia ter celula preenchida", i, ls[i])
		}
	}
}

func TestGridDiaAusenteNaoViraNivelZero(t *testing.T) {
	// Uma semana com um único dia: as outras seis células são buracos, não
	// dias sem contribuição. Trocar um pelo outro pintaria seis quadradinhos
	// apagados que não existem no calendário.
	cal := contributions.Calendar{
		Weeks: []contributions.Week{{Days: []contributions.Day{dia(12, contributions.LevelNone)}}},
	}

	ls := linhas(Grid(cal, StylePlain))

	if got := strings.Count(strings.Join(ls[1:8], ""), string(blocos[0])); got != 1 {
		t.Errorf("queria exatamente 1 celula de nivel 0 no grid, achei %d", got)
	}
}

func TestGridPlainNaoEmiteEscape(t *testing.T) {
	cal := contributions.Calendar{
		Weeks: []contributions.Week{{Days: []contributions.Day{dia(12, contributions.LevelThirdQuartile)}}},
	}

	if out := Grid(cal, StylePlain); strings.Contains(out, "\x1b") {
		t.Error("StylePlain nao pode emitir escape ANSI — e o modo para saida redirecionada")
	}
}

func TestGridColorFechaCadaLinhaComReset(t *testing.T) {
	cal := contributions.Calendar{
		Weeks: []contributions.Week{{Days: []contributions.Day{dia(12, contributions.LevelThirdQuartile)}}},
	}

	// Sem o reset no fim da linha a última cor vaza para o prompt do terminal.
	for i, l := range linhas(Grid(cal, StyleColor))[1:8] {
		if !strings.HasSuffix(l, reset) {
			t.Errorf("linha %d nao termina com reset: %q", i, l)
		}
	}
}

func TestGridVazio(t *testing.T) {
	// Calendário sem semanas não pode dar panic no acesso ao primeiro dia.
	if out := Grid(contributions.Calendar{}, StylePlain); out == "" {
		t.Error("queria alguma mensagem para o calendario vazio")
	}
}
