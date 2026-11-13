package gerador

import "strings"

// css: só o que é deste site; o tema Nepenthes cuida do resto.
func (g *ger) css() string {
	var b strings.Builder
	b.WriteString(`    /* O tema Nepenthes faz o grosso; aqui só o que é deste site. */
    .marca span { display: inline-grid; place-items: center; width: 38px; height: 38px; border-radius: 12px; background: color-mix(in srgb, var(--nep-cor) 14%, #fff); border: 1px solid color-mix(in srgb, var(--nep-cor) 30%, #fff); font-size: 1.15rem; }
    .topo-faixa { background: var(--nep-escuro); color: color-mix(in srgb, #fff 75%, var(--nep-cor)); font-size: .85rem; padding: 7px var(--nep-margem); display: flex; justify-content: space-between; flex-wrap: wrap; gap: 4px 18px; }
    .topo-faixa a { color: #fff; text-decoration: none; font-weight: 600; }
    .topo-acoes { display: inline-flex; gap: 16px; align-items: center; }
    .idiomas { display: inline-flex; gap: 8px; } .idiomas a { opacity: .8; }
    .capa:not(.cor) { background: radial-gradient(700px 380px at 85% 30%, color-mix(in srgb, var(--nep-cor) 18%, transparent), transparent 70%), linear-gradient(180deg, color-mix(in srgb, var(--nep-cor) 5%, #fff), var(--nep-fundo)); }
    .ilustracao { aspect-ratio: 5/4; border-radius: 28px; display: grid; place-items: center; position: relative; overflow: hidden;
      background: radial-gradient(circle at 72% 28%, color-mix(in srgb, var(--nep-cor) 45%, #fff) 0 16%, transparent 17%),
                  linear-gradient(155deg, color-mix(in srgb, var(--nep-cor) 22%, #fff), color-mix(in srgb, var(--nep-cor) 80%, #000));
      box-shadow: 0 30px 70px rgba(0,0,0,.18); }
    .ilustracao span { font-size: clamp(5rem, 13vw, 9rem); filter: drop-shadow(0 18px 30px rgba(0,0,0,.25)); }
    .ilustracao::after { content: ""; position: absolute; left: 8%; bottom: 9%; width: 46%; height: 16%; border-radius: 14px; background: rgba(255,255,255,.85); box-shadow: 0 12px 30px rgba(0,0,0,.15); }
    .servico .preco, .produto .preco { font-weight: 700; color: var(--nep-cor); }
    .preco-grande { font-size: 1.7rem; font-weight: 800; color: var(--nep-cor); margin: .2em 0 .6em; }
    .depoimento p { font-size: 1.05rem; color: var(--nep-texto); }
    .depoimento footer { margin-top: 10px; font-size: .9rem; color: var(--nep-suave); }
    .estrelas { color: #f59e0b; letter-spacing: 2px; }
    .estreito { max-width: 760px; margin: 0 auto; }
    details { background: var(--nep-superficie); border: 1px solid var(--nep-linha); border-radius: var(--nep-raio); padding: 16px 20px; margin-bottom: 10px; }
    details summary { cursor: pointer; font-weight: 700; }
    details p { margin: 10px 0 0; color: var(--nep-suave); }
    .ok-grande { text-align: center; max-width: 620px; margin: 0 auto; }
    .ok-grande .icone { width: 84px; height: 84px; font-size: 2.6rem; border-radius: 50%; background: #dcfce7; margin: 0 auto 12px; }
    .aviso-ok { background: #ecfdf5; border: 1px solid #a7f3d0; color: #065f46; border-radius: 12px; padding: 10px 14px; font-weight: 600; }
    .contato { display: grid; grid-template-columns: .9fr 1.1fr; gap: 48px; align-items: start; }
    .contato dl { display: grid; gap: 14px; margin: 0; }
    .contato dt { font-size: .78rem; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; color: var(--nep-suave); }
    .contato dd { margin: 0; font-size: 1.05rem; }
    .contato form { max-width: none; }
    .duplo { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
    .lista-simples { padding-left: 1.1em; } .lista-simples li { margin: .35em 0; }
    .chips { display: flex; flex-wrap: wrap; gap: 8px; margin: 0 0 26px; }
    .chips a { padding: 6px 14px; border-radius: 999px; border: 1px solid var(--nep-linha); background: var(--nep-superficie); color: var(--nep-texto); text-decoration: none; font-size: .9rem; font-weight: 600; }
    .chips a[aria-current=page] { background: var(--nep-cor); border-color: var(--nep-cor); color: #fff; }
    .busca-mini { display: flex; gap: 8px; max-width: none; } .busca-mini input { flex: 1; min-width: 0; }
    .grade + p.centro, .fotos + p.centro { margin-top: 32px; }
    .grupo { margin: 1.4em 0 .5em; font-size: 1.2rem; }
    @media (max-width: 820px) { .contato { grid-template-columns: 1fr; } }
    @media (max-width: 520px) { .duplo { grid-template-columns: 1fr; } }
`)
	if g.tem("agenda") {
		b.WriteString(`    .agenda { display: grid; grid-template-columns: 1.15fr .85fr; gap: 40px; align-items: start; }
    .agenda form { max-width: none; }
    .dias { display: grid; gap: 12px; }
    .dia h4 { margin: 0 0 8px; font-size: .95rem; text-transform: capitalize; }
    .horas { display: flex; flex-wrap: wrap; gap: 6px; }
    .horas span { font-size: .85rem; font-weight: 600; padding: 4px 10px; border-radius: 999px; background: #ecfdf5; color: #047857; border: 1px solid #a7f3d0; }
    .horas em { font-size: .85rem; color: var(--nep-suave); }
    .vivo { display: inline-flex; align-items: center; gap: 8px; font-size: .82rem; color: var(--nep-suave); font-weight: 600; }
    .vivo i { width: 8px; height: 8px; border-radius: 50%; background: #22c55e; box-shadow: 0 0 0 4px rgba(34,197,94,.18); }
    @media (max-width: 860px) { .agenda { grid-template-columns: 1fr; } }
`)
	}
	if g.tem("catalogo") {
		b.WriteString(`    .carrinho-topo { text-decoration: none; font-weight: 700; color: var(--nep-texto); white-space: nowrap; }
    .carrinho-topo b { display: inline-grid; place-items: center; min-width: 22px; height: 22px; border-radius: 999px; background: var(--nep-cor); color: #fff; font-size: .75rem; padding: 0 6px; }
    .produto { display: flex; flex-direction: column; gap: 4px; }
    .produto h3 { margin: 2px 0; font-size: 1.05rem; } .produto h3 a { color: inherit; text-decoration: none; }
    .produto .preco { margin: 0 0 8px; font-size: 1.1rem; }
    .produto form { margin-top: auto; max-width: none; }
    .produto form button { width: 100%; }
    .foto-produto { display: grid; place-items: center; aspect-ratio: 4/3; border-radius: calc(var(--nep-raio) - 4px); overflow: hidden; margin-bottom: 8px; text-decoration: none;
      background: linear-gradient(150deg, color-mix(in srgb, var(--nep-cor) 10%, #fff), color-mix(in srgb, var(--nep-cor) 26%, #fff)); }
    .foto-produto img { width: 100%; height: 100%; object-fit: cover; margin: 0; border-radius: 0; }
    .foto-produto span { font-size: 3.4rem; }
    .foto-produto.grande span { font-size: 7rem; }
    .produto-detalhe { display: grid; grid-template-columns: 1fr 1fr; gap: 48px; align-items: start; margin-top: 12px; }
    .comprar-grande { display: flex; gap: 12px; align-items: end; max-width: none; flex-wrap: wrap; }
    .comprar-grande label { width: 120px; margin: 0; }
    .carrinho { display: grid; grid-template-columns: 1.2fr .8fr; gap: 32px; align-items: start; }
    .carrinho table { width: 100%; border-collapse: collapse; }
    .carrinho th, .carrinho td { text-align: left; padding: 10px 6px; border-bottom: 1px solid var(--nep-linha); }
    .carrinho td form { margin: 0; }
    .carrinho form.cartao { max-width: none; }
    @media (max-width: 860px) { .produto-detalhe, .carrinho { grid-template-columns: 1fr; } }
    @media (max-width: 560px) { .grade.quatro:has(.produto) { grid-template-columns: 1fr 1fr; gap: 12px; } .produto { padding: 12px; } .foto-produto span { font-size: 2.6rem; } .produto h3 { font-size: .95rem; } }
`)
	}
	if g.tem("eventos") {
		b.WriteString(`    .evento { display: grid; grid-template-columns: 64px 1fr; gap: 16px; align-items: start; }
    .evento h3 { margin-top: 0; }
    .data-ev { display: grid; text-align: center; border-radius: 14px; overflow: hidden; border: 1px solid var(--nep-linha); background: var(--nep-superficie); }
    .data-ev b { font-size: 1.6rem; padding: 6px 0 2px; }
    .data-ev span { background: var(--nep-cor); color: #fff; font-size: .75rem; font-weight: 800; text-transform: uppercase; padding: 3px 0; }
    .vagas b { font-size: 1.5rem; color: var(--nep-cor); }
`)
	}
	if g.tem("equipe") {
		b.WriteString(`    .pessoa { text-align: center; }
    .pessoa .foto { width: 104px; height: 104px; margin: 0 auto 14px; border-radius: 50%; display: grid; place-items: center; font-size: 2.6rem; font-weight: 800; color: #fff; overflow: hidden;
      background: linear-gradient(145deg, color-mix(in srgb, var(--nep-cor) 55%, #fff), var(--nep-cor)); }
    .pessoa .foto img { width: 100%; height: 100%; object-fit: cover; border-radius: 0; margin: 0; }
    .pessoa h3 { margin-bottom: 2px; }
`)
	}
	if g.tem("galeria") {
		b.WriteString(`    .fotos { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), 1fr)); gap: 14px; }
    .foto-item { margin: 0; position: relative; border-radius: var(--nep-raio); overflow: hidden; aspect-ratio: 4/3; }
    .foto-item img, .foto-item .sem-foto { width: 100%; height: 100%; object-fit: cover; margin: 0; border-radius: 0; display: grid; place-items: center; font-size: 3rem;
      background: linear-gradient(140deg, color-mix(in srgb, var(--nep-cor) 30%, #fff), color-mix(in srgb, var(--nep-cor) 75%, #000)); }
    .foto-item figcaption { position: absolute; left: 0; right: 0; bottom: 0; padding: 22px 14px 10px; color: #fff; font-weight: 600; font-size: .92rem; background: linear-gradient(transparent, rgba(0,0,0,.6)); }
`)
	}
	if g.tem("planos") {
		b.WriteString(`    .precos { align-items: stretch; }
    .plano { display: flex; flex-direction: column; }
    .plano .valor { margin: .2em 0 .8em; } .plano .valor b { font-size: 2rem; } .plano .valor span { color: var(--nep-suave); margin-left: 4px; }
    .plano ul { padding-left: 1.1em; margin: 0 0 1.2em; flex: 1; } .plano li { margin: .4em 0; }
    .plano.destaque { border: 2px solid var(--nep-cor); box-shadow: 0 24px 50px color-mix(in srgb, var(--nep-cor) 22%, transparent); }
    .plano .selo { align-self: flex-start; }
`)
	}
	if g.tem("blog") {
		b.WriteString(`    .lista-posts { display: grid; gap: 22px; }
    .post-linha { display: grid; grid-template-columns: 220px 1fr; gap: 22px; align-items: start; padding-bottom: 22px; border-bottom: 1px solid var(--nep-linha); }
    .post-linha h2 { margin: 4px 0 6px; font-size: 1.35rem; } .post-linha h2 a { color: inherit; text-decoration: none; }
    .post-linha p { margin: 0; color: var(--nep-suave); }
    .capa-post { display: grid; place-items: center; aspect-ratio: 16/10; border-radius: calc(var(--nep-raio) - 4px); overflow: hidden; font-size: 2.6rem; text-decoration: none;
      background: linear-gradient(140deg, color-mix(in srgb, var(--nep-cor) 14%, #fff), color-mix(in srgb, var(--nep-cor) 34%, #fff)); }
    .capa-post img { width: 100%; height: 100%; object-fit: cover; margin: 0; border-radius: 0; }
    .tags { display: flex; flex-wrap: wrap; gap: 6px; }
    .tags a { font-size: .82rem; padding: 3px 10px; border-radius: 999px; background: color-mix(in srgb, var(--nep-cor) 10%, #fff); color: var(--nep-cor); text-decoration: none; font-weight: 600; }
    .paginacao { display: flex; gap: 6px; margin-top: 28px; }
    .paginacao a { min-width: 38px; height: 38px; display: grid; place-items: center; border-radius: 10px; border: 1px solid var(--nep-linha); text-decoration: none; color: var(--nep-texto); }
    .paginacao a[aria-current=page] { background: var(--nep-cor); color: #fff; border-color: var(--nep-cor); }
    @media (max-width: 640px) { .post-linha { grid-template-columns: 1fr; } }
`)
	}
	if g.tem("wiki") {
		b.WriteString(`    .letras { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 8px; }
    .letras a { width: 34px; height: 34px; display: grid; place-items: center; border-radius: 8px; background: var(--nep-alt); text-decoration: none; font-weight: 700; color: var(--nep-texto); }
    .lista-wiki { columns: 2; padding-left: 1.1em; } .lista-wiki li { margin: .3em 0; break-inside: avoid; }
    @media (max-width: 640px) { .lista-wiki { columns: 1; } }
`)
	}
	if g.tem("forum") {
		b.WriteString(`    .topicos { display: grid; border: 1px solid var(--nep-linha); border-radius: var(--nep-raio); overflow: hidden; background: var(--nep-superficie); }
    .topico { display: flex; gap: 14px; align-items: center; padding: 14px 18px; border-bottom: 1px solid var(--nep-linha); text-decoration: none; color: var(--nep-texto); }
    .topico:last-child { border-bottom: 0; } .topico:hover { background: var(--nep-alt); }
    .avatar { display: inline-grid; place-items: center; width: 36px; height: 36px; border-radius: 50%; background: var(--nep-cor); color: #fff; font-weight: 800; flex: none; vertical-align: middle; }
    .mensagem p:last-child { margin-bottom: 0; }
`)
	}
	if g.tem("downloads") {
		b.WriteString(`    .arquivos { display: grid; gap: 10px; }
    .arquivo { display: grid; grid-template-columns: auto 1fr auto; gap: 16px; align-items: center; padding: 14px 18px; }
    .arquivo h3 { margin: 0; font-size: 1.02rem; } .arquivo p { margin: 2px 0 0; color: var(--nep-suave); font-size: .92rem; }
    .arquivo .icone { margin: 0; width: 44px; height: 44px; font-size: 1.3rem; }
`)
	}
	if g.tem("newsletter") {
		b.WriteString(`    .assinar { display: flex; gap: 8px; max-width: none; } .assinar input { flex: 1; min-width: 0; }
`)
	}
	b.WriteString(`    .preco-grande + * { margin-top: 0; }
`)
	return b.String()
}
