// Command gommit é, por enquanto, uma sonda de terminal: liga o cliente ao
// poller e imprime cada atualização. Quando a etapa da GUI chegar, este
// arquivo vira o entrypoint do Wails e os pacotes internos continuam iguais —
// é justamente por isso que nenhum deles sabe que existe terminal.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/config"
	"github.com/Gustavo-Resende/gommit/internal/github"
	"github.com/Gustavo-Resende/gommit/internal/poller"
	"github.com/Gustavo-Resende/gommit/internal/render"
)

const pollInterval = time.Minute

// envFile é relativo ao diretório de onde o processo foi iniciado, não ao do
// executável. Com `go run .` na raiz do repositório dá na mesma; quando isto
// virar um .exe de widget aberto por atalho, vai ser preciso resolver o
// caminho a partir do os.Executable().
const envFile = ".env"

func main() {
	cfg, err := config.Load(envFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// NotifyContext devolve um contexto que se cancela sozinho no Ctrl+C.
	// O efeito em cascata: o select do poller acorda, a goroutine retorna, o
	// defer fecha o canal, o for abaixo termina e o main sai limpo — sem
	// matar o processo no meio de uma requisição.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := github.NewClient(cfg.Token)
	p := poller.New(client, cfg.Login, pollInterval)

	fmt.Printf("consultando %s a cada %s — Ctrl+C para sair\n", cfg.Login, pollInterval)

	// NO_COLOR é convenção estabelecida (no-color.org): qualquer valor definido
	// desliga a cor. Vale para console antigo e para quando a saída é
	// redirecionada para arquivo, onde escape ANSI vira lixo.
	style := render.StyleColor
	if os.Getenv("NO_COLOR") != "" {
		style = render.StylePlain
	}

	// primeiro controla o desenho inicial. Redesenhar o grid a cada ciclo
	// encheria o scrollback com 1440 grids por dia; ele só aparece na primeira
	// consulta e quando algo muda, e o resto vira uma linha de sinal de vida.
	primeiro := true

	// range num canal consome até ele ser fechado. Nenhuma condição de parada
	// aqui: quem decide o fim é o produtor.
	for update := range p.Start(ctx) {
		if update.Err != nil {
			fmt.Fprintln(os.Stderr, "erro:", update.Err)
			continue
		}

		if primeiro || update.Changed {
			fmt.Print("\n" + render.Grid(update.Calendar, style))
			primeiro = false
		}

		line := fmt.Sprintf("total=%d", update.Calendar.Total)

		if today, ok := update.Calendar.Today(time.Now()); ok {
			line += fmt.Sprintf(" hoje=%d nivel=%d", today.Count, today.Level)
		}

		if update.Changed {
			line += "  <- contribuicao nova!"
		}

		fmt.Println(line)
	}
}
