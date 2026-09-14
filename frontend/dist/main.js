// Frontend do gommit.
//
// Sem import, sem bundler, sem node_modules: o Wails injeta o runtime em
// window.runtime e os métodos ligados em window.go.<pacote>.<Struct>. Como o
// projeto inteiro é stdlib-only, manter o frontend sem dependência também é
// coerente — e um dia o HTML do protótipo visual entra aqui quase direto.

const EVENTO = "calendar:update";
const CHAVE_OPACIDADE = "gommit:opacidade";

// Numa janela frameless de produção não existe console para abrir: um erro de
// JS deixaria a interface pela metade, em silêncio, sem nenhuma pista. Jogar a
// mensagem na barra de status transforma isso num sintoma visível.
window.addEventListener("error", (e) => {
  const s = document.getElementById("status");
  if (s) s.textContent = `js: ${e.message}`;
});

const el = {
  janela: document.getElementById("janela"),
  login: document.getElementById("login"),
  total: document.getElementById("total"),
  status: document.getElementById("status"),
  months: document.getElementById("months"),
  grid: document.getElementById("grid"),
  tooltip: document.getElementById("tooltip"),
  config: document.getElementById("config"),
  painel: document.getElementById("painel"),
  opacidade: document.getElementById("opacidade"),
  opacidadeValor: document.getElementById("opacidade-valor"),
  min: document.getElementById("min"),
  quit: document.getElementById("quit"),
};

const App = window.go.main.App;

// ---------- barra de título ----------

el.quit.addEventListener("click", () => App.Quit());
el.min.addEventListener("click", () => App.Minimize());

// O login vem de uma chamada só, no load: ele é config, não dado de ciclo, e
// não muda enquanto o app está aberto.
App.Login()
  .then((login) => (el.login.textContent = login || "gommit"))
  .catch(console.error);

// ---------- dados ----------

// Duas fontes para o mesmo desenho, e as duas são necessárias:
//
// - O evento cobre as atualizações seguintes.
// - A chamada ao Snapshot() cobre a primeira, porque o backend começa a
//   consultar antes de esta página existir. Sem ela, o primeiro emit sai para
//   uma plateia vazia e a janela ficaria em branco por um minuto.
window.runtime.EventsOn(EVENTO, desenhar);
App.Snapshot().then(desenhar).catch(console.error);

function desenhar(snap) {
  if (!snap) return;

  el.status.textContent = snap.error || "";

  // Ready separa "ainda não busquei" de "busquei e deu zero". Sem essa
  // distinção a janela recém-aberta mentiria dizendo que o ano está vazio.
  if (!snap.ready) {
    el.total.textContent = snap.error ? "" : "carregando…";
    return;
  }

  el.total.textContent = `${snap.total} contribuições no último ano`;

  desenharMeses(snap.months);
  desenharGrid(snap);
}

function desenharMeses(months) {
  el.months.replaceChildren();
  for (const m of months || []) {
    const span = document.createElement("span");
    span.textContent = m.label;
    // +1 porque grid-column é 1-based e o Go conta colunas do zero.
    span.style.gridColumn = m.column + 1;
    el.months.append(span);
  }
}

function desenharGrid(snap) {
  const hoje = snap.today ? snap.today.date : null;

  // Um fragmento e um replaceChildren só no fim: montar direto no #grid faria
  // o navegador recalcular layout a cada uma das ~370 células.
  const frag = document.createDocumentFragment();
  let celulaDeHoje = null;

  snap.weeks.forEach((semana, coluna) => {
    for (const dia of semana) {
      const cell = document.createElement("div");
      cell.className = "cell";
      cell.style.background = `var(--nivel-${dia.level})`;

      // Aqui está a razão de o Go mandar o weekday pronto: a linha vem do dia
      // da semana, nunca da posição dentro do array. A primeira semana do
      // período costuma ser parcial e começar numa quarta — contar pelo índice
      // jogaria o ano inteiro para a linha errada.
      cell.style.gridRow = dia.weekday + 1;
      cell.style.gridColumn = coluna + 1;

      // dataset em vez de title: o tooltip é nosso, e o nativo apareceria
      // junto, com um segundo de atraso e a cara do sistema.
      cell.dataset.data = dia.date;
      cell.dataset.count = dia.count;

      // A classe não pinta nada hoje (o grid não marca o dia atual de forma
      // permanente, só no hover) — fica como gancho semântico para quem for
      // estilizar depois. Quem guarda a célula para a animação é a variável.
      if (dia.date === hoje) {
        cell.classList.add("today");
        celulaDeHoje = cell;
      }

      frag.append(cell);
    }
  });

  el.grid.replaceChildren(frag);
  esconderTooltip();

  // A animação é o feedback de "contribuição nova detectada". Ela só existe
  // quando o total subiu — o poller já filtra queda de total (o período de um
  // ano desliza e perde dias antigos, o que não é contribuição nova).
  if (snap.changed && celulaDeHoje) {
    celulaDeHoje.classList.add("pulse");
    celulaDeHoje.addEventListener(
      "animationend",
      () => celulaDeHoje.classList.remove("pulse"),
      { once: true },
    );
  }
}

// ---------- tooltip ----------

// Delegação: um listener no container em vez de 370. Além de mais barato, ele
// sobrevive ao replaceChildren — listeners presos às células morreriam com elas
// a cada redesenho.
el.grid.addEventListener("mouseover", (e) => {
  const cell = e.target.closest(".cell");
  if (cell) mostrarTooltip(cell);
});
el.grid.addEventListener("mouseleave", esconderTooltip);

function mostrarTooltip(cell) {
  const n = Number(cell.dataset.count);
  const contagem = n === 0 ? "Nenhuma contribuição" : `${n} contribuiç${n === 1 ? "ão" : "ões"}`;

  el.tooltip.innerHTML = `<b></b> <span></span>`;
  el.tooltip.querySelector("b").textContent = contagem;
  el.tooltip.querySelector("span").textContent = `em ${formatarData(cell.dataset.data)}`;
  el.tooltip.hidden = false;

  // Posiciona depois de exibir: escondido, o elemento não tem dimensão e o
  // cálculo de centralização sairia todo errado.
  const c = cell.getBoundingClientRect();
  const t = el.tooltip.getBoundingClientRect();

  let x = c.left + c.width / 2 - t.width / 2;
  // Clamp nas bordas: sem isso, os quadrados das pontas empurram o tooltip
  // para fora da janela, que tem overflow hidden — ele simplesmente sumiria.
  x = Math.max(4, Math.min(x, window.innerWidth - t.width - 4));

  // Acima da célula, ou abaixo se não couber.
  let y = c.top - t.height - 6;
  if (y < 4) y = c.bottom + 6;

  el.tooltip.style.left = `${x}px`;
  el.tooltip.style.top = `${y}px`;
}

function esconderTooltip() {
  el.tooltip.hidden = true;
}

// formatarData converte "2026-08-16" em "16 de agosto".
//
// A armadilha: new Date("2026-08-16") é interpretado como UTC pelo padrão do
// JS. Em UTC-3 isso vira 15/08 às 21h, e o tooltip mostraria o dia anterior.
// Quebrar a string e usar o construtor de componentes cria a data no fuso
// local, que é o que o Go já garantiu do outro lado.
function formatarData(iso) {
  const [ano, mes, dia] = iso.split("-").map(Number);
  return new Date(ano, mes - 1, dia).toLocaleDateString("pt-BR", {
    day: "numeric",
    month: "long",
  });
}

// ---------- ajustes ----------

el.config.addEventListener("click", () => {
  const abrindo = el.painel.hidden;
  el.painel.hidden = !abrindo;
  el.config.setAttribute("aria-expanded", String(abrindo));
});

// Fecha ao clicar fora. Sem isso o painel fica aberto tapando o grid, e num
// widget desse tamanho isso é metade da tela.
document.addEventListener("click", (e) => {
  if (el.painel.hidden) return;
  if (el.painel.contains(e.target) || el.config.contains(e.target)) return;
  el.painel.hidden = true;
  el.config.setAttribute("aria-expanded", "false");
});

el.opacidade.addEventListener("input", () => aplicarOpacidade(el.opacidade.value));

function aplicarOpacidade(valor) {
  el.janela.style.setProperty("--opacidade", valor / 100);
  el.opacidadeValor.textContent = `${valor}%`;
  el.opacidade.value = valor;
  // localStorage do webview persiste entre execuções, então a preferência
  // sobrevive a fechar o app — sem precisar de arquivo de config no Go.
  localStorage.setItem(CHAVE_OPACIDADE, valor);
}

aplicarOpacidade(Number(localStorage.getItem(CHAVE_OPACIDADE)) || 100);
