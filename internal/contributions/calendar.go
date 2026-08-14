// Package contributions modela o calendário de contribuições do GitHub: o
// total do período, as semanas e os dias que formam o quadriculado do perfil.
//
// Este pacote é deliberadamente burro: só tipos e regras que dependem apenas
// deles mesmos. Ele não sabe que existe HTTP, GraphQL ou janela — quem traduz
// a resposta da API para cá é o pacote github.
package contributions

import "time"

// Level é a intensidade do verde de um dia, na mesma escala que o GitHub usa.
// O valor vem pronto da API (contributionLevel) porque o corte de cada quartil
// depende do histórico do usuário, não só da contagem do dia.
type Level int

const (
	LevelNone Level = iota
	LevelFirstQuartile
	LevelSecondQuartile
	LevelThirdQuartile
	LevelFourthQuartile
)

// Day é um quadradinho do grid.
//
// Date é sempre a meia-noite local do dia — quem constrói o Day garante isso
// (ver github.toCalendar). Sem essa garantia, comparar datas vira loteria.
type Day struct {
	Date  time.Time
	Count int
	Level Level
}

// Week é uma coluna do grid: até sete dias, de domingo a sábado. A primeira e
// a última semana do período costumam vir incompletas.
type Week struct {
	Days []Day
}

// Calendar é o calendário inteiro de um período.
type Calendar struct {
	Total int
	Weeks []Week
}

// Today devolve o dia correspondente a now e informa se ele foi encontrado.
//
// O segundo retorno booleano é o idioma padrão em Go para "achei ou não" — o
// mesmo do acesso a mapa. Evita devolver um erro para uma ausência que é
// perfeitamente normal (o calendário pode simplesmente não cobrir a data).
func (c Calendar) Today(now time.Time) (Day, bool) {
	year, month, day := now.Date()

	for _, week := range c.Weeks {
		for _, d := range week.Days {
			// Comparar time.Time com == é uma armadilha: a igualdade leva em
			// conta hora, fuso e até o relógio monotônico embutido. Dois
			// valores que representam o mesmo instante podem não ser iguais.
			// Por isso a comparação é feita pelos componentes da data.
			y, m, dd := d.Date.Date()
			if y == year && m == month && dd == day {
				return d, true
			}
		}
	}

	return Day{}, false
}
