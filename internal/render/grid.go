// Package render desenha o calendário como texto para o terminal.
//
// É uma etapa de prova, não o produto final: a janela do Wails vai desenhar o
// mesmo dado em HTML. Mas a organização do grid — semanas em colunas, dias da
// semana em linhas, buracos nas pontas — é a mesma nos dois, e é mais barato
// acertar isso aqui do que dentro do frontend.
package render

import (
	"strings"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/contributions"
)

// Style diz como pintar as células.
//
// Uma constante nomeada em vez de um `bool` no parâmetro: na chamada,
// `Grid(cal, true)` não diz nada a quem lê, enquanto `Grid(cal, StyleColor)`
// se explica sozinho.
type Style int

const (
	// StyleColor usa cor de 24 bits — o que o Windows Terminal entende.
	StyleColor Style = iota
	// StylePlain troca cor por densidade do caractere. Serve para console
	// velho, saída redirecionada para arquivo e para os testes, que ficam
	// legíveis sem escapes no meio.
	StylePlain
)

// diasNaSemana é 7 por definição do calendário, não por acaso do dado. Fixar
// aqui deixa claro que o grid tem sempre 7 linhas, mesmo que a primeira e a
// última semana venham incompletas.
const diasNaSemana = 7

// vazia marca a célula sem dia correspondente: as pontas do período caem no
// meio de uma semana. Sem esse sentinela, o nível 0 ("dia sem contribuição")
// e "dia que não existe" ficariam indistinguíveis.
const vazia = -1

// As cores são as do próprio GitHub, em escapes de 24 bits (38;2;R;G;B).
// Índice = contributions.Level, então a ordem importa.
var cores = [...]string{
	"\x1b[38;2;45;51;59m",  // nenhuma
	"\x1b[38;2;14;68;41m",  // 1º quartil
	"\x1b[38;2;0;109;50m",  // 2º quartil
	"\x1b[38;2;38;166;65m", // 3º quartil
	"\x1b[38;2;57;211;83m", // 4º quartil
}

const reset = "\x1b[0m"

// blocos são o equivalente sem cor: a intensidade vira densidade do caractere.
var blocos = [...]rune{'·', '░', '▒', '▓', '█'}

// rotulos tem exatamente 3 runas cada, e é disso que o alinhamento depende.
//
// Armadilha: len("sáb") é 4, não 3 — len conta bytes, e o "á" ocupa dois em
// UTF-8. Por isso o alinhamento aqui é feito com literais de largura fixa e
// nunca com len(). Um strings.Repeat(" ", 4-len(r)) desalinharia a linha.
var rotulos = [...]string{"dom ", "seg ", "ter ", "qua ", "qui ", "sex ", "sáb "}

// margem alinha o cabeçalho de meses com o início das células.
const margem = "    "

var meses = [...]string{"jan", "fev", "mar", "abr", "mai", "jun", "jul", "ago", "set", "out", "nov", "dez"}

// Grid devolve o calendário inteiro como um bloco de texto, já com \n no fim.
//
// Devolve string em vez de escrever num io.Writer porque quem chama decide o
// destino — hoje o terminal, amanhã um teste ou um log. Se isso virar gargalo
// (não vai: são ~370 células), a versão com Writer evita a alocação.
func Grid(cal contributions.Calendar, style Style) string {
	if len(cal.Weeks) == 0 {
		return "(calendário vazio)\n"
	}

	// grade[diaDaSemana][semana] — invertido em relação ao dado, que vem como
	// semanas contendo dias. O terminal escreve linha a linha, e uma linha é
	// um dia da semana atravessando o ano inteiro; sem essa transposição seria
	// preciso varrer todas as semanas sete vezes.
	grade := make([][]int, diasNaSemana)
	for i := range grade {
		grade[i] = make([]int, len(cal.Weeks))
		for j := range grade[i] {
			grade[i][j] = vazia
		}
	}

	for coluna, semana := range cal.Weeks {
		for _, d := range semana.Days {
			// A linha vem do Weekday() do próprio dia, não da posição dele no
			// slice: na primeira semana do período o slice pode começar numa
			// quarta-feira, e contar a partir do índice jogaria o ano todo
			// para a linha errada.
			grade[int(d.Date.Weekday())][coluna] = int(d.Level)
		}
	}

	// strings.Builder em vez de `s += ...` num laço: concatenar strings aloca
	// e copia tudo de novo a cada volta (O(n²)). O Builder escreve num buffer
	// que cresce amortizado.
	var b strings.Builder

	b.WriteString(cabecalhoMeses(cal.Weeks))
	b.WriteByte('\n')

	// `range` sobre um int (Go 1.22+) no lugar do for clássico de três partes:
	// só o contador importa aqui, e a forma curta não deixa espaço para errar
	// a condição de parada.
	for linha := range diasNaSemana {
		b.WriteString(rotulos[linha])
		for _, nivel := range grade[linha] {
			b.WriteString(celula(nivel, style))
		}
		// O reset vai no fim da linha, não em cada célula: sem ele a última
		// cor "vaza" e o prompt do terminal sai pintado de verde.
		if style == StyleColor {
			b.WriteString(reset)
		}
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(legenda(style))
	b.WriteByte('\n')

	return b.String()
}

// celula devolve o caractere de uma célula, já com a cor quando for o caso.
func celula(nivel int, style Style) string {
	if nivel == vazia {
		return " "
	}
	// Trava defensiva: se a API inventar um nível novo, o parseLevel já
	// degrada para 0 — mas um índice fora do array aqui seria panic, e widget
	// nenhum deve morrer por causa de um enum novo.
	if nivel < 0 || nivel >= len(blocos) {
		nivel = 0
	}

	if style == StylePlain {
		return string(blocos[nivel])
	}
	return cores[nivel] + "█"
}

// cabecalhoMeses monta a régua de meses alinhada às colunas das semanas.
func cabecalhoMeses(semanas []contributions.Week) string {
	// []rune e não []byte: os nomes são ASCII hoje, mas escrever em runas
	// deixa o índice significando "coluna do grid" em vez de "byte", que é o
	// que a posição precisa querer dizer.
	linha := make([]rune, len(semanas))
	for i := range linha {
		linha[i] = ' '
	}

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

		nome := []rune(meses[mes-1])
		// Só escreve se couber e não colidir com o nome anterior — a primeira
		// semana do período costuma ser parcial e pode cair colada na
		// seguinte.
		if coluna >= proximaLivre && coluna+len(nome) <= len(linha) {
			copy(linha[coluna:], nome)
			proximaLivre = coluna + len(nome) + 1
		}
	}

	return margem + string(linha)
}

// legenda desenha a escala de intensidade no mesmo estilo do grid.
func legenda(style Style) string {
	var b strings.Builder
	b.WriteString(margem + "menos ")
	for nivel := range blocos {
		b.WriteString(celula(nivel, style))
	}
	if style == StyleColor {
		b.WriteString(reset)
	}
	b.WriteString(" mais")
	return b.String()
}
