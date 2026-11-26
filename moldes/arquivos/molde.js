/*!
 * Scripts dos moldes do Nepenthes. Baseado nos scripts.js do Start Bootstrap
 * Clean Blog e Simple Sidebar (Copyright 2013-2023 Start Bootstrap, MIT).
 */
window.addEventListener('DOMContentLoaded', function () {
  // Clean Blog: o menu some ao descer e volta ao subir
  var nav = document.getElementById('mainNav');
  if (nav) {
    var pos = 0, alto = nav.clientHeight;
    window.addEventListener('scroll', function () {
      var topo = document.body.getBoundingClientRect().top * -1;
      if (topo < pos) {
        if (topo > 0 && nav.classList.contains('is-fixed')) nav.classList.add('is-visible');
        else nav.classList.remove('is-visible', 'is-fixed');
      } else {
        nav.classList.remove('is-visible');
        if (topo > alto && !nav.classList.contains('is-fixed')) nav.classList.add('is-fixed');
      }
      pos = topo;
    });
  }
  // Simple Sidebar: botão que abre e fecha a barra lateral
  var botao = document.getElementById('sidebarToggle');
  if (botao) {
    botao.addEventListener('click', function (e) {
      e.preventDefault();
      document.body.classList.toggle('sb-sidenav-toggled');
    });
  }
});
