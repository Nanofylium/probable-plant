/*!
 * Nepenthes Movimento 0.1.0
 * Uma linha dá movimento a qualquer página HTML parada:
 *   <script src="nepenthes.js" defer></script>
 *
 * Sem configurar nada (modo automático):
 *   - títulos, textos, imagens e cartões surgem ao rolar, em cascata
 *   - transição suave entre páginas e rolagem suave nas âncoras
 *   - botões com efeito de toque e leve elevação
 *   - cabeçalho ganha sombra quando a página rola
 *
 * Atributos opcionais em qualquer elemento:
 *   nep-anim="subir|descer|esquerda|direita|zoom|desfocar|girar|virar|surgir"
 *   nep-atraso="200"  nep-duracao="900"  nep-repetir   nep-parado (não anima)
 *   nep-contar              conta de 0 até o número do texto ("1.200+", "R$ 3.500,90")
 *   nep-digitar="A|B|C"     efeito de máquina de escrever (alterna frases)
 *   nep-paralaxe="0.3"      move mais devagar que a rolagem
 *   nep-flutuar             flutua suavemente
 *   nep-inclinar            cartão inclina em 3D seguindo o mouse
 *   nep-magnetico           botão é "puxado" pelo cursor
 *   nep-letreiro="30"       faixa rolando sem fim (logos, avisos); número = segundos por volta
 *
 * Opções no próprio <script>:
 *   data-auto="false"  data-intensidade="suave|media|forte"  data-progresso="true"  data-cor="#8a4b14"
 *
 * Respeita quem pediu "reduzir movimento" no sistema. Se o JavaScript falhar, nada fica escondido.
 */
(function () {
  'use strict';
  if (window.Nepenthes && window.Nepenthes.versao) return;

  var doc = document, root = doc.documentElement;
  var script = doc.currentScript || doc.querySelector('script[src*="nepenthes"]');
  function opt(n, d) { var v = script && script.getAttribute('data-' + n); return v == null ? d : v; }

  var AUTO = opt('auto', 'true') !== 'false';
  var K = { suave: 0.6, media: 1, forte: 1.7 }[opt('intensidade', 'media')] || 1;
  var COR = opt('cor', '');
  var REDUZ = !!(window.matchMedia && matchMedia('(prefers-reduced-motion: reduce)').matches);
  var TEM_IO = 'IntersectionObserver' in window;
  var D = Math.round(28 * K);
  var EASE = 'cubic-bezier(.16,.84,.28,1)';
  var TIPOS = ['subir', 'descer', 'esquerda', 'direita', 'zoom', 'desfocar', 'girar', 'virar', 'surgir'];

  /* ---------- estilos ---------- */
  var css =
    '.nep-r{transition:opacity var(--nep-dur,.8s) ' + EASE + ' var(--nep-atraso,0s),transform var(--nep-dur,.8s) ' + EASE + ' var(--nep-atraso,0s),filter var(--nep-dur,.8s) ' + EASE + ' var(--nep-atraso,0s)}' +
    '.nep-r.nep-espera{opacity:0}' +
    '.nep-espera.nep-subir{transform:translate3d(0,' + D + 'px,0)}' +
    '.nep-espera.nep-descer{transform:translate3d(0,-' + D + 'px,0)}' +
    '.nep-espera.nep-esquerda{transform:translate3d(-' + Math.round(D * 1.6) + 'px,0,0)}' +
    '.nep-espera.nep-direita{transform:translate3d(' + Math.round(D * 1.6) + 'px,0,0)}' +
    '.nep-espera.nep-zoom{transform:scale(' + (1 - 0.07 * K).toFixed(3) + ')}' +
    '.nep-espera.nep-desfocar{filter:blur(' + Math.round(10 * K) + 'px);transform:translate3d(0,' + Math.round(D / 2) + 'px,0)}' +
    '.nep-espera.nep-girar{transform:rotate(' + (-6 * K).toFixed(1) + 'deg) scale(.94)}' +
    '.nep-espera.nep-virar{transform:perspective(900px) rotateX(' + Math.round(22 * K) + 'deg);transform-origin:50% 0}' +
    '[nep-digitar]::after{content:"";display:inline-block;width:.08em;height:1em;margin-left:.06em;vertical-align:-.12em;background:currentColor;animation:nep-cursor 1s steps(1) infinite}' +
    '@keyframes nep-cursor{50%{opacity:0}}' +
    '[nep-flutuar]{animation:nep-flutua 5s ease-in-out infinite}' +
    '@keyframes nep-flutua{50%{transform:translateY(-' + Math.round(10 * K) + 'px)}}' +
    '[nep-inclinar]{transition:transform .25s ease;transform-style:preserve-3d;will-change:transform}' +
    '[nep-magnetico]{transition:transform .25s ease}' +
    '[nep-letreiro]{overflow:hidden;position:relative;-webkit-mask-image:linear-gradient(90deg,transparent,#000 8%,#000 92%,transparent);mask-image:linear-gradient(90deg,transparent,#000 8%,#000 92%,transparent)}' +
    '.nep-trilho{display:flex;width:max-content;gap:inherit;animation:nep-letreiro var(--nep-volta,30s) linear infinite}' +
    '.nep-trilho>*{flex:none}[nep-letreiro]:hover .nep-trilho{animation-play-state:paused}' +
    '@keyframes nep-letreiro{to{transform:translateX(-50%)}}' +
    '.nep-onda{position:absolute;border-radius:50%;transform:scale(0);background:currentColor;opacity:.28;pointer-events:none;animation:nep-onda .6s ease-out forwards}' +
    '@keyframes nep-onda{to{transform:scale(1);opacity:0}}' +
    '#nep-progresso{position:fixed;top:0;left:0;height:3px;width:100%;transform-origin:0 50%;transform:scaleX(0);z-index:2147483000;background:' + (COR || 'currentColor') + ';pointer-events:none}';
  if (AUTO && !REDUZ) {
    css +=
      'html{scroll-behavior:smooth}' +
      '@view-transition{navigation:auto}' +
      '::view-transition-old(root){animation:nep-sai .25s ease both}::view-transition-new(root){animation:nep-entra .35s ' + EASE + ' both}' +
      '@keyframes nep-sai{to{opacity:0}}@keyframes nep-entra{from{opacity:0;transform:translateY(8px)}}' +
      ':where(button,.btn,.botao,.button,input[type=submit],a[class*=btn],a[class*=botao],a[class*=button]){transition:transform .2s ease,box-shadow .2s ease,filter .2s ease}' +
      ':where(button,.btn,.botao,.button,input[type=submit],a[class*=btn],a[class*=botao],a[class*=button]):hover{transform:translateY(-2px);box-shadow:0 8px 20px -10px rgba(0,0,0,.45)}' +
      ':where(button,.btn,.botao,.button,input[type=submit],a[class*=btn],a[class*=botao],a[class*=button]):active{transform:translateY(0) scale(.98)}' +
      ':where(header,.header,.cabecalho){transition:box-shadow .3s ease,background-color .3s ease}' +
      'html.nep-rolou :where(header,.header,.cabecalho){box-shadow:0 6px 24px -14px rgba(0,0,0,.35)}';
  }
  if (REDUZ) css += '[nep-digitar]::after,[nep-flutuar],.nep-trilho{animation:none!important}';
  var st = doc.createElement('style');
  st.id = 'nepenthes-movimento';
  st.textContent = css;
  (doc.head || root).appendChild(st);

  /* ---------- revelar ao rolar ---------- */
  var io = null;
  if (TEM_IO && !REDUZ) {
    io = new IntersectionObserver(function (entradas) {
      var vis = entradas.filter(function (e) { return e.isIntersecting; });
      vis.sort(function (a, b) { return (a.boundingClientRect.top - b.boundingClientRect.top) || (a.boundingClientRect.left - b.boundingClientRect.left); });
      vis.forEach(function (e, i) {
        var el = e.target;
        if (!el.hasAttribute('nep-atraso')) el.style.setProperty('--nep-atraso', Math.min(i * 0.08, 0.6) + 's');
        el.classList.remove('nep-espera');
        if (!el.hasAttribute('nep-repetir')) io.unobserve(el);
      });
      entradas.forEach(function (e) {
        if (!e.isIntersecting && e.target.hasAttribute('nep-repetir') && e.boundingClientRect.top > 0) e.target.classList.add('nep-espera');
      });
    }, { threshold: 0.12, rootMargin: '0px 0px -6% 0px' });
  }

  function preparar(el, tipo) {
    if (!io || el.__nep) return;
    el.__nep = 1;
    if (TIPOS.indexOf(tipo) < 0) tipo = 'subir';
    var at = el.getAttribute('nep-atraso'), du = el.getAttribute('nep-duracao');
    if (at) el.style.setProperty('--nep-atraso', (parseFloat(at) / 1000) + 's');
    if (du) el.style.setProperty('--nep-dur', (parseFloat(du) / 1000) + 's');
    el.classList.add('nep-r', 'nep-espera', 'nep-' + tipo);
    io.observe(el);
    // garantia: se algo der errado, nada fica invisível
    setTimeout(function () { if (el.classList.contains('nep-espera') && !el.hasAttribute('nep-repetir') && visivelNaTela(el)) el.classList.remove('nep-espera'); }, 2500);
  }
  function visivelNaTela(el) { var r = el.getBoundingClientRect(); return r.bottom > 0 && r.top < innerHeight; }

  function ignorar(el) {
    if (el.__nep || el.closest('[nep-parado],script,style,template,noscript,svg')) return true;
    if (el.closest('.nep-r') || el.querySelector('.nep-r')) return true;
    if (el.closest('nav,[role=navigation],dialog,[popover]')) return true;
    var cs = getComputedStyle(el);
    if (cs.display === 'none' || cs.visibility === 'hidden' || cs.position === 'fixed' || cs.position === 'sticky') return true;
    if (el.closest('header,.header,.cabecalho') && !el.closest('main')) return true;
    return el.offsetWidth === 0 && el.offsetHeight === 0;
  }

  var BLOCOS = 'h1,h2,h3,h4,h5,h6,p,ul,ol,dl,blockquote,pre,table,form,figure,img,picture,video,iframe,hr,.card,.cartao';
  function eUnidade(pai) {
    var cs = getComputedStyle(pai);
    if (cs.display.indexOf('grid') < 0 && cs.display.indexOf('flex') < 0) return false;
    var filhos = pai.children;
    if (filhos.length < 2) return false;
    var blocos = 0;
    for (var i = 0; i < filhos.length; i++) if (filhos[i].children.length > 0 || /^(IMG|PICTURE|FIGURE|ARTICLE|LI)$/.test(filhos[i].tagName)) blocos++;
    return blocos >= 2;
  }

  function auto(raiz) {
    if (!io) return;
    raiz = raiz || doc.body;
    if (!raiz || !raiz.querySelectorAll) return;
    // cabeçalho desce na entrada
    if (raiz === doc.body) {
      var cab = doc.querySelector('body > header, body > .header, body > .cabecalho, body > nav');
      if (cab && !cab.closest('[nep-parado]') && !cab.hasAttribute('nep-anim')) preparar(cab, 'descer');
    }
    // cartões em grade/linha surgem como unidades
    var pais = raiz.querySelectorAll('div,section,ul,ol,main,article');
    if (pais.length < 4000) {
      for (var i = 0; i < pais.length; i++) {
        var p = pais[i];
        if (p.closest('nav,[nep-parado],header,.header,.cabecalho,[nep-letreiro]') || !eUnidade(p)) continue;
        for (var j = 0; j < p.children.length; j++) {
          var c = p.children[j];
          if (!c.hasAttribute('nep-anim') && !ignorar(c)) preparar(c, /^(IMG|PICTURE|FIGURE|VIDEO)$/.test(c.tagName) ? 'zoom' : 'subir');
        }
      }
    }
    var els = raiz.querySelectorAll(BLOCOS);
    for (var k = 0; k < els.length; k++) {
      var el = els[k];
      if (el.hasAttribute('nep-anim') || ignorar(el)) continue;
      if (/^(LI|TD|TR)$/.test(el.parentElement && el.parentElement.tagName)) continue;
      var t = el.tagName;
      preparar(el, /^(IMG|PICTURE|VIDEO|IFRAME|FIGURE)$/.test(t) ? 'zoom' : (t === 'H1' ? 'desfocar' : 'subir'));
    }
  }

  function explicitos(raiz) {
    raiz.querySelectorAll('[nep-anim]').forEach(function (el) { preparar(el, el.getAttribute('nep-anim')); });
  }

  /* ---------- contador ---------- */
  function contar(el) {
    if (el.__nepc) return; el.__nepc = 1;
    var txt = el.textContent;
    var m = txt.match(/\d{1,3}(?:\.\d{3})+(?:,\d+)?|\d+(?:[.,]\d+)?/);
    if (!m) return;
    var bruto = m[0], dec = 0, alvo;
    if (/\.\d{3}/.test(bruto) || /,/.test(bruto)) { // formato brasileiro
      var partes = bruto.split(','); dec = partes[1] ? partes[1].length : 0;
      alvo = parseFloat(partes[0].replace(/\./g, '') + (dec ? '.' + partes[1] : ''));
    } else {
      var pp = bruto.split('.'); dec = pp[1] ? pp[1].length : 0; alvo = parseFloat(bruto);
    }
    var antes = txt.slice(0, m.index), depois = txt.slice(m.index + bruto.length);
    var milhar = /\./.test(bruto) && /\.\d{3}/.test(bruto) || alvo >= 1000;
    function fmt(v) {
      return antes + v.toLocaleString('pt-BR', { minimumFractionDigits: dec, maximumFractionDigits: dec, useGrouping: milhar }) + depois;
    }
    el.setAttribute('aria-label', txt.trim());
    if (REDUZ || !TEM_IO) return;
    el.textContent = fmt(0);
    var dur = parseFloat(el.getAttribute('nep-duracao')) || 1800;
    var obs = new IntersectionObserver(function (es) {
      if (!es[0].isIntersecting) return;
      obs.disconnect();
      var t0 = performance.now();
      (function passo(t) {
        var p = Math.min(1, (t - t0) / dur), e = 1 - Math.pow(1 - p, 3);
        el.textContent = fmt(p < 1 ? alvo * e : alvo);
        if (p < 1) requestAnimationFrame(passo); else el.textContent = txt;
      })(t0);
    }, { threshold: 0.4 });
    obs.observe(el);
  }

  /* ---------- máquina de escrever ---------- */
  function digitar(el) {
    if (el.__nepd) return; el.__nepd = 1;
    var v = el.getAttribute('nep-digitar');
    var frases = (v && v.trim() ? v : el.textContent).split('|').map(function (s) { return s.trim(); }).filter(Boolean);
    if (!frases.length) return;
    el.setAttribute('aria-label', frases.join(', '));
    if (REDUZ) { el.textContent = frases[0]; return; }
    var i = 0, n = 0, apagando = false, vel = 55 / K;
    el.textContent = '';
    (function tique() {
      var f = frases[i];
      n += apagando ? -1 : 1;
      el.textContent = f.slice(0, n);
      var espera = apagando ? vel / 2 : vel + Math.random() * 40;
      if (!apagando && n >= f.length) {
        if (frases.length === 1) return;
        apagando = true; espera = 1700;
      } else if (apagando && n <= 0) {
        apagando = false; i = (i + 1) % frases.length; espera = 350;
      }
      setTimeout(tique, espera);
    })();
  }

  /* ---------- paralaxe e progresso (um só laço de rolagem) ---------- */
  var paralaxes = [], barra = null, agendado = false;
  function aoRolar() {
    if (agendado) return; agendado = true;
    requestAnimationFrame(function () {
      agendado = false;
      var vh = innerHeight;
      root.classList.toggle('nep-rolou', scrollY > 8);
      paralaxes.forEach(function (el) {
        var r = el.parentElement ? el.parentElement.getBoundingClientRect() : el.getBoundingClientRect();
        if (r.bottom < -200 || r.top > vh + 200) return;
        var centro = r.top + r.height / 2 - vh / 2;
        el.style.transform = 'translate3d(0,' + (centro * -el.__nepv).toFixed(1) + 'px,0)';
      });
      if (barra) {
        var max = root.scrollHeight - vh;
        barra.style.transform = 'scaleX(' + (max > 0 ? Math.min(1, scrollY / max) : 0) + ')';
      }
    });
  }

  /* ---------- inclinar e magnético ---------- */
  function inclinar(el) {
    if (el.__nepi || REDUZ) return; el.__nepi = 1;
    var g = parseFloat(el.getAttribute('nep-inclinar')) || 8 * K;
    el.addEventListener('pointermove', function (e) {
      if (e.pointerType !== 'mouse') return;
      var r = el.getBoundingClientRect(), x = (e.clientX - r.left) / r.width - 0.5, y = (e.clientY - r.top) / r.height - 0.5;
      el.style.transform = 'perspective(900px) rotateY(' + (x * g).toFixed(2) + 'deg) rotateX(' + (-y * g).toFixed(2) + 'deg) translateZ(0)';
    });
    el.addEventListener('pointerleave', function () { el.style.transform = ''; });
  }
  function magnetico(el) {
    if (el.__nepm || REDUZ) return; el.__nepm = 1;
    el.addEventListener('pointermove', function (e) {
      if (e.pointerType !== 'mouse') return;
      var r = el.getBoundingClientRect();
      el.style.transform = 'translate(' + ((e.clientX - r.left - r.width / 2) * 0.25 * K).toFixed(1) + 'px,' + ((e.clientY - r.top - r.height / 2) * 0.35 * K).toFixed(1) + 'px)';
    });
    el.addEventListener('pointerleave', function () { el.style.transform = ''; });
  }

  /* ---------- letreiro ---------- */
  function letreiro(el) {
    if (el.__nepl) return; el.__nepl = 1;
    var trilho = doc.createElement('div');
    trilho.className = 'nep-trilho';
    while (el.firstChild) trilho.appendChild(el.firstChild);
    var copia = trilho.cloneNode(true);
    var itens = Array.prototype.slice.call(copia.childNodes);
    itens.forEach(function (n) { if (n.nodeType === 1) n.setAttribute('aria-hidden', 'true'); trilho.appendChild(n); });
    el.appendChild(trilho);
    var s = parseFloat(el.getAttribute('nep-letreiro'));
    if (s > 0) trilho.style.setProperty('--nep-volta', s + 's');
  }

  /* ---------- onda nos botões ---------- */
  function onda(e) {
    if (REDUZ) return;
    var b = e.target.closest && e.target.closest('button,.btn,.botao,.button,a[class*=btn],a[class*=botao],a[class*=button],[nep-onda]');
    if (!b || b.closest('[nep-parado]')) return;
    var cs = getComputedStyle(b);
    if (cs.position === 'static') b.style.position = 'relative';
    if (cs.overflow !== 'hidden') b.style.overflow = 'hidden';
    var r = b.getBoundingClientRect(), t = Math.max(r.width, r.height) * 2.2, o = doc.createElement('span');
    o.className = 'nep-onda';
    o.style.cssText = 'width:' + t + 'px;height:' + t + 'px;left:' + (e.clientX - r.left - t / 2) + 'px;top:' + (e.clientY - r.top - t / 2) + 'px';
    b.appendChild(o);
    setTimeout(function () { o.remove(); }, 650);
  }

  /* ---------- montar ---------- */
  function varrer(raiz) {
    raiz = raiz || doc.body;
    if (!raiz || !raiz.querySelectorAll) return;
    explicitos(raiz);
    if (AUTO) auto(raiz);
    raiz.querySelectorAll('[nep-contar]').forEach(contar);
    raiz.querySelectorAll('[nep-digitar]').forEach(digitar);
    raiz.querySelectorAll('[nep-inclinar]').forEach(inclinar);
    raiz.querySelectorAll('[nep-magnetico]').forEach(magnetico);
    raiz.querySelectorAll('[nep-letreiro]').forEach(letreiro);
    if (!REDUZ) raiz.querySelectorAll('[nep-paralaxe]').forEach(function (el) {
      if (el.__nepv) return;
      el.__nepv = parseFloat(el.getAttribute('nep-paralaxe')) || 0.2;
      el.style.willChange = 'transform';
      paralaxes.push(el);
    });
    aoRolar();
  }

  function iniciar() {
    if (opt('progresso', 'false') === 'true' && !doc.getElementById('nep-progresso')) {
      barra = doc.createElement('div'); barra.id = 'nep-progresso'; barra.setAttribute('aria-hidden', 'true');
      if (!COR) barra.style.color = getComputedStyle(doc.querySelector('a') || doc.body).color;
      doc.body.appendChild(barra);
    }
    varrer(doc.body);
    addEventListener('scroll', aoRolar, { passive: true });
    addEventListener('resize', aoRolar, { passive: true });
    if (AUTO) doc.addEventListener('pointerdown', onda, { passive: true });
    if ('MutationObserver' in window) {
      var fila = [], t = null;
      new MutationObserver(function (ms) {
        ms.forEach(function (m) { m.addedNodes.forEach(function (n) { if (n.nodeType === 1 && n.className !== 'nep-onda' && n.className !== 'nep-trilho') fila.push(n); }); });
        if (fila.length && !t) t = setTimeout(function () { var f = fila; fila = []; t = null; f.forEach(function (n) { if (n.isConnected) varrer(n.parentElement || n); }); }, 60);
      }).observe(doc.body, { childList: true, subtree: true });
    }
  }

  window.Nepenthes = {
    versao: '0.1.0',
    atualizar: function (el) { varrer(el || doc.body); },
    animar: function (el, tipo) {
      if (!el || REDUZ) return;
      el.__nep = 0; el.classList.remove('nep-r', 'nep-espera');
      TIPOS.forEach(function (t) { el.classList.remove('nep-' + t); });
      void el.offsetWidth;
      if (io) preparar(el, tipo || 'subir');
    }
  };

  if (doc.readyState === 'loading') doc.addEventListener('DOMContentLoaded', iniciar);
  else iniciar();
})();
