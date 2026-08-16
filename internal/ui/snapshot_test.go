package ui

import (
	"encoding/json"
	"errors"
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

func semana(dias ...contributions.Day) contributions.Week {
	return contributions.Week{Days: dias}
}

func TestWeekdayVemDoDiaNaoDaPosicao(t *testing.T) {
	// 12/08/2026 é uma quarta-feira (Weekday 3), sozinha no índice 0 do slice.
	// Se o Weekday saísse da posição no slice, viria 0 — e o ano inteiro
	// desandaria uma linha no frontend.
	cal := contributions.Calendar{Weeks: []contributions.Week{semana(dia(12, contributions.LevelFourthQuartile))}}

	snap := FromCalendar(cal, false, time.Date(2026, time.August, 12, 10, 0, 0, 0, time.Local))

	if got := snap.Weeks[0][0].Weekday; got != 3 {
		t.Errorf("Weekday da quarta-feira devia ser 3, veio %d", got)
	}
}

func TestDateSaiSemHoraNemFuso(t *testing.T) {
	// O contrato com o JS é uma data pura. Se vazar hora ou offset, o frontend
	// volta a ter que reparsear — e a armadilha de fuso volta junto.
	cal := contributions.Calendar{Weeks: []contributions.Week{semana(dia(12, contributions.LevelNone))}}

	got := FromCalendar(cal, false, time.Now()).Weeks[0][0].Date
	if got != "2026-08-12" {
		t.Errorf("Date devia ser 2026-08-12, veio %q", got)
	}
}

func TestTodayNuloQuandoForaDoPeriodo(t *testing.T) {
	cal := contributions.Calendar{Weeks: []contributions.Week{semana(dia(12, contributions.LevelNone))}}

	snap := FromCalendar(cal, false, time.Date(2026, time.August, 16, 0, 0, 0, 0, time.Local))

	if snap.Today != nil {
		t.Errorf("Today devia ser nulo fora do periodo, veio %+v", *snap.Today)
	}
}

func TestTodayPreenchidoIgnoraAHoraDoDia(t *testing.T) {
	cal := contributions.Calendar{Weeks: []contributions.Week{semana(dia(12, contributions.LevelThirdQuartile))}}

	// Fim da tarde: o dia do calendário é meia-noite, então comparar instantes
	// falharia. A comparação tem que ser por componentes da data.
	snap := FromCalendar(cal, false, time.Date(2026, time.August, 12, 18, 30, 0, 0, time.Local))

	if snap.Today == nil {
		t.Fatal("Today devia estar preenchido")
	}
	if snap.Today.Level != int(contributions.LevelThirdQuartile) {
		t.Errorf("Level do dia de hoje veio %d", snap.Today.Level)
	}
}

func TestSnapshotZeradoNaoEstaReady(t *testing.T) {
	// É o estado da janela recém-aberta, antes da primeira resposta da API.
	// Sem esse flag ele seria indistinguível de um ano sem contribuição nenhuma.
	var vazio Snapshot
	if vazio.Ready {
		t.Error("o snapshot zerado nao pode se dizer pronto")
	}

	if snap := FromCalendar(contributions.Calendar{}, false, time.Now()); !snap.Ready {
		t.Error("um ciclo bem-sucedido, mesmo vazio, e Ready")
	}
}

func TestWithErrorPreservaOsDadosEDesligaAAnimacao(t *testing.T) {
	cal := contributions.Calendar{
		Total: 42,
		Weeks: []contributions.Week{semana(dia(12, contributions.LevelFirstQuartile))},
	}
	bom := FromCalendar(cal, true, time.Now())

	comErro := bom.WithError(errors.New("sem rede"))

	if comErro.Total != 42 || len(comErro.Weeks) != 1 {
		t.Error("o grid antigo tem que continuar na tela quando um ciclo falha")
	}
	if comErro.Error != "sem rede" {
		t.Errorf("mensagem de erro veio %q", comErro.Error)
	}
	if comErro.Changed {
		t.Error("um ciclo que falhou nao e contribuicao nova — a animacao repetiria a cada erro")
	}
	// WithError é value receiver justamente para não mexer no original, que já
	// pode estar sendo lido pelo frontend.
	if bom.Error != "" || !bom.Changed {
		t.Error("WithError nao pode mutar o snapshot de origem")
	}
}

func TestMesesNaoSeAtropelam(t *testing.T) {
	// Semana parcial no fim de julho seguida de agosto: sem a folga, os dois
	// rótulos sairiam em colunas coladas e ficariam ilegíveis.
	julho := contributions.Day{Date: time.Date(2026, time.July, 31, 0, 0, 0, 0, time.Local)}
	agosto := contributions.Day{Date: time.Date(2026, time.August, 2, 0, 0, 0, 0, time.Local)}

	got := mesesDe([]contributions.Week{semana(julho), semana(agosto)})

	if len(got) != 1 || got[0].Label != "jul" {
		t.Errorf("queria so o rotulo de julho, veio %+v", got)
	}
}

func TestJSONUsaNomesMinusculos(t *testing.T) {
	// O frontend lê snap.total, não snap.Total. Um rename em Go sem ajustar a
	// tag quebraria o JS em silêncio — este teste é o alarme.
	cal := contributions.Calendar{Total: 7, Weeks: []contributions.Week{semana(dia(12, contributions.LevelNone))}}

	out, err := json.Marshal(FromCalendar(cal, false, time.Now()))
	if err != nil {
		t.Fatal(err)
	}

	for _, campo := range []string{`"ready"`, `"total"`, `"changed"`, `"weeks"`, `"months"`, `"weekday"`, `"date"`} {
		if !strings.Contains(string(out), campo) {
			t.Errorf("faltou o campo %s no JSON: %s", campo, out)
		}
	}
	// omitempty: sem erro e sem hoje, os campos somem em vez de virar null.
	if strings.Contains(string(out), `"error"`) {
		t.Error("error sem valor devia sumir do JSON")
	}
}
