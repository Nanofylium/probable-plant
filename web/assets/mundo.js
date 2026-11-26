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
    // v: ponto da foto; w: ponto "por trás" (o fundo puxado das beiradas para
    // dentro do recorte); mk: máscara dos objetos da frente. A camada da frente
    // some fora da máscara; a de trás mostra o fundo só onde há recorte.
    gl.attachShader(prog, sh(gl.VERTEX_SHADER, "attribute vec3 p;attribute vec2 uv;attribute vec2 uv2;uniform mat4 m;varying vec2 v;varying vec2 w;void main(){v=uv;w=uv2;gl_Position=m*vec4(p,1.0);}"));
    gl.attachShader(prog, sh(gl.FRAGMENT_SHADER, "precision mediump float;uniform sampler2D t;uniform sampler2D mk;uniform float frente;varying vec2 v;varying vec2 w;void main(){float a=texture2D(mk,v).r;if(frente>0.5){float al=smoothstep(0.25,0.75,a);if(al<0.02)discard;gl_FragColor=vec4(texture2D(t,v).rgb,al);}else{gl_FragColor=mix(texture2D(t,v),texture2D(t,w),smoothstep(0.2,0.6,a));}}"));
    gl.linkProgram(prog);
    gl.useProgram(prog);
    var locP = gl.getAttribLocation(prog, "p"), locUV = gl.getAttribLocation(prog, "uv"), locM = gl.getUniformLocation(prog, "m");
    var locUV2 = gl.getAttribLocation(prog, "uv2"), locFrente = gl.getUniformLocation(prog, "frente");
    gl.uniform1i(gl.getUniformLocation(prog, "t"), 0); gl.uniform1i(gl.getUniformLocation(prog, "mk"), 1);

    // câmera: a foto foi tirada com ~60° na horizontal
    var hfov = 60 * Math.PI / 180, tanX = Math.tan(hfov / 2), tanY = tanX / asp;
    var PERTO = 1, LONGE = 9, ANDAR_MAX = .42, GIRO_MAX = .2, INCL_MAX = .08;
    var fundo = [0.1, 0.1, 0.1];

    // duas camadas (como num cenário de teatro). Os objetos que se destacam
    // do fundo (uma árvore, uma pessoa, um poste) viram um recorte inteiro, com
    // contorno fino; o fundo fica atrás, e no lugar do recorte ele é completado
    // com o que está em volta. Assim, ao andar, o objeto se move por inteiro,
    // sem rasgar em pedaços.
    var camadas = [];
    function caixa(src, w, h, rx, ry, modo) { // modo 0: média, 1: mínimo, 2: máximo (separável)
      var tmp = new Float32Array(w * h), out = new Float32Array(w * h), x, y, q, acc, c, v;
      for (y = 0; y < h; y++) for (x = 0; x < w; x++) {
        acc = modo === 1 ? 9 : modo === 2 ? -9 : 0; c = 0;
        for (q = Math.max(0, x - rx); q <= Math.min(w - 1, x + rx); q++) { v = src[y * w + q]; acc = modo === 1 ? Math.min(acc, v) : modo === 2 ? Math.max(acc, v) : acc + v; c++; }
        tmp[y * w + x] = modo ? acc : acc / c;
      }
      for (y = 0; y < h; y++) for (x = 0; x < w; x++) {
        acc = modo === 1 ? 9 : modo === 2 ? -9 : 0; c = 0;
        for (q = Math.max(0, y - ry); q <= Math.min(h - 1, y + ry); q++) { v = tmp[q * w + x]; acc = modo === 1 ? Math.min(acc, v) : modo === 2 ? Math.max(acc, v) : acc + v; c++; }
        out[y * w + x] = modo ? acc : acc / c;
      }
      return out;
    }
    // recorte dos objetos da frente, na resolução da profundidade
    function recorte(Dp, dw, dh) {
      var M = dw * dh, i;
      // fundo: o mais longe num trecho largo da mesma linha (objetos em pé se
      // destacam dos lados; o chão, que muda de cima para baixo, não)
      var B = caixa(caixa(Dp, dw, dh, Math.round(dw * .15), 0, 1), dw, dh, Math.round(dw * .03), Math.max(1, Math.round(dh * .01)), 0);
      var mk = new Float32Array(M);
      for (i = 0; i < M; i++) mk[i] = Dp[i] - B[i] > .1 ? 1 : 0;
      // fecha os buraquinhos (galhos finos continuam presos ao objeto)
      var r2 = Math.max(1, Math.round(dw * .006));
      mk = caixa(caixa(mk, dw, dh, r2, r2, 2), dw, dh, r2, r2, 1);
      // pedaços pequenos demais não valem um recorte
      var visto = new Uint8Array(M), pilha = [], comp = [], minimo = M * .004;
      for (i = 0; i < M; i++) {
        if (mk[i] < .5 || visto[i]) continue;
        pilha.length = 0; comp.length = 0; pilha.push(i); visto[i] = 1;
        while (pilha.length) {
          var k = pilha.pop(), x = k % dw, y = (k - x) / dw;
          comp.push(k);
          if (x > 0 && mk[k - 1] > .5 && !visto[k - 1]) { visto[k - 1] = 1; pilha.push(k - 1); }
          if (x < dw - 1 && mk[k + 1] > .5 && !visto[k + 1]) { visto[k + 1] = 1; pilha.push(k + 1); }
          if (y > 0 && mk[k - dw] > .5 && !visto[k - dw]) { visto[k - dw] = 1; pilha.push(k - dw); }
          if (y < dh - 1 && mk[k + dw] > .5 && !visto[k + dw]) { visto[k + dw] = 1; pilha.push(k + dw); }
        }
        if (comp.length < minimo) comp.forEach(function (k2) { mk[k2] = 0; });
      }
      // vãos fechados por dentro do objeto (céu entre os galhos) andam junto com ele
      var fora = new Uint8Array(M);
      pilha.length = 0;
      for (i = 0; i < M; i++) {
        var bx = i % dw, by = (i - bx) / dw;
        if ((bx === 0 || by === 0 || bx === dw - 1 || by === dh - 1) && mk[i] < .5) { fora[i] = 1; pilha.push(i); }
      }
      while (pilha.length) {
        var k4 = pilha.pop(), x4 = k4 % dw, y4 = (k4 - x4) / dw;
        [x4 > 0 ? k4 - 1 : -1, x4 < dw - 1 ? k4 + 1 : -1, y4 > 0 ? k4 - dw : -1, y4 < dh - 1 ? k4 + dw : -1].forEach(function (q) {
          if (q >= 0 && !fora[q] && mk[q] < .5) { fora[q] = 1; pilha.push(q); }
        });
      }
      for (i = 0; i < M; i++) if (!fora[i]) mk[i] = 1;
      return {mk: mk, B: B};
    }
    function malha(dados, dw, dh) {
      var GX = toque ? 150 : 220, GY = Math.max(8, Math.round(GX / asp)), EXTRA = .5;
      var NX = GX + 1, NY = GY + 1, N = NX * NY;
      if (N > 65535 && !gl.getExtension("OES_element_index_uint")) return false;
      function espelha(u) { return u < 0 ? -u : u > 1 ? 2 - u : u; }
      var Dp = new Float32Array(dw * dh), i, j, k;
      for (i = 0; i < dw * dh; i++) Dp[i] = (dados[i * 4] * 256 + dados[i * 4 + 1]) / 65535; // 16 bits em R e G
      var rc = recorte(Dp, dw, dh), mk = rc.mk;
      // a máscara vai para a placa de vídeo (contorno fino, sem degraus da malha)
      // contorno levemente esfumado, para a emenda com o fundo não aparecer
      var r3 = Math.max(1, Math.round(dw * .003)), suave = caixa(mk, dw, dh, r3, r3, 0), mb = new Uint8Array(dw * dh);
      for (i = 0; i < dw * dh; i++) mb[i] = Math.round(suave[i] * 255);
      gl.activeTexture(gl.TEXTURE1);
      var tm = gl.createTexture();
      gl.bindTexture(gl.TEXTURE_2D, tm);
      gl.pixelStorei(gl.UNPACK_ALIGNMENT, 1);
      gl.texImage2D(gl.TEXTURE_2D, 0, gl.LUMINANCE, dw, dh, 0, gl.LUMINANCE, gl.UNSIGNED_BYTE, mb);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
      gl.activeTexture(gl.TEXTURE0);
      function bil(A, u, v) {
        u = Math.min(1, Math.max(0, u)) * (dw - 1); v = Math.min(1, Math.max(0, v)) * (dh - 1);
        var x0 = Math.floor(u), y0 = Math.floor(v), x1 = Math.min(x0 + 1, dw - 1), y1 = Math.min(y0 + 1, dh - 1), fx = u - x0, fy = v - y0;
        return (A[y0 * dw + x0] * (1 - fx) + A[y0 * dw + x1] * fx) * (1 - fy) + (A[y1 * dw + x0] * (1 - fx) + A[y1 * dw + x1] * fx) * fy;
      }
      // em cada vértice: profundidade do fundo e, perto de um recorte, a média
      // da profundidade só dos pontos do recorte (assim o objeto fica inteiriço)
      var U = new Float32Array(N), V = new Float32Array(N), Df = new Float32Array(N), Dr = new Float32Array(N), tem = new Uint8Array(N);
      var rg = Math.max(1, Math.ceil(dw * (1 + 2 * EXTRA) / GX));
      // dentro do recorte, os vãos entre as folhas pegam a profundidade das
      // folhas em volta, e a média é larga: o objeto fica liso, sem ondular
      var Dc = caixa(Dp, dw, dh, Math.max(2, Math.round(dw * .012)), Math.max(2, Math.round(dw * .012)), 2), ra = rg * 3;
      for (j = 0; j < NY; j++) for (i = 0; i < NX; i++) {
        k = j * NX + i;
        // a malha passa da borda da foto, espelhando a imagem, para não aparecer o vazio ao virar
        var u = -EXTRA + (1 + 2 * EXTRA) * i / GX, v = -EXTRA + (1 + 2 * EXTRA) * j / GY, eu = espelha(u), ev = espelha(v);
        U[k] = u; V[k] = v; Df[k] = bil(rc.B, eu, ev);
        var px = Math.round(eu * (dw - 1)), py = Math.round(ev * (dh - 1)), s = 0, c = 0;
        for (var y = Math.max(0, py - ra); y <= Math.min(dh - 1, py + ra); y += 2) for (var x = Math.max(0, px - ra); x <= Math.min(dw - 1, px + ra); x += 2) {
          var q = y * dw + x; if (mk[q] > .5) { s += Dc[q]; c++; }
        }
        Dr[k] = c ? s / c : Dc[py * dw + px];
        for (y = Math.max(0, py - rg); y <= Math.min(dh - 1, py + rg) && !tem[k]; y++) for (x = Math.max(0, px - rg); x <= Math.min(dw - 1, px + rg); x++) if (mk[y * dw + x] > .5) { tem[k] = 1; break; }
      }
      // o fundo atrás do recorte: cada vértice coberto mostra o que está do
      // outro lado da beirada mais próxima, como num espelho (a mata continua
      // mata, o céu continua céu), em vez de esticar a beirada em listras
      var W2 = new Float32Array(N * 2), fonte = new Int32Array(N).fill(-1), fila = [], cab = 0;
      for (k = 0; k < N; k++) {
        W2[k * 2] = espelha(U[k]); W2[k * 2 + 1] = espelha(V[k]);
        if (!tem[k]) { fonte[k] = k; fila.push(k); }
      }
      while (cab < fila.length) {
        var k5 = fila[cab++], x5 = k5 % NX, y5 = (k5 - x5) / NX;
        for (var dy = -1; dy <= 1; dy++) for (var dx = -1; dx <= 1; dx++) {
          var xx = x5 + dx, yy = y5 + dy, q5 = yy * NX + xx;
          if ((dx || dy) && xx >= 0 && yy >= 0 && xx < NX && yy < NY && fonte[q5] < 0) { fonte[q5] = fonte[k5]; fila.push(q5); }
        }
      }
      for (k = 0; k < N; k++) if (tem[k] && fonte[k] >= 0) {
        var f5 = fonte[k];
        W2[k * 2] = espelha(espelha(U[f5]) * 2 - espelha(U[k])); W2[k * 2 + 1] = espelha(espelha(V[f5]) * 2 - espelha(V[k]));
      }
      // alisa o espelho (a beirada da malha é em degraus)
      for (var passada = 0; passada < 4; passada++) {
        var W3 = W2.slice();
        for (k = 0; k < N; k++) if (tem[k]) {
          var x6 = k % NX, y6 = (k - x6) / NX, su = 0, sv = 0, n6 = 0;
          for (var ey = -1; ey <= 1; ey++) for (var ex = -1; ex <= 1; ex++) {
            var x7 = x6 + ex, y7 = y6 + ey;
            if (x7 >= 0 && y7 >= 0 && x7 < NX && y7 < NY) { su += W3[(y7 * NX + x7) * 2]; sv += W3[(y7 * NX + x7) * 2 + 1]; n6++; }
          }
          W2[k * 2] = su / n6; W2[k * 2 + 1] = sv / n6;
        }
      }
      function camada(disp, frente) {
        var pos = new Float32Array(N * 3), uvs = new Float32Array(N * 2), Z = new Float32Array(N);
        for (var k2 = 0; k2 < N; k2++) {
          var z = 1 / (disp[k2] * (1 / PERTO - 1 / LONGE) + 1 / LONGE);
          Z[k2] = z;
          pos[k2 * 3] = (U[k2] * 2 - 1) * tanX * z; pos[k2 * 3 + 1] = (1 - V[k2] * 2) * tanY * z; pos[k2 * 3 + 2] = -z;
          uvs[k2 * 2] = espelha(U[k2]); uvs[k2 * 2 + 1] = espelha(V[k2]);
        }
        var idx = new (N > 65535 ? Uint32Array : Uint16Array)(GX * GY * 6), n = 0;
        function tri(a, b, c) {
          // o recorte vai inteiro: sem cortes por dentro, que viravam pedaços soltos
          if (frente && (!tem[a] || !tem[b] || !tem[c])) return;
          idx[n++] = a; idx[n++] = b; idx[n++] = c;
        }
        for (var y = 0; y < GY; y++) for (var x = 0; x < GX; x++) {
          var a = y * NX + x, b = a + 1, c = a + NX, d2 = c + 1;
          tri(a, c, b); tri(b, c, d2);
        }
        function buf(alvoB, arr) { var bf = gl.createBuffer(); gl.bindBuffer(alvoB, bf); gl.bufferData(alvoB, arr, gl.STATIC_DRAW); return bf; }
        return {p: buf(gl.ARRAY_BUFFER, pos), uv: buf(gl.ARRAY_BUFFER, uvs), uv2: buf(gl.ARRAY_BUFFER, frente ? uvs : W2), i: buf(gl.ELEMENT_ARRAY_BUFFER, idx.subarray(0, n)), n: n, frente: frente};
      }
      camadas = [camada(Df, false)];
      var cf = camada(Dr, true);
      if (cf.n) camadas.push(cf);
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
        gl.bindBuffer(gl.ARRAY_BUFFER, c.uv2); gl.enableVertexAttribArray(locUV2); gl.vertexAttribPointer(locUV2, 2, gl.FLOAT, false, 0, 0);
        gl.uniform1f(locFrente, c.frente ? 1 : 0);
        if (c.frente) { gl.enable(gl.BLEND); gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA); } else gl.disable(gl.BLEND);
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
