package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeEnv cria um .env descartável e devolve o caminho.
//
// t.TempDir() dá um diretório que o próprio testing apaga no fim — nada de
// sujeira no repositório e nenhum teste enxergando o arquivo do outro.
func writeEnv(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("escrevendo .env de teste: %v", err)
	}
	return path
}

// clearEnv zera as variáveis para o teste não depender do ambiente da máquina.
//
// t.Setenv em vez de os.Setenv: ele restaura o valor anterior no fim do teste
// automaticamente. Com os.Setenv o vazamento entre testes seria silencioso e o
// resultado dependeria da ordem de execução.
func clearEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GITHUB_LOGIN", "")
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_LOGIN")
}

func TestLoadLeArquivo(t *testing.T) {
	clearEnv(t)

	// Comentário, linha em branco, aspas, espaço em volta do "=" e CRLF: o
	// formato real de um .env editado à mão no Windows.
	path := writeEnv(t, "# comentario\r\n\r\nGITHUB_TOKEN = \"ghp_abc\"\r\nGITHUB_LOGIN=fulano\r\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load devolveu erro: %v", err)
	}
	if cfg.Token != "ghp_abc" {
		t.Errorf("Token = %q, queria %q", cfg.Token, "ghp_abc")
	}
	if cfg.Login != "fulano" {
		t.Errorf("Login = %q, queria %q", cfg.Login, "fulano")
	}
}

func TestLoadAmbienteVenceArquivo(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITHUB_TOKEN", "do_ambiente")

	path := writeEnv(t, "GITHUB_TOKEN=do_arquivo\nGITHUB_LOGIN=fulano\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load devolveu erro: %v", err)
	}
	if cfg.Token != "do_ambiente" {
		t.Errorf("Token = %q, queria o valor do ambiente", cfg.Token)
	}
}

func TestLoadValorComIgual(t *testing.T) {
	clearEnv(t)

	path := writeEnv(t, "GITHUB_TOKEN=a=b=c\nGITHUB_LOGIN=fulano\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load devolveu erro: %v", err)
	}
	if cfg.Token != "a=b=c" {
		t.Errorf("Token = %q, queria %q — o corte deve ser só no primeiro '='", cfg.Token, "a=b=c")
	}
}

func TestLoadSemArquivoUsaAmbiente(t *testing.T) {
	clearEnv(t)
	t.Setenv("GITHUB_TOKEN", "ghp_abc")
	t.Setenv("GITHUB_LOGIN", "fulano")

	// Caminho inexistente: é o caso de produção, onde não há .env nenhum.
	if _, err := Load(filepath.Join(t.TempDir(), "nao-existe")); err != nil {
		t.Fatalf("arquivo ausente não devia ser erro: %v", err)
	}
}

func TestLoadReclamaDasDuasFaltas(t *testing.T) {
	clearEnv(t)

	_, err := Load(filepath.Join(t.TempDir(), "nao-existe"))
	if err == nil {
		t.Fatal("queria erro quando falta tudo, veio nil")
	}
	for _, quero := range []string{"GITHUB_TOKEN", "GITHUB_LOGIN"} {
		if !strings.Contains(err.Error(), quero) {
			t.Errorf("erro %q devia citar %s", err, quero)
		}
	}
}
