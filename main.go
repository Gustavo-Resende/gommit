// Command gommit é o widget: uma janela sem moldura, sempre visível, com o
// grid de contribuições do GitHub.
//
// Este arquivo é a única parte do app que conhece o Wails. Os pacotes internal/
// não mudaram nada para a janela existir — a mesma pilha (config → github →
// poller) alimenta a sonda de terminal em cmd/probe.
package main

import (
	"embed"
	"fmt"
	"os"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/config"
	"github.com/Gustavo-Resende/gommit/internal/github"
	"github.com/Gustavo-Resende/gommit/internal/poller"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// A diretiva abaixo empacota o frontend dentro do binário — é o que faz o .exe
// ser um arquivo só, sem pasta de assets ao lado. O prefixo `all:` é preciso para
// incluir arquivos começados com "_" ou "."; sem ele o embed os ignora em
// silêncio, e você descobre com uma janela em branco no build (mas não no dev,
// que lê do disco).
//
//go:embed all:frontend/dist
var assets embed.FS

const pollInterval = time.Minute

func main() {
	// A falha de configuração não aborta o processo de propósito. Um widget é
	// aberto por atalho, sem terminal: um os.Exit(1) aqui seria um clique que
	// não faz absolutamente nada, sem nenhuma pista do motivo. Melhor abrir a
	// janela e dizer o que faltou.
	var p *poller.Poller
	cfg, err := config.Load(config.ResolveEnvFile())
	if err == nil {
		p = poller.New(github.NewClient(cfg.Token), cfg.Login, pollInterval)
	} else {
		// A sonda ainda é usada do terminal, então o stderr continua valendo.
		fmt.Fprintln(os.Stderr, err)
	}

	app := NewApp(p, cfg.Login, err)

	// As dimensões saem do conteúdo, somando o que o CSS define — a janela é
	// frameless e não redimensionável, então sobra de espaço aparece como
	// retângulo vazio e falta corta o grid.
	//
	//	largura = 28 (padding) + 27 (rótulos + gap) + 54×11 + 53×3 = 808
	//	altura  = 12 + 19 (cabeçalho) + 8 + 13 (meses) + 95 (7 linhas) + 12 = 159
	//
	// 54 colunas e não 53: a janela de um ano quase sempre cai em 53 semanas,
	// mas começando e terminando no meio de uma, dá 54 colunas parciais. Errar
	// para mais custa alguns pixels vazios; errar para menos corta o quadrado
	// de hoje, que é justamente o que o widget existe para mostrar.
	erro := wails.Run(&options.App{
		Title:  "gommit",
		Width:  812,
		Height: 164,

		// Frameless tira a barra de título — é o que diferencia um widget de um
		// app. O custo é que a janela deixa de ter botão de fechar e de ser
		// arrastável pela barra, e as duas coisas passam a ser
		// responsabilidade do frontend (App.Quit e --wails-draggable).
		Frameless:     true,
		AlwaysOnTop:   true,
		DisableResize: true,

		// Alpha 0: a janela em si não pinta nada, quem pinta é o CSS. É o que
		// permite o controle de opacidade do painel de ajustes — se a cor de
		// fundo fosse opaca aqui, nenhum rgba() do frontend atravessaria.
		BackgroundColour: &options.RGBA{R: 13, G: 17, B: 23, A: 0},

		Windows: &windows.Options{
			// As duas andam juntas e fazem coisas diferentes:
			// WebviewIsTransparent faz o webview respeitar alpha 0 no CSS;
			// WindowIsTranslucent faz a janela por baixo dele ser translúcida.
			// Só a primeira deixaria o buraco preto em vez de vazado.
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},

		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup,

		// Bind expõe os métodos exportados do App para o JavaScript. Só os
		// exportados: startup e consume ficam de fora, que é o que se quer.
		Bind: []any{app},
	})
	if erro != nil {
		fmt.Fprintln(os.Stderr, "gommit:", erro)
		os.Exit(1)
	}
}
