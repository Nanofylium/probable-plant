/* Nepenthes: passeio vivo. Mostra os quadros que `nep mundo` gerou (com o
   LingBot-World ou simulados) e deixa o visitante andar: segurar ↑, a tela ou
   o botão anda; ↓ volta; ← e → escolhem o caminho. Com {rolar: true}, rolar a
   página também anda. */
(function () {
  if (window.__nepMundo) return;
  window.__nepMundo = 1;

  var CSS =
    ".nep-mundo{margin:28px auto;max-width:1100px;position:relative}" +
    ".nep-mundo>img{display:block;width:100%;height:auto;border-radius:18px}" +
    ".nep-mundo figcaption{margin-top:12px;text-align:center;color:inherit}.nep-mundo figcaption span{opacity:.75}" +
    ".nep-mundo-rolo{position:relative}.nep-mundo-fixo{position:sticky;top:max(12px,calc(50vh - 34vw))}" +
    ".nep-mundo-palco{position:relative;border-radius:18px;overflow:hidden;background:#111;box-shadow:0 18px 50px rgba(0,0,0,.22);touch-action:pan-y;user-select:none;-webkit-user-select:none;outline:none}" +
    ".nep-mundo-palco:focus-visible{box-shadow:0 0 0 3px var(--nep-cor,#2f6b4f),0 18px 50px rgba(0,0,0,.22)}" +
    ".nep-mundo-palco canvas{display:block;width:100%;height:auto;max-height:66vh;object-fit:cover}" +
    ".nep-mundo-dica{position:absolute;left:50%;bottom:18px;transform:translateX(-50%);background:rgba(0,0,0,.62);color:#fff;padding:9px 16px;border-radius:999px;font-size:.92rem;white-space:nowrap;pointer-events:none;transition:opacity .4s;backdrop-filter:blur(6px)}" +
    ".nep-mundo-barra{position:absolute;left:0;bottom:0;height:4px;background:var(--nep-cor,#2f6b4f);width:0;transition:width .08s linear}" +
    ".nep-mundo-carga{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;color:#fff;background:rgba(0,0,0,.35);font-size:.95rem;transition:opacity .4s}" +
    ".nep-mundo-selo{position:absolute;top:12px;right:12px;background:rgba(0,0,0,.5);color:#fff;font-size:.72rem;padding:4px 10px;border-radius:999px;letter-spacing:.02em}" +
    ".nep-mundo-ctl{display:flex;flex-wrap:wrap;gap:8px;justify-content:center;align-items:center;margin-top:14px}" +
    ".nep-mundo-ctl button{font:inherit;border-radius:999px;padding:9px 16px;border:1px solid rgba(127,127,127,.35);background:transparent;color:inherit;cursor:pointer;touch-action:none}" +
    ".nep-mundo-ctl button[aria-pressed=true]{background:var(--nep-cor,#2f6b4f);border-color:var(--nep-cor,#2f6b4f);color:#fff}" +
    ".nep-mundo-ctl .nep-mundo-andar{background:var(--nep-cor,#2f6b4f);border-color:var(--nep-cor,#2f6b4f);color:#fff;font-weight:700;padding:10px 22px}" +
    ".nep-mundo-sep{width:1px;height:26px;background:rgba(127,127,127,.3);margin:0 4px}" +
    ".nep-mundo .nep-mundo-ctl button{text-transform:none!important;letter-spacing:0!important;font-size:.92rem;line-height:1.2;min-height:0;box-shadow:none}" +
    "@media (max-width:560px){.nep-mundo-ctl{gap:6px}.nep-mundo .nep-mundo-ctl button{font-size:.82rem;padding:7px 11px}.nep-mundo .nep-mundo-ctl .nep-mundo-andar{padding:8px 16px}.nep-mundo-sep{display:none}" +
    ".nep-mundo-dica{font-size:.8rem;bottom:12px}.nep-mundo-selo{top:8px;right:8px}}";

  function el(tag, cls, txt) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (txt != null) e.textContent = txt;
    return e;
  }
  function pad(n) { return ("00" + n).slice(-3); }

  function montar(fig) {
    var g;
    try { g = JSON.parse(fig.getAttribute("data-mundo")); } catch (e) { return; }
    fig.setAttribute("data-pronto", "1");
    var img0 = fig.querySelector("img"), legenda = fig.querySelector("figcaption");
    var W = g.largura || 832, H = g.altura || 480, fps = g.fps || 16;
    var palco = el("div", "nep-mundo-palco"), cv = el("canvas"), ctx = cv.getContext("2d");
    cv.width = W; cv.height = H;
    palco.tabIndex = 0;
    palco.setAttribute("role", "application");
    palco.setAttribute("aria-label", "Passeio: segure a seta para cima para andar, seta para baixo para voltar, esquerda e direita para escolher o caminho");
    var toque = window.matchMedia && matchMedia("(pointer: coarse)").matches;
    var dica = el("div", "nep-mundo-dica", toque ? "Segure a tela para andar" : "Segure ↑ ou a tela para andar"), barra = el("div", "nep-mundo-barra"), carga = el("div", "nep-mundo-carga", "Carregando o passeio…");
    var selo = el("div", "nep-mundo-selo", g.fonte === "lingbot-world" ? "Passeio gerado com LingBot-World" : "Passeio simulado");
    palco.appendChild(cv); palco.appendChild(barra); palco.appendChild(selo); palco.appendChild(carga); palco.appendChild(dica);
    var ctl = el("div", "nep-mundo-ctl");
    var voltar = el("button", "", "↓ Voltar"), andar = el("button", "nep-mundo-andar", "Andar ↑");
    voltar.type = andar.type = "button";
    var botoes = g.caminhos.map(function (c, i) {
      var b = el("button", "", c.nome);
      b.type = "button";
      b.setAttribute("aria-pressed", i === 0 ? "true" : "false");
      b.onclick = function () { escolher(i); palco.focus({preventScroll: true}); };
      return b;
    });
    if (botoes.length > 1) { botoes.forEach(function (b) { ctl.appendChild(b); }); ctl.appendChild(el("span", "nep-mundo-sep")); }
    ctl.appendChild(voltar); ctl.appendChild(andar);

    var alvo = fig;
    if (g.rolar) {
      // a página rola por uma faixa alta enquanto o passeio fica parado na tela
      var rolo = el("div", "nep-mundo-rolo"), fixo = el("div", "nep-mundo-fixo");
      rolo.style.height = "260vh";
      fixo.appendChild(palco); fixo.appendChild(ctl); rolo.appendChild(fixo);
      fig.insertBefore(rolo, img0);
      alvo = rolo;
    } else {
      fig.insertBefore(palco, img0);
      fig.insertBefore(ctl, img0);
    }
    img0.remove();

    // quadros: carrega o caminho atual primeiro, os outros depois
    var cache = g.caminhos.map(function () { return []; }), prontos = g.caminhos.map(function () { return 0; });
    function carregar(ci, depois) {
      var c = g.caminhos[ci];
      if (cache[ci].length) { if (depois) depois(); return; }
      for (var i = 0; i < c.quadros; i++) {
        var im = new Image();
        im.decoding = "async";
        im.onload = im.onerror = function () {
          prontos[ci]++;
          if (ci === atual) {
            carga.textContent = "Carregando o passeio… " + Math.round(prontos[ci] * 100 / c.quadros) + "%";
            if (prontos[ci] >= c.quadros) carga.style.opacity = 0;
          }
          if (prontos[ci] === 1 || prontos[ci] === c.quadros) pede();
          if (prontos[ci] === c.quadros && depois) depois();
        };
        im.src = g.base + c.id + "/" + pad(i + 1) + "." + c.ext;
        cache[ci].push(im);
      }
    }

    var atual = 0, pos = 0, dir = 0, proximo = -1, ultimo = 0, rodando = false, dicaFora = false;
    function n() { return g.caminhos[atual].quadros; }
    function desenhar() {
      var qs = cache[atual], i = Math.floor(pos), f = pos - i;
      var a = qs[Math.min(i, qs.length - 1)], b = qs[Math.min(i + 1, qs.length - 1)];
      if (a && a.complete && a.naturalWidth) {
        ctx.globalAlpha = 1;
        ctx.drawImage(a, 0, 0, W, H);
        if (f > .02 && b !== a && b && b.complete && b.naturalWidth) { ctx.globalAlpha = f; ctx.drawImage(b, 0, 0, W, H); ctx.globalAlpha = 1; }
      }
      barra.style.width = (pos / Math.max(n() - 1, 1) * 100).toFixed(1) + "%";
    }
    function passo(t) {
      var dt = Math.min((t - ultimo) / 1000, .1);
      ultimo = t;
      var v = dir * fps;
      if (proximo >= 0) v = -fps * 3; // volta depressa ao começo antes de trocar de caminho
      pos = Math.max(0, Math.min(n() - 1, pos + v * dt));
      if (proximo >= 0 && pos <= 0) {
        atual = proximo; proximo = -1;
        botoes.forEach(function (b, i) { b.setAttribute("aria-pressed", i === atual ? "true" : "false"); });
        carga.style.opacity = prontos[atual] >= n() ? 0 : 1;
        carregar(atual);
      }
      desenhar();
      if (dir !== 0 || proximo >= 0) requestAnimationFrame(passo);
      else rodando = false;
    }
    function pede() {
      if (rodando) { return; }
      if (dir === 0 && proximo < 0) { requestAnimationFrame(desenhar); return; }
      rodando = true; ultimo = performance.now(); requestAnimationFrame(passo);
    }
    function mover(d) {
      dir = d;
      if (d && !dicaFora) { dicaFora = true; dica.style.opacity = 0; }
      pede();
    }
    function escolher(i) {
      if (i === atual && proximo < 0) return;
      carregar(i);
      if (pos <= 0) { proximo = i; pos = 0; rodando = true; ultimo = performance.now(); requestAnimationFrame(passo); return; }
      proximo = i; pede();
    }

    // ← vai pelo caminho da esquerda (→ pela direita); sem ele, passa para o vizinho
    function lado(id, passo) {
      for (var i = 0; i < g.caminhos.length; i++) if (g.caminhos[i].id === id) { escolher(i === atual ? 0 : i); return; }
      escolher((atual + passo + g.caminhos.length) % g.caminhos.length);
    }

    // segurar: botões, tela, teclado
    function segurar(b, d) {
      b.addEventListener("pointerdown", function (e) { e.preventDefault(); b.setPointerCapture && b.setPointerCapture(e.pointerId); mover(d); });
      ["pointerup", "pointercancel", "lostpointercapture"].forEach(function (ev) { b.addEventListener(ev, function () { if (dir === d) mover(0); }); });
      b.addEventListener("keydown", function (e) { if (e.key === " " || e.key === "Enter") { e.preventDefault(); mover(d); } });
      b.addEventListener("keyup", function () { mover(0); });
    }
    segurar(andar, 1); segurar(voltar, -1);
    if (!g.rolar) segurar(cv, 1);
    palco.addEventListener("keydown", function (e) {
      var k = e.key;
      if (k === "ArrowUp" || k === "w" || k === "W") { e.preventDefault(); mover(1); }
      else if (k === "ArrowDown" || k === "s" || k === "S") { e.preventDefault(); mover(-1); }
      else if (k === "ArrowLeft" || k === "a" || k === "A") { e.preventDefault(); lado("esquerda", -1); }
      else if (k === "ArrowRight" || k === "d" || k === "D") { e.preventDefault(); lado("direita", 1); }
    });
    palco.addEventListener("keyup", function (e) { if (/^(Arrow(Up|Down)|[wWsS])$/.test(e.key)) mover(0); });
    palco.addEventListener("blur", function () { mover(0); });

    if (g.rolar) {
      dica.textContent = toque ? "Deslize a página para andar" : "Role a página para andar";
      var rolou = function () {
        var r = alvo.getBoundingClientRect(), total = r.height - window.innerHeight * .7;
        var p = Math.max(0, Math.min(1, -r.top / Math.max(total, 1)));
        if (dir === 0 && proximo < 0) {
          pos = p * (n() - 1);
          if (p > 0 && !dicaFora) { dicaFora = true; dica.style.opacity = 0; }
          requestAnimationFrame(desenhar);
        }
      };
      window.addEventListener("scroll", rolou, {passive: true});
    }

    // só baixa os quadros quando o passeio chega perto da tela
    var comecar = function () {
      carregar(0, function () { g.caminhos.forEach(function (_, i) { if (i) carregar(i); }); });
    };
    if (window.IntersectionObserver) {
      var io = new IntersectionObserver(function (es) { if (es[0].isIntersecting) { io.disconnect(); comecar(); } }, {rootMargin: "400px"});
      io.observe(fig);
    } else comecar();
  }

  function todos() {
    if (!document.getElementById("nep-mundo-css")) { var st = el("style"); st.id = "nep-mundo-css"; st.textContent = CSS; document.head.appendChild(st); }
    Array.prototype.forEach.call(document.querySelectorAll(".nep-mundo[data-mundo]:not([data-pronto])"), montar);
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", todos);
  else todos();
  window.nepMundos = todos;
})();
