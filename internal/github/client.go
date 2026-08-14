// Package github conversa com a API GraphQL do GitHub e traduz a resposta
// para os tipos de domínio do pacote contributions.
//
// A tradução acontece aqui de propósito: se o formato da API mudar, ou se a
// origem dos dados virar outra, o estrago fica contido neste pacote e o resto
// do app não percebe.
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Gustavo-Resende/gommit/internal/contributions"
)

const endpoint = "https://api.github.com/graphql"

// dateLayout é o formato das datas na resposta ("2024-03-15"). Em Go o layout
// não usa símbolos: você escreve a data de referência (2 de janeiro de 2006,
// 15h04:05) no formato desejado.
const dateLayout = "2006-01-02"

// contributionsQuery busca o calendário do último ano. O GraphQL do GitHub
// exige POST com o corpo {"query": ..., "variables": {...}} — não existe
// equivalente REST para este dado.
const contributionsQuery = `
query($login: String!) {
  user(login: $login) {
    contributionsCollection {
      contributionCalendar {
        totalContributions
        weeks {
          contributionDays {
            date
            contributionCount
            contributionLevel
          }
        }
      }
    }
  }
}`

// Client faz as chamadas autenticadas à API.
//
// O http.Client vive aqui dentro, e não numa variável global, para que o
// timeout seja explícito e para que os testes possam injetar um transporte
// falso sem mexer em estado compartilhado.
type Client struct {
	httpClient *http.Client
	token      string
}

// NewClient devolve um Client pronto para uso com o token informado.
func NewClient(token string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		token:      token,
	}
}

// FetchCalendar busca o calendário de contribuições do usuário login.
//
// O ctx é o primeiro parâmetro por convenção: é ele que permite abortar a
// requisição quando o app está fechando, em vez de esperar o timeout inteiro.
func (c *Client) FetchCalendar(ctx context.Context, login string) (contributions.Calendar, error) {
	payload, err := json.Marshal(graphQLRequest{
		Query:     contributionsQuery,
		Variables: map[string]any{"login": login},
	})
	if err != nil {
		return contributions.Calendar{}, fmt.Errorf("github: montando o corpo: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return contributions.Calendar{}, fmt.Errorf("github: criando a requisição: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return contributions.Calendar{}, fmt.Errorf("github: chamando a API: %w", err)
	}
	// O corpo é um io.ReadCloser aberto: sem este Close a conexão nunca volta
	// para o pool e o programa vaza sockets. O defer garante que ele rode em
	// qualquer caminho de saída daqui para baixo.
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Um 401 traz a explicação no corpo. Sem isso, o erro seria só
		// "401 Unauthorized" e você ficaria adivinhando o que houve.
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return contributions.Calendar{}, fmt.Errorf("github: status %s: %s", resp.Status, bytes.TrimSpace(snippet))
	}

	var parsed apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return contributions.Calendar{}, fmt.Errorf("github: decodificando a resposta: %w", err)
	}

	// Armadilha do GraphQL: erro de negócio (login inexistente, token sem
	// escopo, query malformada) volta com status 200 e um array "errors" no
	// corpo. Checar só o status code não basta.
	if len(parsed.Errors) > 0 {
		return contributions.Calendar{}, fmt.Errorf("github: %s", parsed.Errors[0].Message)
	}

	return toCalendar(parsed)
}

// graphQLRequest é o corpo que o endpoint espera.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

// Os tipos abaixo espelham a forma da resposta. São unexported porque só
// existem para o json.Decoder preencher: nada fora deste pacote precisa
// conhecer o formato da API.
//
// As tags `json:"..."` são o que liga o campo Go ao nome no JSON. Sem elas o
// decoder faz um match case-insensitive que funciona por acidente em alguns
// casos e falha em silêncio nos outros — deixando o campo zerado sem erro.
type apiResponse struct {
	Data   apiData    `json:"data"`
	Errors []apiError `json:"errors"`
}

type apiError struct {
	Message string `json:"message"`
}

type apiData struct {
	User apiUser `json:"user"`
}

type apiUser struct {
	ContributionsCollection apiCollection `json:"contributionsCollection"`
}

type apiCollection struct {
	ContributionCalendar apiCalendar `json:"contributionCalendar"`
}

type apiCalendar struct {
	TotalContributions int       `json:"totalContributions"`
	Weeks              []apiWeek `json:"weeks"`
}

type apiWeek struct {
	ContributionDays []apiDay `json:"contributionDays"`
}

type apiDay struct {
	Date              string `json:"date"`
	ContributionCount int    `json:"contributionCount"`
	ContributionLevel string `json:"contributionLevel"`
}

// toCalendar converte a resposta crua da API no tipo de domínio.
//
// É aqui que a fronteira entre "formato de fora" e "modelo de dentro" fica
// concreta: depois desta função ninguém mais no app vê string de data nem
// enum em texto.
func toCalendar(r apiResponse) (contributions.Calendar, error) {
	src := r.Data.User.ContributionsCollection.ContributionCalendar

	cal := contributions.Calendar{
		Total: src.TotalContributions,
		// make com capacidade: o tamanho final já é conhecido, então o append
		// não precisa realocar o slice a cada crescimento.
		Weeks: make([]contributions.Week, 0, len(src.Weeks)),
	}

	for _, w := range src.Weeks {
		week := contributions.Week{
			Days: make([]contributions.Day, 0, len(w.ContributionDays)),
		}

		for _, d := range w.ContributionDays {
			// ParseInLocation em vez de Parse: Parse assumiria UTC, e aí um
			// dia do calendário "começaria" às 21h do dia anterior no fuso de
			// Brasília — o quadrado de hoje acenderia na hora errada.
			date, err := time.ParseInLocation(dateLayout, d.Date, time.Local)
			if err != nil {
				return contributions.Calendar{}, fmt.Errorf("github: data inválida %q: %w", d.Date, err)
			}

			week.Days = append(week.Days, contributions.Day{
				Date:  date,
				Count: d.ContributionCount,
				Level: parseLevel(d.ContributionLevel),
			})
		}

		cal.Weeks = append(cal.Weeks, week)
	}

	return cal, nil
}

// parseLevel traduz o enum de contributionLevel da API para o tipo do domínio.
func parseLevel(s string) contributions.Level {
	switch s {
	case "FIRST_QUARTILE":
		return contributions.LevelFirstQuartile
	case "SECOND_QUARTILE":
		return contributions.LevelSecondQuartile
	case "THIRD_QUARTILE":
		return contributions.LevelThirdQuartile
	case "FOURTH_QUARTILE":
		return contributions.LevelFourthQuartile
	default:
		// Cobre "NONE" e qualquer nível novo que a API venha a inventar.
		// Degradar para o quadrado apagado é melhor que derrubar o app.
		return contributions.LevelNone
	}
}
