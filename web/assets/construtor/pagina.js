/* Nepenthes: construtor de páginas inteiras (GrapesJS, BSD-3-Clause).
   As "ilhas" são o código do Nepenthes da página (listas, formulários,
   regiões ao vivo): aparecem com a prévia de verdade, não se editam por
   dentro, podem ser movidas ou apagadas e voltam intactas ao salvar. */
(function () {
  var D = JSON.parse(document.getElementById("pv-dados").textContent);
  var blocks = window["gjs-blocks-basic"];
  if (blocks && blocks.default) blocks = blocks.default;
  var estado = document.getElementById("pv-estado");
  var sujo = false;

  function ilhas(ed) {
    ed.DomComponents.addType("nep-ilha", {
      isComponent: function (el) {
        if (el && el.getAttribute && (el.hasAttribute("data-nep-ilha") || el.hasAttribute("data-nep-novo"))) return { type: "nep-ilha", tagName: el.tagName.toLowerCase() };
      },
      model: {
        defaults: { editable: false, droppable: false, stylable: false, copyable: true, name: "Bloco do Nepenthes", traits: [] },
        init: function () {
          var a = this.getAttributes();
          var k = a["data-nep-ilha"] != null ? a["data-nep-ilha"] : a["data-nep-novo"];
          var p = D.previas[k];
          if (p == null || !String(p).trim()) p = '<span class="nep-ilha-vazia">Código do Nepenthes (aparece no site quando tiver conteúdo)</span>';
          this.components("");
          this.set("content", p);
        },
        toHTML: function () {
          var a = this.getAttributes(), t = this.get("tagName") || "div";
          if (a["data-nep-ilha"] != null) return "<" + t + ' data-nep-ilha="' + a["data-nep-ilha"] + '"></' + t + ">";
          return "<" + t + ' data-nep-novo="' + a["data-nep-novo"] + '"></' + t + ">";
        }
      }
    });
  }

  var cor = "#2f7d5b";
  var secoes = [
    { id: "capa", label: "Capa", html: '<section style="padding:88px 24px;text-align:center;background:#f3efe6"><h1 style="font-size:2.6rem;line-height:1.15;margin:0 0 14px">Um título forte para esta página</h1><p style="font-size:1.15rem;max-width:640px;margin:0 auto 26px;color:#4b554f">Uma frase curta que explica o que você faz e para quem.</p><a href="/contato" style="display:inline-block;padding:13px 24px;border-radius:999px;background:' + cor + ';color:#fff;text-decoration:none;font-weight:700">Fale com a gente</a></section>' },
    { id: "servicos", label: "3 serviços", html: '<section style="padding:64px 24px"><h2 style="text-align:center;font-size:2rem;margin:0 0 32px">O que fazemos</h2><div style="display:flex;flex-wrap:wrap;gap:20px;max-width:1100px;margin:0 auto">' + [1, 2, 3].map(function (i) { return '<div style="flex:1 1 260px;padding:26px;border:1px solid #e2e1da;border-radius:16px;background:#fff"><h3 style="margin:0 0 8px">Serviço ' + i + '</h3><p style="margin:0;color:#5f6a64">Conte em duas linhas o que a pessoa recebe e por que vale a pena.</p></div>'; }).join("") + "</div></section>" },
    { id: "lado", label: "Imagem e texto", html: '<section style="padding:64px 24px"><div style="display:flex;flex-wrap:wrap;gap:36px;align-items:center;max-width:1100px;margin:0 auto"><img src="' + (D.base || "") + '/_nep/molde/img/mesa.jpg" alt="" style="flex:1 1 320px;max-width:100%;border-radius:18px"><div style="flex:1 1 320px"><h2 style="font-size:1.9rem;margin:0 0 12px">Sobre nós</h2><p style="color:#4b554f;margin:0 0 18px">Conte a sua história: desde quando, o que faz de diferente e o que o cliente pode esperar.</p><a href="/contato" style="color:' + cor + ';font-weight:700">Agende uma conversa →</a></div></div></section>' },
    { id: "depoimento", label: "Depoimento", html: '<section style="padding:64px 24px;background:#f7f6f1"><blockquote style="max-width:760px;margin:0 auto;text-align:center;font-size:1.35rem;line-height:1.5">“Atendimento atencioso e resultado acima do esperado. Recomendo de olhos fechados.”<footer style="margin-top:14px;font-size:1rem;color:#5f6a64">Maria, cliente desde 2021</footer></blockquote></section>' },
    { id: "chamada", label: "Chamada", html: '<section style="padding:56px 24px;background:' + cor + ';color:#fff;text-align:center"><h2 style="font-size:2rem;margin:0 0 10px">Pronto para começar?</h2><p style="margin:0 0 22px;opacity:.9">Responda em poucos minutos pelo WhatsApp ou pelo formulário.</p><a href="/contato" style="display:inline-block;padding:13px 24px;border-radius:999px;background:#fff;color:' + cor + ';text-decoration:none;font-weight:700">Quero falar com vocês</a></section>' }
  ];

  var ed = grapesjs.init({
    container: "#pv-editor",
    height: "100%",
    width: "auto",
    fromElement: false,
    components: D.html,
    style: "",
    storageManager: false,
    telemetry: false,
    protectedCss: "",
    cssIcons: (D.base || "") + "/_nep/construtor/font-awesome.min.css",
    i18n: window.gjsPt ? { locale: "pt", detectLocale: false, messages: { pt: window.gjsPt } } : {},
    plugins: [ilhas].concat(blocks ? [blocks] : []),
    deviceManager: {
      devices: [
        { id: "desktop", name: "Computador", width: "" },
        { id: "tablet", name: "Tablet", width: "768px", widthMedia: "992px" },
        { id: "mobile", name: "Celular", width: "375px", widthMedia: "480px" }
      ]
    },
    assetManager: { upload: D.upload, uploadName: "arquivos", params: { _nep_csrf: D.csrf }, multiUpload: true, autoAdd: true, assets: D.midia || [] }
  });

  window.nepEditor = ed;
  ed.on("load", function () {
    var bm = ed.Blocks;
    var pt = { column1: "1 coluna", column2: "2 colunas", column3: "3 colunas", "column3-7": "2 colunas 3/7", text: "Texto", link: "Link", image: "Imagem", video: "Vídeo", map: "Mapa" };
    // seções e blocos do Nepenthes primeiro; os básicos do GrapesJS depois
    var basicos = Object.keys(pt).map(function (id) { var b = bm.get(id); if (!b) return null; var o = b.toJSON(); bm.remove(id); o.id = id; o.label = pt[id]; o.category = "Básico"; return o; }).filter(Boolean);
    secoes.forEach(function (s) { bm.add("sec-" + s.id, { label: s.label, category: "Seções", content: s.html, media: '<svg viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="14" rx="2" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M7 10h10M7 14h6" stroke="currentColor" stroke-width="1.5"/></svg>' }); });
    (D.blocos || []).forEach(function (b) {
      bm.add("nep-" + b.id, { label: b.nome, category: "Nepenthes", content: { type: "nep-ilha", tagName: "div", attributes: { "data-nep-novo": b.id } }, media: '<svg viewBox="0 0 24 24"><path d="M12 3c4 5 7 9 7 13a7 7 0 0 1-14 0c0-4 3-8 7-13z" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>' });
    });
    basicos.forEach(function (o) { bm.add(o.id, o); });
    bm.render();
    var ob = ed.Panels.getButton("views", "open-blocks");
    if (ob) ob.set("active", true);
    var doc = ed.Canvas.getDocument();
    var primeiro = doc.head.firstChild;
    (D.estilos || []).forEach(function (e) { // os estilos do site, na mesma ordem
      var el;
      if (e.href) { el = doc.createElement("link"); el.rel = "stylesheet"; el.href = e.href; }
      else { el = doc.createElement("style"); el.textContent = e.css; }
      doc.head.insertBefore(el, primeiro);
    });
    var st = doc.createElement("style");
    st.textContent = "[data-nep-ilha],[data-nep-novo]{outline:1px dashed rgba(47,125,91,.55);outline-offset:2px}div[data-nep-ilha],div[data-nep-novo]{display:block}.nep-ilha-vazia{display:inline-block;font:12px system-ui;color:#2f7d5b;background:#e8f2ec;border-radius:6px;padding:4px 8px}";
    doc.head.appendChild(st);
    sujo = false;
    estado.textContent = "Arraste blocos da direita. Blocos com borda tracejada são código do Nepenthes.";
  });
  ed.on("update", function () { sujo = true; estado.className = ""; estado.textContent = "Alterações não salvas"; });
  window.addEventListener("beforeunload", function (e) { if (sujo) { e.preventDefault(); e.returnValue = ""; } });

  var btn = document.getElementById("pv-salvar");
  function salvar() {
    btn.disabled = true;
    estado.className = ""; estado.textContent = "Salvando…";
    var h = ed.getHtml().replace(/^<body[^>]*>/i, "").replace(/<\/body>\s*$/i, "");
    var c = ed.getCss({ avoidProtected: true }) || "";
    fetch(D.salvar, { method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ _nep_csrf: D.csrf, r: D.rota, html: h, css: c }) })
      .then(function (r) { return r.json().then(function (d) { if (!r.ok) throw new Error(d.erro || "erro " + r.status); return d; }); })
      .then(function () { sujo = false; estado.textContent = "Salvo. Já está no site."; })
      .catch(function (e) { estado.className = "erro"; estado.textContent = e.message; })
      .then(function () { btn.disabled = false; });
  }
  btn.onclick = salvar;
  document.addEventListener("keydown", function (e) { if ((e.ctrlKey || e.metaKey) && (e.key === "s" || e.key === "S")) { e.preventDefault(); salvar(); } });
})();
