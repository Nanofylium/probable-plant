package moldes

import (
	"strings"
	"testing"
)

func TestLerTag(t *testing.T) {
	p, ok := LerTag([]byte(`<p>oi</p><nepenthes tipo='portal' nome="Folha &amp; Cia" e-mail=a@b.com instagram="@folha">
	  Portal <b>da</b> cidade.
	</nepenthes>`))
	if !ok || p.Tipo != "portal" || p.Nome != "Folha & Cia" || p.Email != "a@b.com" || p.Instagram != "folha" || p.Descricao != "Portal da cidade." {
		t.Fatalf("%+v %v", p, ok)
	}
	if _, ok := LerTag([]byte(`<nepenthes>uma padaria</nepenthes>`)); ok {
		t.Fatal("sem atributos é o assistente")
	}
	if _, ok := LerTag([]byte(`<nepenthes data-x="1">uma padaria</nepenthes>`)); ok {
		t.Fatal("atributo desconhecido não pede molde")
	}
}

func TestDeduzir(t *testing.T) {
	for desc, quer := range map[string]string{
		"Documentação do nosso software": "wiki", "Jornal de notícias da cidade": "portal",
		"Meu blog de viagens": "blog", "Padaria artesanal no centro": "estatico",
	} {
		if got := Deduzir(desc); got != quer {
			t.Errorf("%q: %s, queria %s", desc, got, quer)
		}
	}
}

func TestMontarEscapaOQueVemDaTag(t *testing.T) {
	out, err := Montar(Pedido{Tipo: "blog", Nome: `Oi {{ db.all("x") }} <script>`, Descricao: `"aspas" e \barra`})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `nome="Oi {{`) || strings.Contains(out, "Oi {{ db.all(\"x\") }} <script>.") || !strings.Contains(out, `"Equipe Oi {{ db.all(\"x\") }} <script>"`) {
		t.Fatal("o nome entrou sem escape")
	}
}
