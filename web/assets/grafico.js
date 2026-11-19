/* Nepenthes: desenha os gráficos de grafico() com o Apache ECharts (Apache 2.0),
   na cor e na fonte do site. Sem JavaScript, fica a tabela com os números. */
(function () {
  if (window.__nepGrafico) return;
  window.__nepGrafico = 1;
  var src = document.currentScript && document.currentScript.getAttribute("data-echarts");

  function carregar(pronto) {
    if (window.echarts) return pronto();
    var s = document.createElement("script");
    s.src = src;
    s.onload = pronto;
    document.head.appendChild(s);
  }

  function hex(c) {
    var m = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i.exec((c || "").trim());
    if (!m) return null;
    var h = m[1].length === 3 ? m[1].replace(/./g, "$&$&") : m[1];
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
  }
  function hsl(r, g, b) {
    r /= 255; g /= 255; b /= 255;
    var mx = Math.max(r, g, b), mn = Math.min(r, g, b), h = 0, s = 0, l = (mx + mn) / 2, d = mx - mn;
    if (d) {
      s = l > .5 ? d / (2 - mx - mn) : d / (mx + mn);
      h = mx === r ? (g - b) / d + (g < b ? 6 : 0) : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
      h *= 60;
    }
    return [h, s * 100, l * 100];
  }
  // paleta a partir da cor do site: a própria cor e vizinhas no círculo das cores
  function paleta(el) {
    var cs = getComputedStyle(el), base = hex(cs.getPropertyValue("--nep-cor")) || [47, 107, 79];
    var c = hsl(base[0], base[1], base[2]), s = Math.max(c[1], 45), l = Math.min(Math.max(c[2], 38), 58);
    return [0, 38, 165, -55, 95, 210, 280].map(function (d, i) {
      return "hsl(" + ((c[0] + d + 360) % 360).toFixed(0) + "," + (i ? s * .85 : s).toFixed(0) + "%," + (i ? l + 6 : l).toFixed(0) + "%)";
    });
  }
  function luz(c) {
    var m = /rgba?\((\d+),\s*(\d+),\s*(\d+)/.exec(c);
    return m ? (0.299 * m[1] + 0.587 * m[2] + 0.114 * m[3]) / 255 : 1;
  }
  function fundo(el) {
    for (var e = el; e; e = e.parentElement) {
      var b = getComputedStyle(e).backgroundColor;
      if (b && !/rgba\(0, 0, 0, 0\)|transparent/.test(b)) return b;
    }
    return "rgb(255,255,255)";
  }

  function desenhar(fig) {
    var g;
    try { g = JSON.parse(fig.getAttribute("data-grafico")); } catch (e) { return; }
    fig.setAttribute("data-pronto", "1");
    var cs = getComputedStyle(fig), texto = cs.color, fonte = getComputedStyle(document.body).fontFamily;
    var escuro = luz(fundo(fig)) < .45, linha = escuro ? "rgba(255,255,255,.12)" : "rgba(0,0,0,.08)";
    var cores = g.cores && g.cores.length ? g.cores : paleta(fig);
    var area = document.createElement("div");
    area.style.cssText = "width:100%;height:" + (g.altura || 320) + "px";
    var tab = fig.querySelector(".nep-grafico-dados");
    if (tab) tab.style.cssText = "position:absolute;width:1px;height:1px;overflow:hidden;clip:rect(0 0 0 0);white-space:nowrap";
    fig.style.position = "relative";
    fig.insertBefore(area, tab);

    var fmt = function (v) {
      var n = Number(v);
      return g.moeda ? "R$ " + n.toLocaleString("pt-BR", {minimumFractionDigits: 2, maximumFractionDigits: 2}) : n.toLocaleString("pt-BR");
    };
    var redondo = g.tipo === "pizza" || g.tipo === "rosca";
    var varias = (g.series || []).length > 1;
    var op = {
      color: cores, animationDuration: 900, animationEasing: "cubicOut",
      textStyle: {fontFamily: fonte, color: texto},
      tooltip: {trigger: redondo ? "item" : "axis", valueFormatter: fmt, backgroundColor: escuro ? "#1b2027" : "#fff",
        borderColor: linha, textStyle: {color: escuro ? "#eee" : "#222", fontFamily: fonte}},
      legend: varias || redondo ? {bottom: 0, icon: "roundRect", textStyle: {color: texto, fontFamily: fonte}} : undefined,
      grid: {left: 8, right: 16, top: 16, bottom: varias ? 36 : 8, containLabel: true}
    };
    if (redondo) {
      var s0 = (g.series && g.series[0]) || {valores: []};
      op.series = [{type: "pie", name: s0.nome, radius: g.tipo === "rosca" ? ["46%", "72%"] : ["0%", "72%"], center: ["50%", "45%"],
        itemStyle: {borderRadius: g.tipo === "rosca" ? 8 : 4, borderColor: fundo(fig), borderWidth: 2},
        label: {color: texto, formatter: "{b}\n{d}%"},
        data: (g.rotulos || []).map(function (r, i) { return {name: r, value: s0.valores[i]}; })}];
    } else {
      var deitado = g.tipo === "horizontal";
      var cat = {type: "category", data: g.rotulos, axisLine: {lineStyle: {color: linha}}, axisTick: {show: false}, axisLabel: {color: texto}};
      var val = {type: "value", splitLine: {lineStyle: {color: linha}}, axisLabel: {color: texto, formatter: function (v) { return g.moeda ? "R$ " + v.toLocaleString("pt-BR") : v.toLocaleString("pt-BR"); }}};
      op.xAxis = deitado ? val : cat;
      if (deitado) cat.inverse = true;
      op.yAxis = deitado ? cat : val;
      op.series = (g.series || []).map(function (s, i) {
        var linhaTipo = g.tipo === "linhas" || g.tipo === "area";
        var o = {type: linhaTipo ? "line" : "bar", name: s.nome, data: s.valores};
        if (linhaTipo) {
          o.smooth = true; o.symbolSize = 7; o.lineStyle = {width: 3};
          if (g.tipo === "area") o.areaStyle = {color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{offset: 0, color: cores[i % cores.length]}, {offset: 1, color: "rgba(255,255,255,0)"}]), opacity: .35};
        } else {
          o.barMaxWidth = 46;
          o.itemStyle = {borderRadius: deitado ? [0, 6, 6, 0] : [6, 6, 0, 0]};
        }
        return o;
      });
    }
    var ch = echarts.init(area, null, {renderer: "svg"});
    ch.setOption(op);
    if (window.ResizeObserver) new ResizeObserver(function () { ch.resize(); }).observe(area);
    else window.addEventListener("resize", function () { ch.resize(); });
  }

  function todos() {
    var els = document.querySelectorAll(".nep-grafico[data-grafico]:not([data-pronto])");
    if (els.length) carregar(function () { Array.prototype.forEach.call(els, desenhar); });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", todos);
  else todos();
  // as páginas que trocam o conteúdo (regiões ao vivo, prévia) chamam de novo
  window.nepGraficos = todos;
})();
