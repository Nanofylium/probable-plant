/* Nepenthes: liga o construtor visual (GrapesJS) às caixas <textarea data-construtor>
   do painel. O conteúdo vai e volta como "<style>css</style>html"; o servidor
   limpa o HTML e isola o CSS na hora de mostrar no site. */
(function () {
  var me = document.currentScript;
  var base = me.getAttribute("data-base") || "";
  var csrf = me.getAttribute("data-csrf") || "";
  var midia = [];
  try { midia = JSON.parse(me.getAttribute("data-midia") || "[]"); } catch (e) {}
  var blocks = window["gjs-blocks-basic"];
  if (blocks && blocks.default) blocks = blocks.default;

  document.querySelectorAll("textarea[data-construtor],textarea[data-builder]").forEach(function (ta) {
    var css = "";
    var html = ta.value.replace(/<style[^>]*>([\s\S]*?)<\/style>/gi, function (_, c) { css += c; return ""; });
    var box = document.createElement("div");
    box.className = "nep-gjs";
    ta.style.display = "none";
    ta.parentNode.insertBefore(box, ta.nextSibling);

    var ed = grapesjs.init({
      container: box,
      height: "640px",
      width: "auto",
      fromElement: false,
      components: html,
      style: css,
      storageManager: false,
      telemetry: false, // nada sai do painel para servidores de terceiros
      cssIcons: base + "/_nep/construtor/font-awesome.min.css",
      i18n: window.gjsPt ? { locale: "pt", detectLocale: false, messages: { pt: window.gjsPt } } : {},
      plugins: blocks ? [blocks] : [],
      deviceManager: {
        devices: [
          { id: "desktop", name: "Computador", width: "" },
          { id: "tablet", name: "Tablet", width: "768px", widthMedia: "992px" },
          { id: "mobile", name: "Celular", width: "375px", widthMedia: "480px" }
        ]
      },
      assetManager: {
        upload: base + "/_admin/construtor/arquivo",
        uploadName: "arquivos",
        params: { _nep_csrf: csrf },
        multiUpload: true,
        autoAdd: true,
        assets: midia // imagens da biblioteca de mídia do painel
      }
    });
    if (blocks) {
      // nomes dos blocos em português
      var pt = { column1: "1 coluna", column2: "2 colunas", column3: "3 colunas", "column3-7": "2 colunas 3/7", text: "Texto", link: "Link", image: "Imagem", video: "Vídeo", map: "Mapa" };
      ed.on("load", function () {
        Object.keys(pt).forEach(function (id) {
          var b = ed.Blocks.get(id);
          if (b) b.set({ label: pt[id], category: "Básico" });
        });
        ed.Blocks.render();
      });
    }
    // abre a lista de blocos ao carregar
    ed.on("load", function () {
      var b = ed.Panels.getButton("views", "open-blocks");
      if (b) b.set("active", true);
    });

    var form = ta.form;
    if (form) {
      form.addEventListener("submit", function () {
        var h = ed.getHtml().replace(/^<body[^>]*>/i, "").replace(/<\/body>\s*$/i, "");
        var c = ed.getCss({ avoidProtected: true }) || "";
        ta.value = (c ? "<style>" + c + "</style>" : "") + h;
      });
    }
  });
})();
