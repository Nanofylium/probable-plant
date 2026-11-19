/* Nepenthes: jogos de promoção (roleta, raspadinha, memória) desenhados com o
   Hilo (Alibaba, MIT). O prêmio vem do servidor; aqui só a animação. */
(function () {
  if (window.__nepJogos) return;
  window.__nepJogos = 1;
  var me = document.currentScript, HILO = me.getAttribute("data-hilo"), API = me.getAttribute("data-api");

  function carregar(pronto) {
    if (window.Hilo && Hilo.Stage) return pronto();
    var s = document.createElement("script");
    s.src = HILO; s.onload = pronto; document.head.appendChild(s);
  }
  function el(tag, cls, txt) { var e = document.createElement(tag); if (cls) e.className = cls; if (txt) e.textContent = txt; return e; }

  // cores do site
  function cores(box) {
    var cs = getComputedStyle(box), c = (cs.getPropertyValue("--nep-cor") || "").trim() || "#2f6b4f";
    var m = /^#([0-9a-f]{6})$/i.exec(c), rgb = m ? [0, 2, 4].map(function (i) { return parseInt(m[1].substr(i, 2), 16); }) : [47, 107, 79];
    var mx = function (t, k) { return "#" + rgb.map(function (v) { var x = Math.round(v + (t - v) * k); return ("0" + x.toString(16)).slice(-2); }).join(""); };
    return {cor: m ? c : "#2f6b4f", clara: mx(255, .82), media: mx(255, .45), escura: mx(0, .35), texto: cs.color, fonte: getComputedStyle(document.body).fontFamily};
  }

  var CSS = ".nep-jogo{position:relative;text-align:center;max-width:520px;margin:0 auto}" +
    ".nep-jogo-palco{position:relative;margin:14px auto;max-width:100%;touch-action:none}.nep-jogo-palco canvas:not(.nep-raspa){position:relative!important;display:block;margin:0 auto}" +
    ".nep-jogo-form{display:grid;gap:10px;max-width:380px;margin:12px auto;text-align:left}.nep-jogo-form label{display:grid;gap:4px;font-weight:600;font-size:.92rem}" +
    ".nep-jogo-form .aceite{display:flex;gap:8px;align-items:flex-start;font-weight:400;font-size:.85rem}" +
    ".nep-jogo-res{margin:14px auto 0;padding:16px;border-radius:14px;border:2px dashed currentColor;max-width:380px}" +
    ".nep-jogo-res b.cod{display:block;font-size:1.7rem;letter-spacing:.12em;margin:6px 0}.nep-jogo-erro{color:#b42318;font-weight:600}" +
    ".nep-raspa{position:absolute;inset:0;margin:auto;cursor:crosshair;border-radius:18px}";

  function pedir(box, corpo) {
    return fetch(API, {method: "POST", headers: {"Content-Type": "application/json", "X-Nep-Csrf": box.getAttribute("data-csrf")}, body: JSON.stringify(corpo), credentials: "same-origin"})
      .then(function (r) { return r.json().then(function (j) { if (!r.ok || j.erro) throw new Error(j.erro || "Erro " + r.status); return j; }); });
  }

  function montar(box) {
    var g; try { g = JSON.parse(box.getAttribute("data-jogo")); } catch (e) { return; }
    box.setAttribute("data-pronto", "1");
    var c = cores(box), ocupado = false;
    var palco = el("div", "nep-jogo-palco"), form = el("div", "nep-jogo-form"), res = el("div", "nep-jogo-res"), erro = el("p", "nep-jogo-erro");
    res.hidden = true; erro.hidden = true;
    var nome, contato, aceite;
    if (g.contato) {
      form.innerHTML = '<label>Seu nome<input autocomplete="name" maxlength="80"></label><label>WhatsApp ou e-mail<input autocomplete="email" maxlength="80"></label>' +
        '<label class="aceite"><input type="checkbox"> <span>Aceito receber o cupom e as ofertas por este contato. Posso pedir para sair quando quiser.</span></label>';
      var ins = form.querySelectorAll("input"); nome = ins[0]; contato = ins[1]; aceite = ins[2];
    }
    var botao = el("button", "botao", g.botao || {roleta: "Girar a roleta", raspadinha: "Quero raspar", memoria: "Começar o jogo"}[g.tipo]);
    botao.type = "button";
    form.appendChild(botao);
    box.appendChild(palco); box.appendChild(form); box.appendChild(erro); box.appendChild(res);
    function dados(extra) {
      var d = {id: g.id, nome: nome ? nome.value : "", contato: contato ? contato.value : "", aceite: aceite ? aceite.checked : false};
      for (var k in extra) d[k] = extra[k];
      return d;
    }
    function falha(e) { erro.textContent = e.message; erro.hidden = false; ocupado = false; botao.disabled = false; }
    function mostra(r) {
      res.hidden = false; res.innerHTML = "";
      if (!r.cupom) { res.appendChild(el("p", "", r.premio + ". Volte amanhã e tente de novo!")); return; }
      res.appendChild(el("p", "", "Você ganhou: " + r.premio));
      res.appendChild(el("b", "cod", r.codigo));
      var v = r.valido_ate.split("-");
      res.appendChild(el("small", "", "Mostre este código na loja ou no pedido. Vale até " + v[2] + "/" + v[1] + "/" + v[0] + "."));
      if (navigator.clipboard) {
        var cp = el("button", "botao secundario pequeno", "Copiar código"); cp.type = "button"; cp.style.marginTop = "10px";
        cp.onclick = function () { navigator.clipboard.writeText(r.codigo); cp.textContent = "Copiado!"; };
        res.appendChild(el("br")); res.appendChild(cp);
      }
      form.hidden = true;
    }

    // o palco é desenhado em 420 de largura e reduzido no celular (escala do Hilo)
    var L = Math.min(palco.clientWidth || 420, 420), esc = L / 420, alto = {memoria: 330, raspadinha: 360}[g.tipo] || 420;
    var stage = new Hilo.Stage({renderType: "canvas", container: palco, width: 420, height: alto, scaleX: esc, scaleY: esc});
    palco.style.height = Math.round(alto * esc) + "px";
    var atualiza = function () { stage.updateViewport(); };
    window.addEventListener("scroll", atualiza, {passive: true}); window.addEventListener("resize", atualiza);
    stage.enableDOMEvent([Hilo.event.POINTER_START, Hilo.event.POINTER_END], true);
    var ticker = new Hilo.Ticker(60); ticker.addTick(stage); ticker.addTick(Hilo.Tween); ticker.start();

    if (g.tipo === "roleta") roleta(); else if (g.tipo === "raspadinha") raspadinha(); else memoria();

    // ---------- roleta ----------
    function roleta() {
      var n = g.premios.length, R = 190, CX = 210, CY = 222, P = Math.PI;
      var roda = new Hilo.Container({x: CX, y: CY});
      for (var i = 0; i < n; i++) {
        var a0 = -P / 2 - P / n + i * 2 * P / n, a1 = a0 + 2 * P / n;
        var fundoS = i % 2 ? c.clara : c.cor;
        if (n % 2 && i === n - 1) fundoS = c.media;
        var gr = new Hilo.Graphics({width: 2 * R, height: 2 * R, x: -R, y: -R});
        gr.beginFill(fundoS).moveTo(R, R);
        for (var k = 0; k <= 24; k++) { var a = a0 + (a1 - a0) * k / 24; gr.lineTo(R + R * Math.cos(a), R + R * Math.sin(a)); }
        gr.closePath().endFill();
        roda.addChild(gr);
        var suporte = new Hilo.Container({rotation: (a0 + a1) / 2 * 180 / P});
        suporte.addChild(new Hilo.Text({text: g.premios[i], color: fundoS === c.cor ? "#ffffff" : c.escura, font: "700 15px " + c.fonte,
          width: R * .64, maxWidth: R * .64, textAlign: "right", x: R * .28, y: -10}));
        roda.addChild(suporte);
      }
      var aro = new Hilo.Graphics({width: 2 * R + 20, height: 2 * R + 20, x: -R - 10, y: -R - 10});
      aro.lineStyle(10, "#ffffff").drawCircle(10, 10, R).endFill();
      roda.addChild(aro);
      stage.addChild(roda);
      var miolo = new Hilo.Graphics({width: 64, height: 64, x: CX - 32, y: CY - 32});
      miolo.beginFill("#ffffff").drawCircle(2, 2, 30).endFill();
      miolo.lineStyle(4, c.cor).drawCircle(8, 8, 24).endFill();
      stage.addChild(miolo);
      var seta = new Hilo.Graphics({width: 40, height: 44, x: CX - 20, y: CY - R - 26});
      seta.beginFill(c.escura).moveTo(0, 0).lineTo(40, 0).lineTo(20, 40).closePath().endFill();
      stage.addChild(seta);
      botao.onclick = function () {
        if (ocupado) return; ocupado = true; botao.disabled = true; erro.hidden = true;
        pedir(box, dados({acao: "jogar"})).then(function (r) {
          var setor = 360 / n, alvo = 360 * 6 - r.indice * setor + (Math.random() - .5) * setor * .6;
          roda.rotation = roda.rotation % 360;
          Hilo.Tween.to(roda, {rotation: alvo}, {duration: 5200, ease: Hilo.Ease.Cubic.EaseOut, onComplete: function () { mostra(r); }});
        }).catch(falha);
      };
    }

    // ---------- raspadinha ----------
    function raspadinha() {
      var cartao = new Hilo.Graphics({width: 380, height: 260, x: 20, y: 50});
      cartao.beginFill(c.clara).drawRoundRect(0, 0, 380, 260, 18).endFill();
      cartao.lineStyle(3, c.cor).drawRoundRect(8, 8, 364, 244, 14).endFill();
      stage.addChild(cartao);
      var premio = new Hilo.Text({text: "", color: c.escura, font: "800 30px " + c.fonte, textAlign: "center", width: 340, maxWidth: 340, x: 40, y: 168});
      var chapeu = new Hilo.Text({text: "VOCÊ GANHOU", color: c.cor, font: "800 14px " + c.fonte, textAlign: "center", width: 340, x: 40, y: 132});
      stage.addChild(chapeu); stage.addChild(premio);
      var raspa = el("canvas", "nep-raspa"); raspa.width = 380; raspa.height = 260;
      raspa.style.cssText = "position:absolute;left:50%;top:" + (50 * esc) + "px;width:" + (380 * esc) + "px;height:" + (260 * esc) + "px;transform:translateX(-50%);border-radius:18px;cursor:crosshair;touch-action:none";
      palco.appendChild(raspa);
      var x2 = raspa.getContext("2d");
      var gr = x2.createLinearGradient(0, 0, 380, 260); gr.addColorStop(0, "#c9ccd1"); gr.addColorStop(.5, "#eef0f2"); gr.addColorStop(1, "#b6bac0");
      x2.fillStyle = gr; x2.fillRect(0, 0, 380, 260);
      x2.fillStyle = "#555"; x2.font = "700 20px " + c.fonte; x2.textAlign = "center"; x2.fillText("RASPE AQUI", 190, 138);
      var liberado = false, r0 = null, fim = false;
      function raspar(e) {
        if (!liberado || fim) return;
        var b = raspa.getBoundingClientRect(), p = e.touches ? e.touches[0] : e;
        var x = (p.clientX - b.left) * 380 / b.width, y = (p.clientY - b.top) * 260 / b.height;
        x2.globalCompositeOperation = "destination-out"; x2.beginPath(); x2.arc(x, y, 24, 0, 7); x2.fill();
        e.preventDefault();
        var d = x2.getImageData(0, 0, 380, 260).data, vazio = 0;
        for (var i = 3; i < d.length; i += 64) if (d[i] === 0) vazio++;
        if (vazio / (d.length / 64) > .5) { fim = true; raspa.style.transition = "opacity .5s"; raspa.style.opacity = 0; mostra(r0); }
      }
      var apertado = false;
      raspa.addEventListener("pointerdown", function (e) { apertado = true; raspar(e); });
      raspa.addEventListener("pointermove", function (e) { if (apertado) raspar(e); });
      window.addEventListener("pointerup", function () { apertado = false; });
      botao.onclick = function () {
        if (ocupado) return; ocupado = true; botao.disabled = true; erro.hidden = true;
        pedir(box, dados({acao: "jogar"})).then(function (r) {
          r0 = r; liberado = true;
          chapeu.text = r.cupom ? "VOCÊ GANHOU" : "QUE PENA"; premio.text = r.premio;
          botao.textContent = "Agora raspe o cartão";
        }).catch(falha);
      };
    }

    // ---------- memória ----------
    function memoria() {
      var imgs = (g.imagens || []).slice(0, 6), cartas = [], abertas = [], pares = 0, token = null, W = 92, H = 92, gap = 12;
      var baralho = imgs.concat(imgs).map(function (u, i) { return {u: u, k: i % imgs.length}; });
      for (var i = baralho.length - 1; i > 0; i--) { var j = Math.floor(Math.random() * (i + 1)), t = baralho[i]; baralho[i] = baralho[j]; baralho[j] = t; }
      var cols = 4, x0 = (420 - (cols * W + (cols - 1) * gap)) / 2;
      baralho.forEach(function (b, i) {
        var cx = x0 + (i % cols) * (W + gap) + W / 2, cy = 10 + Math.floor(i / cols) * (H + gap) + H / 2;
        var carta = new Hilo.Container({x: cx, y: cy, width: W, height: H, pivotX: W / 2, pivotY: H / 2});
        var frente = new Hilo.Container({visible: false});
        var fundoF = new Hilo.Graphics({width: W, height: H}); fundoF.beginFill("#ffffff").drawRoundRect(0, 0, W, H, 14).endFill();
        fundoF.lineStyle(3, c.cor).drawRoundRect(2, 2, W - 4, H - 4, 12).endFill();
        frente.addChild(fundoF);
        var im = new Image(); im.crossOrigin = "anonymous";
        im.onload = function () {
          var ehFoto = !/^data:image\/svg/.test(b.u), s = ehFoto ? Math.max((W - 8) / im.width, (H - 8) / im.height) : (W * .55) / im.width;
          var bm = new Hilo.Bitmap({image: im, scaleX: s, scaleY: s});
          bm.x = (W - im.width * s) / 2; bm.y = (H - im.height * s) / 2;
          if (ehFoto) { bm.x = 4; bm.y = 4; bm.scaleX = (W - 8) / im.width; bm.scaleY = (H - 8) / im.height; }
          frente.addChild(bm);
        };
        im.src = b.u;
        var verso = new Hilo.Graphics({width: W, height: H});
        verso.beginLinearGradientFill(0, 0, W, H, [c.cor, c.escura], [0, 1]).drawRoundRect(0, 0, W, H, 14).endFill();
        verso.lineStyle(2, "rgba(255,255,255,.5)").drawRoundRect(8, 8, W - 16, H - 16, 10).endFill();
        carta.addChild(verso); carta.addChild(frente);
        carta.dados = {k: b.k, verso: verso, frente: frente, aberta: false, feita: false};
        carta.on(Hilo.event.POINTER_START, function () { virar(carta); });
        stage.addChild(carta); cartas.push(carta);
      });
      function girar(carta, aberta, pronto) {
        Hilo.Tween.to(carta, {scaleX: 0}, {duration: 140, onComplete: function () {
          carta.dados.verso.visible = !aberta; carta.dados.frente.visible = aberta;
          Hilo.Tween.to(carta, {scaleX: 1}, {duration: 140, onComplete: pronto});
        }});
      }
      function virar(carta) {
        var d = carta.dados;
        if (!token || d.aberta || d.feita || abertas.length >= 2) return;
        d.aberta = true; abertas.push(carta);
        girar(carta, true, function () {
          if (abertas.length < 2) return;
          var a = abertas[0], b = abertas[1];
          if (a.dados.k === b.dados.k) {
            a.dados.feita = b.dados.feita = true; abertas = []; pares++;
            if (pares === imgs.length) pedir(box, dados({acao: "concluir", token: token})).then(mostra).catch(falha);
          } else setTimeout(function () {
            a.dados.aberta = b.dados.aberta = false; abertas = [];
            girar(a, false); girar(b, false);
          }, 650);
        });
      }
      botao.onclick = function () {
        if (ocupado) return; ocupado = true; botao.disabled = true; erro.hidden = true;
        pedir(box, dados({acao: "jogar"})).then(function (r) { token = r.token; botao.textContent = "Ache os " + imgs.length + " pares!"; }).catch(falha);
      };
    }
  }

  function todos() {
    var els = document.querySelectorAll(".nep-jogo[data-jogo]:not([data-pronto])");
    if (!els.length) return;
    if (!document.getElementById("nep-jogos-css")) { var st = el("style"); st.id = "nep-jogos-css"; st.textContent = CSS; document.head.appendChild(st); }
    carregar(function () { Array.prototype.forEach.call(els, montar); });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", todos); else todos();
})();
