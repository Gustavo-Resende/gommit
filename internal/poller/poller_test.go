package poller

import (
	"context"
	"testing"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/contributions"
)

// fakeFetcher é a razão prática de a interface Fetcher existir: aqui o poller
// roda inteiro sem token, sem rede e sem esperar um minuto por ciclo.
type fakeFetcher struct {
	totals []int
	calls  int
}

func (f *fakeFetcher) FetchCalendar(ctx context.Context, login string) (contributions.Calendar, error) {
	total := f.totals[f.calls]
	f.calls++
	return contributions.Calendar{Total: total}, nil
}

func TestCheckDetectaAumentoDoTotal(t *testing.T) {
	fetcher := &fakeFetcher{totals: []int{10, 10, 12, 11}}
	p := New(fetcher, "alguem", time.Minute)

	// Table-driven test: o formato padrão em Go para varrer casos sem repetir
	// o corpo do teste. Os ciclos rodam em sequência porque o estado do
	// poller é justamente o que está sendo verificado.
	casos := []struct {
		nome    string
		mudanca bool
	}{
		{"primeiro ciclo nunca conta como mudanca", false},
		{"total igual nao e mudanca", false},
		{"total maior e mudanca", true},
		{"total menor nao e mudanca", false},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			update, err := p.check(context.Background())
			if err != nil {
				t.Fatalf("check devolveu erro: %v", err)
			}
			if update.Changed != caso.mudanca {
				t.Errorf("Changed = %v, queria %v", update.Changed, caso.mudanca)
			}
		})
	}
}

func TestStartFechaOCanalAoCancelar(t *testing.T) {
	fetcher := &fakeFetcher{totals: []int{42}}
	// Intervalo longo de propósito: só a consulta inicial deve acontecer.
	p := New(fetcher, "alguem", time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	updates := p.Start(ctx)

	if first := <-updates; first.Calendar.Total != 42 {
		t.Fatalf("primeiro update com total %d", first.Calendar.Total)
	}

	cancel()

	select {
	case _, aberto := <-updates:
		if aberto {
			t.Fatal("esperava o canal fechado apos o cancelamento")
		}
	case <-time.After(time.Second):
		t.Fatal("o canal nao fechou apos o cancelamento")
	}
}
