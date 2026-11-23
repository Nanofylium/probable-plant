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

  // ---------- passeio 3D: foto + profundidade, desenhado com WebGL ----------
  function montar3d(fig, g) {
    var img0 = fig.querySelector("img"), legenda = fig.querySelector("figcaption");
    var palco = el("div", "nep-mundo-palco"), cv = el("canvas");
    var gl = cv.getContext("webgl", {antialias: true, alpha: false}) || cv.getContext("experimental-webgl");
    if (!gl) return; // sem WebGL: fica a foto
    var W = g.largura || 1600, H = g.altura || 900, asp = W / H;
    palco.style.aspectRatio = W + " / " + H;
    palco.style.maxHeight = "70vh";
    palco.style.margin = "0 auto";
    cv.style.cssText = "width:100%;height:100%;max-height:none;object-fit:fill;cursor:grab;touch-action:" + (g.rolar ? "pan-y" : "none");
    palco.tabIndex = 0;
    palco.setAttribute("role", "application");
    palco.setAttribute("aria-label", "Passeio em 3D: segure a seta para cima para andar, para baixo para voltar, e use as setas do lado ou arraste para olhar em volta");
    var toque = window.matchMedia && matchMedia("(pointer: coarse)").matches;
    var dica = el("div", "nep-mundo-dica", g.rolar ? (toque ? "Deslize a página para andar" : "Role a página para andar") : toque ? "Segure para andar · arraste para olhar" : "Segure ↑ para andar · arraste para olhar");
    var barra = el("div", "nep-mundo-barra"), carga = el("div", "nep-mundo-carga", "Montando o lugar em 3D…");
    var selo = el("div", "nep-mundo-selo", g.fonte === "estimado" ? "3D estimado" : "3D · profundidade por IA");
    palco.appendChild(cv); palco.appendChild(barra); palco.appendChild(selo); palco.appendChild(carga); palco.appendChild(dica);
    var ctl = el("div", "nep-mundo-ctl");
    var olharE = el("button", "", "← Olhar"), voltar = el("button", "", "↓ Voltar"), andar = el("button", "nep-mundo-andar", "Andar ↑"), olharD = el("button", "", "Olhar →");
    [olharE, voltar, andar, olharD].forEach(function (b) { b.type = "button"; ctl.appendChild(b); });
    var alvo = fig;
    if (g.rolar) {
      var rolo = el("div", "nep-mundo-rolo"), fixo = el("div", "nep-mundo-fixo");
      rolo.style.height = "240vh";
      fixo.appendChild(palco); fixo.appendChild(ctl); rolo.appendChild(fixo);
      fig.insertBefore(rolo, img0);
      alvo = rolo;
    } else { fig.insertBefore(palco, img0); fig.insertBefore(ctl, img0); }
    img0.style.display = "none";

    // shaders
    function sh(tipo, src) { var s = gl.createShader(tipo); gl.shaderSource(s, src); gl.compileShader(s); return s; }
    var prog = gl.createProgram();
    gl.attachShader(prog, sh(gl.VERTEX_SHADER, "attribute vec3 p;attribute vec2 uv;uniform mat4 m;varying vec2 v;void main(){v=uv;gl_Position=m*vec4(p,1.0);}"));
    gl.attachShader(prog, sh(gl.FRAGMENT_SHADER, "precision mediump float;uniform sampler2D t;varying vec2 v;void main(){gl_FragColor=texture2D(t,v);}"));
    gl.linkProgram(prog);
    gl.useProgram(prog);
    var locP = gl.getAttribLocation(prog, "p"), locUV = gl.getAttribLocation(prog, "uv"), locM = gl.getUniformLocation(prog, "m");

    // câmera: a foto foi tirada com ~60° na horizontal
    var hfov = 60 * Math.PI / 180, tanX = Math.tan(hfov / 2), tanY = tanX / asp;
    var PERTO = 1, LONGE = 9, ANDAR_MAX = .42, GIRO_MAX = .2, INCL_MAX = .08;
    var fundo = [0.1, 0.1, 0.1];

    // duas camadas (como num cenário de teatro): a de trás tem os objetos
    // próximos "empurrados" para o fundo, e a da frente perde os triângulos
    // esticados nas bordas dos objetos; assim, ao andar, aparece o fundo atrás
    // da árvore em vez de um borrão
    var camadas = [];
    function malha(dados, dw, dh) {
      var GX = toque ? 150 : 220, GY = Math.max(8, Math.round(GX / asp)), EXTRA = .5;
      var NX = GX + 1, NY = GY + 1, N = NX * NY;
      if (N > 65535 && !gl.getExtension("OES_element_index_uint")) return false;
      function espelha(u) { return u < 0 ? -u : u > 1 ? 2 - u : u; }
      function prof(u, v) { // disparidade 0..1 (bilinear), 16 bits em R e G
        u = Math.min(1, Math.max(0, u)) * (dw - 1); v = Math.min(1, Math.max(0, v)) * (dh - 1);
        var x0 = Math.floor(u), y0 = Math.floor(v), x1 = Math.min(x0 + 1, dw - 1), y1 = Math.min(y0 + 1, dh - 1), fx = u - x0, fy = v - y0;
        function d(x, y) { var o = (y * dw + x) * 4; return (dados[o] * 256 + dados[o + 1]) / 65535; }
        return (d(x0, y0) * (1 - fx) + d(x1, y0) * fx) * (1 - fy) + (d(x0, y1) * (1 - fx) + d(x1, y1) * fx) * fy;
      }
      var U = new Float32Array(N), V = new Float32Array(N), D = new Float32Array(N), k;
      for (var j = 0; j < NY; j++) for (var i = 0; i < NX; i++) {
        k = j * NX + i;
        // a malha passa da borda da foto, espelhando a imagem, para não aparecer o vazio ao virar
        var u = -EXTRA + (1 + 2 * EXTRA) * i / GX, v = -EXTRA + (1 + 2 * EXTRA) * j / GY;
        U[k] = u; V[k] = v; D[k] = prof(espelha(u), espelha(v));
      }
      // fundo: erosão (mínimo) e depois média, num raio de ~3% da largura
      function filtro(src, r, f) {
        var tmp = new Float32Array(N), out = new Float32Array(N), x, y, q, acc, c;
        for (y = 0; y < NY; y++) for (x = 0; x < NX; x++) {
          acc = f ? 9 : 0; c = 0;
          for (q = Math.max(0, x - r); q <= Math.min(NX - 1, x + r); q++) { var w = src[y * NX + q]; acc = f ? Math.min(acc, w) : acc + w; c++; }
          tmp[y * NX + x] = f ? acc : acc / c;
        }
        for (y = 0; y < NY; y++) for (x = 0; x < NX; x++) {
          acc = f ? 9 : 0; c = 0;
          for (q = Math.max(0, y - r); q <= Math.min(NY - 1, y + r); q++) { var w2 = tmp[q * NX + x]; acc = f ? Math.min(acc, w2) : acc + w2; c++; }
          out[y * NX + x] = f ? acc : acc / c;
        }
        return out;
      }
      var r = Math.max(2, Math.round(GX * .03));
      var Dfundo = filtro(filtro(D, r, true), r, false);
      function camada(disp, cortar) {
        var pos = new Float32Array(N * 3), uvs = new Float32Array(N * 2), Z = new Float32Array(N);
        for (var k2 = 0; k2 < N; k2++) {
          var z = 1 / (disp[k2] * (1 / PERTO - 1 / LONGE) + 1 / LONGE);
          Z[k2] = z;
          pos[k2 * 3] = (U[k2] * 2 - 1) * tanX * z; pos[k2 * 3 + 1] = (1 - V[k2] * 2) * tanY * z; pos[k2 * 3 + 2] = -z;
          uvs[k2 * 2] = espelha(U[k2]); uvs[k2 * 2 + 1] = espelha(V[k2]);
        }
        var idx = new (N > 65535 ? Uint32Array : Uint16Array)(GX * GY * 6), n = 0;
        function tri(a, b, c) {
          if (cortar) { var mx = Math.max(Z[a], Z[b], Z[c]), mn = Math.min(Z[a], Z[b], Z[c]); if (mx / mn > 1.1) return; }
          idx[n++] = a; idx[n++] = b; idx[n++] = c;
        }
        for (var y = 0; y < GY; y++) for (var x = 0; x < GX; x++) {
          var a = y * NX + x, b = a + 1, c = a + NX, d2 = c + 1;
          tri(a, c, b); tri(b, c, d2);
        }
        function buf(alvoB, arr) { var bf = gl.createBuffer(); gl.bindBuffer(alvoB, bf); gl.bufferData(alvoB, arr, gl.STATIC_DRAW); return bf; }
        return {p: buf(gl.ARRAY_BUFFER, pos), uv: buf(gl.ARRAY_BUFFER, uvs), i: buf(gl.ELEMENT_ARRAY_BUFFER, idx.subarray(0, n)), n: n};
      }
      camadas = [camada(Dfundo, false), camada(D, true)];
      return N > 65535 ? gl.UNSIGNED_INT : gl.UNSIGNED_SHORT;
    }

    function mul(a, b) { var o = new Float32Array(16); for (var c = 0; c < 4; c++) for (var r = 0; r < 4; r++) { var s = 0; for (var k = 0; k < 4; k++) s += a[k * 4 + r] * b[c * 4 + k]; o[c * 4 + r] = s; } return o; }
    function persp(fy, a, n, f) { var t = 1 / Math.tan(fy / 2); return new Float32Array([t / a, 0, 0, 0, 0, t, 0, 0, 0, 0, (f + n) / (n - f), -1, 0, 0, 2 * f * n / (n - f), 0]); }
    function rotY(r) { var c = Math.cos(r), s = Math.sin(r); return new Float32Array([c, 0, -s, 0, 0, 1, 0, 0, s, 0, c, 0, 0, 0, 0, 1]); }
    function rotX(r) { var c = Math.cos(r), s = Math.sin(r); return new Float32Array([1, 0, 0, 0, 0, c, s, 0, 0, -s, c, 0, 0, 0, 0, 1]); }
    function trans(x, y, z) { return new Float32Array([1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, x, y, z, 1]); }

    var tipoIdx = null, t = 0, giro = 0, incl = 0, vel = 0, velGiro = 0, tempo = 0, ultimo = 0, rodando = false, dicaFora = false;
    function tamanho() {
      var dpr = Math.min(window.devicePixelRatio || 1, 2), w = Math.round(cv.clientWidth * dpr), h = Math.round(cv.clientHeight * dpr);
      if (w && h && (cv.width !== w || cv.height !== h)) { cv.width = w; cv.height = h; }
    }
    function desenhar() {
      if (!tipoIdx) return;
      tamanho();
      gl.viewport(0, 0, cv.width, cv.height);
      gl.clearColor(fundo[0], fundo[1], fundo[2], 1);
      gl.clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT);
      // ao olhar para o lado, a câmera fecha o ângulo na mesma medida: a vista
      // continua dentro da foto (fora dela não há o que mostrar)
      var tx = Math.tan(Math.max(.12, hfov / 2 - Math.abs(giro))), ty = Math.tan(Math.max(.08, Math.atan(tanY) - Math.abs(incl)));
      var va = cv.width / cv.height, fy = 2 * Math.atan(Math.min(ty, tx / va));
      var balanco = Math.sin(tempo * .9) * .035, sobe = Math.sin(tempo * 1.7) * .006 * (vel ? 1 : 0);
      var vista = mul(rotX(-incl), mul(rotY(-giro), trans(-balanco, -sobe, t)));
      gl.uniformMatrix4fv(locM, false, mul(persp(fy, va, .05, 60), vista));
      camadas.forEach(function (c) {
        gl.bindBuffer(gl.ARRAY_BUFFER, c.p); gl.enableVertexAttribArray(locP); gl.vertexAttribPointer(locP, 3, gl.FLOAT, false, 0, 0);
        gl.bindBuffer(gl.ARRAY_BUFFER, c.uv); gl.enableVertexAttribArray(locUV); gl.vertexAttribPointer(locUV, 2, gl.FLOAT, false, 0, 0);
        gl.bindBuffer(gl.ELEMENT_ARRAY_BUFFER, c.i);
        gl.drawElements(gl.TRIANGLES, c.n, tipoIdx, 0);
      });
      barra.style.width = (t / ANDAR_MAX * 100).toFixed(1) + "%";
    }
    var reduzir = window.matchMedia && matchMedia("(prefers-reduced-motion: reduce)").matches;
    function passo(ts) {
      var dt = Math.min((ts - ultimo) / 1000, .1); ultimo = ts;
      if (!reduzir) tempo += dt;
      t = Math.max(0, Math.min(ANDAR_MAX, t + vel * .28 * dt));
      giro = Math.max(-GIRO_MAX, Math.min(GIRO_MAX, giro + velGiro * .5 * dt));
      desenhar();
      if (visivel) requestAnimationFrame(passo); else rodando = false;
    }
    var visivel = false;
    function anima() { if (!rodando && tipoIdx) { rodando = true; ultimo = performance.now(); requestAnimationFrame(passo); } }
    function some() { if (!dicaFora) { dicaFora = true; dica.style.opacity = 0; } }
    function mover(d) { vel = d; if (d) some(); }
    function virar(d) { velGiro = d; if (d) some(); }

    function segurar(b, f, d) {
      b.addEventListener("pointerdown", function (e) { e.preventDefault(); b.setPointerCapture && b.setPointerCapture(e.pointerId); f(d); });
      ["pointerup", "pointercancel", "lostpointercapture"].forEach(function (ev) { b.addEventListener(ev, function () { f(0); }); });
      b.addEventListener("keydown", function (e) { if (e.key === " " || e.key === "Enter") { e.preventDefault(); f(d); } });
      b.addEventListener("keyup", function () { f(0); });
    }
    segurar(andar, mover, 1); segurar(voltar, mover, -1); segurar(olharE, virar, 1); segurar(olharD, virar, -1);
    palco.addEventListener("keydown", function (e) {
      var k = e.key;
      if (/^(ArrowUp|w|W)$/.test(k)) { e.preventDefault(); mover(1); }
      else if (/^(ArrowDown|s|S)$/.test(k)) { e.preventDefault(); mover(-1); }
      else if (/^(ArrowLeft|a|A)$/.test(k)) { e.preventDefault(); virar(1); }
      else if (/^(ArrowRight|d|D)$/.test(k)) { e.preventDefault(); virar(-1); }
    });
    palco.addEventListener("keyup", function (e) {
      if (/^(Arrow(Up|Down)|[wWsS])$/.test(e.key)) mover(0);
      if (/^(Arrow(Left|Right)|[aAdD])$/.test(e.key)) virar(0);
    });
    palco.addEventListener("blur", function () { mover(0); virar(0); });
    // arrastar olha em volta; segurar parado (sem arrastar) anda
    var arr = null;
    cv.addEventListener("pointerdown", function (e) {
      arr = {x: e.clientX, y: e.clientY, g: giro, i: incl, mexeu: false, id: e.pointerId};
      cv.setPointerCapture && cv.setPointerCapture(e.pointerId);
      cv.style.cursor = "grabbing";
      if (!g.rolar) arr.timer = setTimeout(function () { if (arr && !arr.mexeu) mover(1); }, 220);
    });
    cv.addEventListener("pointermove", function (e) {
      if (!arr) return;
      var dx = e.clientX - arr.x, dy = e.clientY - arr.y;
      if (Math.abs(dx) + Math.abs(dy) > 6) { arr.mexeu = true; if (vel > 0) mover(0); }
      if (!arr.mexeu) return;
      some();
      giro = Math.max(-GIRO_MAX, Math.min(GIRO_MAX, arr.g + dx / cv.clientWidth * 1.2));
      incl = Math.max(-INCL_MAX, Math.min(INCL_MAX, arr.i + dy / cv.clientHeight * .5));
    });
    function solta() { if (arr) { clearTimeout(arr.timer); arr = null; mover(0); cv.style.cursor = "grab"; } }
    cv.addEventListener("pointerup", solta); cv.addEventListener("pointercancel", solta);

    if (g.rolar) {
      window.addEventListener("scroll", function () {
        if (vel) return;
        var r = alvo.getBoundingClientRect(), total = r.height - window.innerHeight * .7;
        var p = Math.max(0, Math.min(1, -r.top / Math.max(total, 1)));
        t = p * ANDAR_MAX;
        if (p > 0) some();
      }, {passive: true});
    }

    function iniciar() {
      var foto = new Image(), prof = new Image(), prontos = 0;
      foto.crossOrigin = prof.crossOrigin = "anonymous";
      function ok() {
        if (++prontos < 2) return;
        var c2 = document.createElement("canvas"); c2.width = prof.naturalWidth; c2.height = prof.naturalHeight;
        var x2 = c2.getContext("2d"); x2.drawImage(prof, 0, 0);
        var dados = x2.getImageData(0, 0, c2.width, c2.height).data;
        // cor de fundo: média da beirada da foto, escurecida
        var c3 = document.createElement("canvas"); c3.width = 8; c3.height = 8;
        var x3 = c3.getContext("2d"); x3.drawImage(foto, 0, 0, 8, 8);
        var px = x3.getImageData(0, 0, 8, 8).data, s = [0, 0, 0];
        for (var i = 0; i < px.length; i += 4) { s[0] += px[i]; s[1] += px[i + 1]; s[2] += px[i + 2]; }
        fundo = s.map(function (v) { return v / 64 / 255 * .6; });
        var tex = gl.createTexture();
        gl.bindTexture(gl.TEXTURE_2D, tex);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, foto);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
        gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
        gl.enable(gl.DEPTH_TEST);
        tipoIdx = malha(dados, c2.width, c2.height);
        if (!tipoIdx) { img0.style.display = ""; palco.remove(); ctl.remove(); return; }
        carga.style.opacity = 0;
        anima();
      }
      foto.onload = prof.onload = ok;
      foto.onerror = prof.onerror = function () { carga.textContent = "Não consegui abrir o passeio."; };
      foto.src = g.base + g.foto; prof.src = g.base + g.profundidade;
    }
    // só monta e anima quando o passeio está na tela
    if (window.IntersectionObserver) {
      var comecou = false;
      new IntersectionObserver(function (es) {
        visivel = es[0].isIntersecting;
        if (visivel && !comecou) { comecou = true; iniciar(); }
        if (visivel) anima();
      }, {rootMargin: "200px"}).observe(palco);
    } else { visivel = true; iniciar(); }
  }

  function montar(fig) {
    var g;
    try { g = JSON.parse(fig.getAttribute("data-mundo")); } catch (e) { return; }
    fig.setAttribute("data-pronto", "1");
    if (g.tipo === "3d") return montar3d(fig, g);
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
