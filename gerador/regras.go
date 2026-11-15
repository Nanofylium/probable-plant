package gerador

import "encoding/json"

// RegrasJSON exporta as regras do nep criar (ramos, palavras, cores, estilos)
// para outros programas usarem as mesmas decisões, como o CMS Mirabilis.
func RegrasJSON() ([]byte, error) {
	type cor struct{ Chave, Hex, Nome string }
	type estilo struct{ Chave, Fonte, Nome string }
	var cs []cor
	for _, c := range cores {
		cs = append(cs, cor{c.chave, c.hex, c.nome})
	}
	var es []estilo
	for _, e := range estilos {
		es = append(es, estilo{e.chave, e.fonte, e.nome})
	}
	return json.MarshalIndent(map[string]any{
		"versao":        1,
		"setores":       append([]Setor{generico}, setores...),
		"chavesModulo":  chavesModulo,
		"modulos":       ModulosTodos,
		"nomesModulo":   NomeModulo,
		"cores":         cs,
		"estilos":       es,
		"codigosIdioma": codIdioma,
	}, "", " ")
}
