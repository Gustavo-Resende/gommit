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

	"github.com/Gustavo-Resende/gommit/internal/github"
	"github.com/Gustavo-Resende/gommit/internal/poller"
)

const pollInterval = time.Minute

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	login := os.Getenv("GITHUB_LOGIN")
	if token == "" || login == "" {
		fmt.Fprintln(os.Stderr, "defina GITHUB_TOKEN e GITHUB_LOGIN no ambiente")
		os.Exit(1)
	}

	// NotifyContext devolve um contexto que se cancela sozinho no Ctrl+C.
	// O efeito em cascata: o select do poller acorda, a goroutine retorna, o
	// defer fecha o canal, o for abaixo termina e o main sai limpo — sem
	// matar o processo no meio de uma requisição.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	client := github.NewClient(token)
	p := poller.New(client, login, pollInterval)

	fmt.Printf("consultando %s a cada %s — Ctrl+C para sair\n", login, pollInterval)

	// range num canal consome até ele ser fechado. Nenhuma condição de parada
	// aqui: quem decide o fim é o produtor.
	for update := range p.Start(ctx) {
		if update.Err != nil {
			fmt.Fprintln(os.Stderr, "erro:", update.Err)
			continue
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
