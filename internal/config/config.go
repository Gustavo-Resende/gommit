// Package config resolve as credenciais do app.
//
// A fonte de verdade é o ambiente do processo. O arquivo .env é apenas uma
// conveniência de desenvolvimento: ele preenche o que ainda não está definido
// e some do caminho em produção, onde as variáveis chegam prontas.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// envFileName é o nome procurado por ResolveEnvFile.
const envFileName = ".env"

// Config são as credenciais que o app precisa para subir.
type Config struct {
	Token string
	Login string
}

// ResolveEnvFile devolve o caminho do .env a usar, procurando em três lugares.
//
// O problema que ela resolve: um caminho relativo é resolvido a partir do
// diretório de onde o processo foi iniciado, não do diretório do executável.
// Com `go run` na raiz do repositório os dois coincidem e a diferença não
// aparece — mas um widget é aberto por atalho, e aí o diretório atual é
// qualquer um (o C:\Windows\System32 da vida). O .env sumiria e o app subiria
// reclamando de credencial faltando.
//
// A ordem é deliberada:
//
//  1. Diretório atual — vence tudo, para dar para rodar um build apontando
//     para outro .env sem mexer em mais nada.
//  2. Diretório do executável — o .env viaja junto do .exe.
//  3. %AppData%\gommit\.env (ou o equivalente do SO) — o lugar onde a
//     credencial fica de uma vez e sobrevive a recompilar o binário.
//
// Não devolve erro: se nada for encontrado, devolve o nome simples e deixa o
// Load seguir. Arquivo ausente não é falha — as variáveis podem vir prontas do
// ambiente, que é a fonte de verdade.
func ResolveEnvFile() string {
	if _, err := os.Stat(envFileName); err == nil {
		return envFileName
	}

	if exe, err := os.Executable(); err == nil {
		// EvalSymlinks porque o os.Executable pode devolver um link: em
		// `go run` o binário vive num diretório temporário, e no Linux é comum
		// o executável estar linkado. Sem resolver, o Dir apontaria para o
		// lugar errado.
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		if p := filepath.Join(filepath.Dir(exe), envFileName); existe(p) {
			return p
		}
	}

	if dir, err := os.UserConfigDir(); err == nil {
		if p := filepath.Join(dir, "gommit", envFileName); existe(p) {
			return p
		}
	}

	return envFileName
}

func existe(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Load carrega o arquivo envFile (quando existe) e devolve as credenciais.
//
// A ordem importa: primeiro o arquivo preenche as lacunas, depois a leitura
// acontece — sempre via os.Getenv, para que exista um único lugar de onde a
// configuração sai, não dois caminhos concorrentes.
func Load(envFile string) (Config, error) {
	if err := loadFile(envFile); err != nil {
		return Config{}, err
	}

	cfg := Config{
		Token: os.Getenv("GITHUB_TOKEN"),
		Login: os.Getenv("GITHUB_LOGIN"),
	}

	// Reportar as duas faltas de uma vez em vez de parar na primeira: quem
	// esqueceu de configurar geralmente esqueceu as duas, e descobrir isso em
	// duas execuções seguidas é irritante à toa.
	var missing []string
	if cfg.Token == "" {
		missing = append(missing, "GITHUB_TOKEN")
	}
	if cfg.Login == "" {
		missing = append(missing, "GITHUB_LOGIN")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("config: faltando %s (defina no ambiente ou em %s)",
			strings.Join(missing, " e "), envFile)
	}

	return cfg, nil
}

// loadFile lê pares CHAVE=VALOR de path e os injeta no ambiente do processo.
//
// Ausência do arquivo não é erro: em produção ele não existe mesmo, e as
// variáveis chegam pelo ambiente. Só a leitura mal-sucedida de um arquivo que
// existe é que merece parar o app.
func loadFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("config: abrindo %s: %w", path, err)
	}
	defer f.Close()

	// bufio.Scanner em vez de ler o arquivo inteiro e dar strings.Split("\n"):
	// o Scanner descarta o \r final das linhas, e este arquivo foi criado no
	// Windows (CRLF). Sem isso o token carregaria um \r invisível no fim e a
	// API responderia 401 sem nenhuma pista do motivo.
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// strings.Cut corta no PRIMEIRO "=" e devolve o resto inteiro. Com
		// strings.Split o valor viraria um slice e um token contendo "="
		// seria truncado no meio.
		name, value, ok := strings.Cut(line, "=")
		if !ok {
			continue // linha sem "=": ignora em vez de derrubar o app.
		}

		name = strings.TrimSpace(name)
		value = strings.Trim(strings.TrimSpace(value), `"'`)

		// O ambiente real vence o arquivo. É a regra do 12-factor e ela tem um
		// efeito prático: dá para sobrescrever um valor do .env só naquela
		// execução, sem editar o arquivo.
		//
		// A armadilha é a mesma vantagem de cabeça para baixo — se sobrou um
		// $env:GITHUB_TOKEN velho na sessão, mexer no .env não muda nada.
		if _, defined := os.LookupEnv(name); defined {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("config: definindo %s: %w", name, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("config: lendo %s: %w", path, err)
	}

	return nil
}
