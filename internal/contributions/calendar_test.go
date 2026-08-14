package contributions

import (
	"testing"
	"time"
)

func TestTodayIgnoraAHoraDoDia(t *testing.T) {
	dia := func(d int) time.Time {
		return time.Date(2026, time.August, d, 0, 0, 0, 0, time.Local)
	}

	cal := Calendar{
		Total: 10,
		Weeks: []Week{{Days: []Day{
			{Date: dia(12), Count: 3},
			{Date: dia(13), Count: 7},
		}}},
	}

	// Meio da tarde: se a comparação fosse feita com == em time.Time, este
	// caso falharia mesmo sendo o dia certo.
	agora := time.Date(2026, time.August, 13, 15, 30, 0, 0, time.Local)

	hoje, ok := cal.Today(agora)
	if !ok {
		t.Fatal("nao encontrou o dia de hoje no calendario")
	}
	if hoje.Count != 7 {
		t.Errorf("Count = %d, queria 7", hoje.Count)
	}
}

func TestTodayForaDoPeriodo(t *testing.T) {
	cal := Calendar{Weeks: []Week{{Days: []Day{
		{Date: time.Date(2026, time.August, 13, 0, 0, 0, 0, time.Local)},
	}}}}

	fora := time.Date(2030, time.January, 1, 0, 0, 0, 0, time.Local)

	if _, ok := cal.Today(fora); ok {
		t.Error("esperava ok=false para data fora do calendario")
	}
}
