/*!
 * Nepenthes Vivo: as regiões <live> de um .nep.html se atualizam sozinhas,
 * sem recarregar a página e sem escrever JavaScript.
 *   <live>          atualiza quando os dados do site mudam (alguém enviou um formulário, por exemplo)
 *   <live every=10> atualiza a cada 10 segundos
 * Formulários dentro de um <live> são enviados sem recarregar a página: a região
 * volta já atualizada (com a mensagem de sucesso ou os erros). Use nep-recarregar
 * no <form> para o envio normal.
 */
(function () {
  'use strict';
  var me = document.currentScript;
  if (!me || !window.fetch) return;
  var base = me.getAttribute('data-base') || '';
  var last = {};
  var version = null;

  function all() { return [].slice.call(document.querySelectorAll('[nep-live]')); }

  function refresh(el) {
    var id = el.getAttribute('nep-live');
    var u = location.pathname + location.search;
    u += (u.indexOf('?') < 0 ? '?' : '&') + '_nep_live=' + encodeURIComponent(id);
    fetch(u, { credentials: 'same-origin', cache: 'no-store', headers: { 'X-Nep-Live': '1' } })
      .then(function (r) { return r.ok ? r.text() : null; })
      .then(function (html) {
        if (html === null || html === last[id]) return;
        last[id] = html;
        el.innerHTML = html;
        if (window.Nepenthes && window.Nepenthes.atualizar) window.Nepenthes.atualizar();
        el.dispatchEvent(new CustomEvent('nep:atualizado', { bubbles: true }));
      })
      .catch(function () {});
  }

  function swap(doc) {
    all().forEach(function (el) {
      var id = el.getAttribute('nep-live');
      var n = doc.querySelector('[nep-live="' + id + '"]');
      if (!n) return;
      last[id] = n.innerHTML;
      el.innerHTML = n.innerHTML;
    });
    if (window.Nepenthes && window.Nepenthes.atualizar) window.Nepenthes.atualizar();
  }

  document.addEventListener('submit', function (e) {
    var f = e.target;
    if (e.defaultPrevented || !f.closest || !f.closest('[nep-live]') || f.hasAttribute('nep-recarregar')) return;
    if ((f.getAttribute('method') || 'get').toLowerCase() !== 'post' || f.enctype === 'multipart/form-data') return;
    e.preventDefault();
    var data = new URLSearchParams(new FormData(f));
    if (e.submitter && e.submitter.name) data.append(e.submitter.name, e.submitter.value);
    var btns = f.querySelectorAll('button, [type=submit]');
    btns.forEach(function (b) { b.disabled = true; });
    fetch(f.action || location.href, { method: 'POST', body: data, credentials: 'same-origin', headers: { 'X-Nep-Live': '1' } })
      .then(function (r) { return r.text(); })
      .then(function (html) { swap(new DOMParser().parseFromString(html, 'text/html')); })
      .catch(function () { f.submit(); })
      .then(function () { btns.forEach(function (b) { b.disabled = false; }); });
  });

  all().forEach(function (el) {
    var n = +el.getAttribute('data-every');
    if (n > 0) setInterval(function () { if (!document.hidden) refresh(el); }, n * 1000);
  });

  var watch = all().filter(function (el) { return !(+el.getAttribute('data-every')); });
  if (!watch.length) return;

  function changed(v) {
    v = String(v).trim();
    if (version !== null && v !== version) watch.forEach(refresh);
    version = v;
  }
  if (window.EventSource && me.getAttribute('data-sse') === '1') {
    var es = new EventSource(base + '/_nep/eventos');
    es.onmessage = function (e) { changed(e.data); };
  } else {
    var poll = function () {
      if (document.hidden) return;
      fetch(base + '/_nep/versao', { cache: 'no-store' }).then(function (r) { return r.text(); }).then(changed).catch(function () {});
    };
    poll();
    setInterval(poll, 4000);
  }
})();
