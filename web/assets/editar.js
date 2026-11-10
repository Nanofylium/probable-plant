/* Nepenthes: edição dos textos direto na página (só para quem está no painel).
   O programador marca a tag com editavel="chave"; aqui quem cuida do site
   clica no texto, escreve e salva, sem mexer no código nem no layout. */
(function () {
  var me = document.currentScript;
  var base = me.getAttribute("data-base") || "";
  var csrf = me.getAttribute("data-csrf") || "";
  var on = false;
  try { on = sessionStorage.getItem("nep-editar") === "1"; } catch (e) {}

  var css = document.createElement("style");
  css.textContent =
    "html.nep-ed [data-nep-ed]{outline:2px dashed rgba(43,107,69,.55);outline-offset:3px;cursor:text;border-radius:2px}" +
    "html.nep-ed [data-nep-tipo=imagem],html.nep-ed [data-nep-tipo=rico]{cursor:pointer}" +
    "html.nep-ed [data-nep-ed]:hover{outline-color:#2b6b45;background:rgba(43,107,69,.06)}" +
    "[data-nep-ed][contenteditable]{outline:2px solid #2b6b45!important;background:#fff;color:inherit}" +
    ".nep-bar{position:fixed;right:16px;bottom:16px;z-index:2147483000;display:flex;gap:6px;font:14px/1.3 system-ui,sans-serif}" +
    ".nep-bar button,.nep-bar a{font:inherit;border:0;border-radius:99px;padding:9px 14px;background:#2b6b45;color:#fff;cursor:pointer;text-decoration:none;box-shadow:0 2px 10px rgba(0,0,0,.2)}" +
    ".nep-bar .sec{background:#fff;color:#2b6b45}.nep-msg{position:fixed;left:50%;bottom:70px;transform:translateX(-50%);background:#1d2a22;color:#fff;padding:8px 14px;border-radius:8px;font:14px system-ui,sans-serif;z-index:2147483001}";
  document.head.appendChild(css);

  var bar = document.createElement("div");
  bar.className = "nep-bar";
  var toggle = document.createElement("button");
  var panel = document.createElement("a");
  panel.href = base + "/_admin/textos";
  panel.className = "sec";
  panel.textContent = "Painel";
  bar.appendChild(panel);
  bar.appendChild(toggle);
  document.body.appendChild(bar);

  var last = null;
  function say(t) {
    if (last) last.remove();
    var m = last = document.createElement("div");
    m.className = "nep-msg";
    m.setAttribute("role", "status");
    m.textContent = t;
    document.body.appendChild(m);
    setTimeout(function () { m.remove(); }, 2200);
  }
  function paint() {
    document.documentElement.classList.toggle("nep-ed", on);
    toggle.textContent = on ? "Concluir edição" : "✎ Editar textos";
    try { sessionStorage.setItem("nep-editar", on ? "1" : "0"); } catch (e) {}
  }
  toggle.addEventListener("click", function () {
    on = !on;
    if (!on && editing) finish(editing, false);
    paint();
  });
  paint();

  var editing = null, before = "";
  function finish(el, save) {
    var key = el.getAttribute("data-nep-ed");
    el.removeAttribute("contenteditable");
    editing = null;
    if (!save) { el.innerHTML = before; return; }
    var text = el.innerText.replace(/ /g, " ").replace(/\n{3,}/g, "\n\n").trim();
    var body = new URLSearchParams({ _nep_csrf: csrf, chave: key, valor: text, ajax: "1" });
    fetch(base + "/_admin/textos", { method: "POST", body: body, credentials: "same-origin", headers: { "Accept": "application/json" } })
      .then(function (r) { return r.json().then(function (j) { return { ok: r.ok, j: j }; }); })
      .then(function (res) {
        if (!res.ok) { el.innerHTML = before; say(res.j.erro || "Não foi possível salvar."); return; }
        var html = res.j.html || "";
        document.querySelectorAll('[data-nep-ed="' + key.replace(/"/g, "") + '"]').forEach(function (o) { o.innerHTML = html; });
        say("Salvo ✓");
      })
      .catch(function () { el.innerHTML = before; say("Sem conexão: o texto não foi salvo."); });
  }

  document.addEventListener("click", function (ev) {
    if (!on) return;
    var el = ev.target.closest && ev.target.closest("[data-nep-ed]");
    if (!el || el === editing) return;
    ev.preventDefault();
    ev.stopPropagation();
    var tipo = el.getAttribute("data-nep-tipo");
    if (tipo !== "texto") {
      location.href = base + "/_admin/textos#ed-" + encodeURIComponent(el.getAttribute("data-nep-ed"));
      return;
    }
    if (editing) finish(editing, true);
    editing = el;
    before = el.innerHTML;
    try { el.contentEditable = "plaintext-only"; } catch (e) { el.contentEditable = "true"; }
    if (el.contentEditable !== "plaintext-only") el.contentEditable = "true";
    el.focus();
    say("Escreva e aperte Enter para salvar (Shift+Enter quebra a linha, Esc cancela).");
  }, true);

  document.addEventListener("keydown", function (ev) {
    if (!editing) return;
    if (ev.key === "Escape") { ev.preventDefault(); finish(editing, false); }
    else if (ev.key === "Enter" && !ev.shiftKey) { ev.preventDefault(); finish(editing, true); }
  });
  document.addEventListener("focusout", function (ev) {
    if (editing && ev.target === editing) finish(editing, true);
  });
})();
