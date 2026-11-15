<?php

/**
 * @package     Mirabilis.Administrator
 * @subpackage  com_mirabilis
 *
 * Lê a descrição do site em português e decide o ramo, o nome, a cidade, a cor,
 * a fonte, as seções e o template. As regras (ramos, palavras, cores, estilos)
 * vêm do regras.json, exportado pela linguagem Nepenthes com
 * "nep criar --regras": o CMS e o nep decidem igual.
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

namespace Mirabilis\Component\Mirabilis\Administrator\Helper;

\defined('_JEXEC') or die;

final class Entender
{
    /** Template para cada ramo. */
    public const TEMPLATE_DO_RAMO = [
        'generico'    => 'rajah',
        'veterinaria' => 'lowii',
        'saude'       => 'lowii',
        'beleza'      => 'lowii',
        'restaurante' => 'truncata',
        'hotel'       => 'truncata',
        'escola'      => 'edwardsiana',
        'academia'    => 'rajah',
        'advocacia'   => 'rajah',
        'tecnologia'  => 'rajah',
        'loja'        => 'ampullaria',
        'associacao'  => 'edwardsiana',
        'comunidade'  => 'edwardsiana',
    ];

    public const TEMPLATES = [
        'rajah'       => ['Rajah', 'empresas, escritórios, consultorias e tecnologia', '#2f6b4f'],
        'villosa'     => ['Villosa', 'blogs, revistas e portais de notícia', '#9b2335'],
        'ampullaria'  => ['Ampullaria', 'lojas e catálogos', '#7c3aed'],
        'lowii'       => ['Lowii', 'saúde, bem-estar, beleza e pets', '#0f766e'],
        'truncata'    => ['Truncata', 'restaurantes, cafés, pousadas e hotéis', '#b45309'],
        'edwardsiana' => ['Edwardsiana', 'escolas, associações, ONGs e comunidades', '#1d4ed8'],
    ];

    /** Seções que o Joomla faz sozinho; as outras viram uma página simples e um aviso. */
    public const NATIVOS = ['servicos', 'catalogo', 'agenda', 'eventos', 'planos', 'equipe', 'galeria', 'depoimentos', 'blog', 'downloads', 'vagas', 'faq'];

    private static ?array $regras = null;

    public static function regras(): array
    {
        if (self::$regras === null) {
            self::$regras = json_decode((string) file_get_contents(\dirname(__DIR__, 2) . '/regras.json'), true) ?: [];
            // o blog do CMS não tem comentários sem extensão: não prometer
            self::$regras['nomesModulo']['blog'] = 'blog (categorias e tags)';
        }

        return self::$regras;
    }

    public static function semAcento(string $s): string
    {
        $s = mb_strtolower($s, 'UTF-8');

        return strtr($s, [
            'á' => 'a', 'à' => 'a', 'â' => 'a', 'ã' => 'a', 'ä' => 'a', 'é' => 'e', 'è' => 'e', 'ê' => 'e', 'ë' => 'e',
            'í' => 'i', 'ì' => 'i', 'î' => 'i', 'ï' => 'i', 'ó' => 'o', 'ò' => 'o', 'ô' => 'o', 'õ' => 'o', 'ö' => 'o',
            'ú' => 'u', 'ù' => 'u', 'û' => 'u', 'ü' => 'u', 'ç' => 'c', 'ñ' => 'n',
        ]);
    }

    /** Posições (em bytes) em que o radical começa uma palavra. "x$" = palavra inteira. */
    public static function achar(string $txt, string $chave): array
    {
        $inteira = str_ends_with($chave, '$');
        $chave   = rtrim($chave, '$');
        $p       = '/(?:^|[^\p{L}\p{N}])(' . preg_quote($chave, '/') . ')' . ($inteira ? '(?:$|[^\p{L}\p{N}])' : '') . '/u';
        $out     = [];

        if (preg_match_all($p, $txt, $m, PREG_OFFSET_CAPTURE)) {
            foreach ($m[1] as $x) {
                $out[] = $x[1];
            }
        }

        return $out;
    }

    private static function mencao(string $txt, array $chaves): array
    {
        $sim = $nao = 0;
        $neg = '/(?:^|[^\p{L}])(sem|nao|nem|exceto|menos|tirar|tire|tirando)\s+(?:(?:o|a|os|as|de|do|da|um|uma|quero|precisa|precisa de|preciso|preciso de|ter|nenhum|nenhuma)\s+){0,3}$/u';
        $enc = '/(?:^|[^\p{L}])(sem|nem)\s+[\p{L}-]+(?:\s+[\p{L}-]+)?\s+(?:e|ou|nem)\s+$/u';

        foreach ($chaves as $c) {
            foreach (self::achar($txt, $c) as $pos) {
                $ini   = max(0, $pos - 40);
                $antes = substr($txt, $ini, $pos - $ini);
                $antes = mb_convert_encoding($antes, 'UTF-8', 'UTF-8');
                $i     = max(strrpos($antes, ',') ?: -1, strrpos($antes, '.') ?: -1, strrpos($antes, ';') ?: -1, strrpos($antes, ':') ?: -1);

                if ($i >= 0) {
                    $antes = substr($antes, $i + 1);
                }

                if (preg_match($neg, $antes) || preg_match($enc, $antes)) {
                    $nao++;
                } else {
                    $sim++;
                }
            }
        }

        return [$sim, $nao];
    }

    private static function limparNome(string $s): string
    {
        $s = trim($s, " \"“”«»'");
        $s = preg_replace('/[{}<>\[\]|`\\\\]/u', '', $s);
        $s = mb_substr($s, 0, 50);

        if ($s === mb_strtolower($s)) {
            $ws = preg_split('/\s+/u', $s);

            foreach ($ws as $i => $w) {
                if ($i > 0 && \in_array($w, ['de', 'da', 'do', 'dos', 'das', 'e'], true)) {
                    continue;
                }
                $ws[$i] = mb_strtoupper(mb_substr($w, 0, 1)) . mb_substr($w, 1);
            }
            $s = implode(' ', $ws);
        }

        return $s;
    }

    /**
     * Entende a descrição.
     *
     * @return array{descricao:string,setor:array,nome:string,cidade:string,cor:string,fonte:string,modulos:array,pedidos:array,tirados:array,template:string,notas:array,avisos:array}
     */
    public static function plano(string $desc): array
    {
        $r    = self::regras();
        $desc = trim($desc);
        $txt  = self::semAcento($desc);
        $p    = ['descricao' => $desc, 'notas' => [], 'avisos' => [], 'cidade' => ''];

        // ramo
        $setores = $r['setores'];
        $melhor  = $setores[0];
        $pontos  = 0;

        foreach (\array_slice($setores, 1) as $s) {
            $n = 0;

            foreach ($s['Chaves'] ?? [] as $c) {
                $n += \count(self::achar($txt, $c));
            }

            if ($n > $pontos) {
                $melhor = $s;
                $pontos = $n;
            }
        }
        $p['setor']   = self::completar($melhor, $setores[0]);
        $p['notas'][] = $pontos > 0 ? 'Ramo: ' . $melhor['Rotulo'] : 'Ramo: não reconheci um ramo específico, usei o modelo de empresa';

        // nome
        $nome = '';

        if (preg_match('/\b(?:chamad[oa]s?|de nome|com o nome|nomead[oa]|nome)\s*:?\s+(.{2,60}?)(?:\s+(?:em|no|na|nos|nas|com|que|para|pra|onde|e|sem|so|só|apenas|cor|de cor)\s|[,.;!?(]|$)/iu', $desc, $m)) {
            $nome = self::limparNome($m[1]);
        }

        if ($nome === '' && preg_match('/["“”«»\']([^"“”«»\']{2,60})["“”«»\']/u', $desc, $m)) {
            $nome = self::limparNome($m[1]);
        }

        if ($nome === '' && preg_match("/^(?:(?:uma?|o|a|site d[aeo]|um site d[aeo]|site para|um site para)\\s+)*((?:[\\p{Ll}\\p{Lu}]+\\s+){1,4}(?:de|do|da|dos|das)\\s+\\p{Lu}[\\p{L}'-]*(?:\\s+(?:de|do|da|dos|das)?\\s*\\p{Lu}[\\p{L}'-]*){0,3})/u", $desc, $m)) {
            $n    = trim($m[1]);
            $nome = self::limparNome(mb_strtoupper(mb_substr($n, 0, 1)) . mb_substr($n, 1));
        }

        if ($nome === '') {
            $nome = self::nomeNoMeio($desc);
        }

        if ($nome === '') {
            $nome         = $p['setor']['Nome'];
            $p['notas'][] = 'Nome: não achei, usei "' . $nome . '" (troque depois ou escreva "chamada X")';
        } else {
            $p['notas'][] = 'Nome: ' . $nome;
        }
        $p['nome'] = $nome;

        // cidade
        if (preg_match_all("/\\b(?:em|de)\\s+((?:São|Santo|Santa|Rio|Belo|Porto|Campo|Vila|Nova|Novo|Foz|Juiz|Ribeirão|Feira|Praia|[A-ZÁÉÍÓÚÂÊÔÃÕÇ])[\\p{L}'-]*(?:\\s+(?:do|da|de|dos|das)?\\s*[A-ZÁÉÍÓÚÂÊÔÃÕÇ][\\p{L}'-]*){0,3})/u", $desc, $mm, PREG_SET_ORDER)) {
            foreach ($mm as $m) {
                $c = trim($m[1]);

                if ($c === '' || str_contains($nome, $c)) {
                    continue;
                }
                $p['cidade']  = $c;
                $p['notas'][] = 'Cidade: ' . $c;
                break;
            }
        }

        // módulos: com / só / sem
        $pedidos = $tirados = [];

        foreach ($r['modulos'] as $mod) {
            [$sim, $nao] = self::mencao($txt, $r['chavesModulo'][$mod] ?? []);

            if ($nao > 0) {
                $tirados[$mod] = true;
            } elseif ($sim > 0) {
                $pedidos[$mod] = true;
            }
        }
        $so        = (bool) preg_match('/(?:^|[^\p{L}])(so|somente|apenas|unicamente)\s/u', $txt);
        $soContato = $so && !$pedidos && self::achar($txt, 'contato');
        $escolhe   = [];

        if ((!$so || !$pedidos) && !$soContato) {
            foreach ($p['setor']['Modulos'] as $mod) {
                $escolhe[$mod] = true;
            }
        }
        $escolhe += $pedidos;

        foreach ($tirados as $mod => $_) {
            unset($escolhe[$mod]);
        }
        $p['modulos'] = $p['pedidos'] = $p['tirados'] = [];

        foreach ($r['modulos'] as $mod) {
            if (!empty($escolhe[$mod])) {
                $p['modulos'][] = $mod;
            }
            if (!empty($pedidos[$mod]) && empty($tirados[$mod])) {
                $p['pedidos'][] = $mod;
            }
            if (!empty($tirados[$mod])) {
                $p['tirados'][] = $mod;
            }
        }

        if ($so && ($pedidos || $soContato)) {
            $p['notas'][] = '"Só": deixei apenas o que você pediu (mais contato e páginas)';
        }

        // template: o do ramo; blog sem outro ramo vira revista; nome de template escrito vale mais
        $tpl = self::TEMPLATE_DO_RAMO[$p['setor']['ID']] ?? 'rajah';

        if ($p['setor']['ID'] === 'generico' && (self::achar($txt, 'blog') || self::achar($txt, 'revista') || self::achar($txt, 'jornal') || self::achar($txt, 'noticia') || self::achar($txt, 'portal'))) {
            $tpl = 'villosa';
        }

        foreach (array_keys(self::TEMPLATES) as $t) {
            if (self::achar($txt, $t)) {
                $tpl = $t;
            }
        }
        $p['template'] = $tpl;

        // ramo não reconhecido: a capa usa o que a pessoa escreveu, sem números inventados
        if ($tpl !== 'villosa' && $p['setor']['ID'] === 'generico') {
            $antes = trim((string) strstr($desc, $p['nome'], true), " \t,.;:-");
            $p['setor']['Numeros'] = [];

            if ($antes !== '' && mb_strlen($antes) <= 40 && str_contains($desc, $p['nome'])) {
                $p['setor']['Chapeu'] = $antes;
                $p['setor']['Titulo'] = $p['nome'];
            }
        }

        // revista/blog sem outro ramo: a capa fala de leitura e a página é das publicações
        if ($tpl === 'villosa' && $p['setor']['ID'] === 'generico') {
            $fica = ['blog' => true];

            foreach ($p['pedidos'] as $mod) {
                $fica[$mod] = true;
            }
            $p['modulos'] = array_values(array_filter($r['modulos'], static fn ($m) => !empty($fica[$m]) && !\in_array($m, $p['tirados'], true)));
            $p['setor']   = array_merge($p['setor'], [
                'Rotulo'     => 'revista / blog',
                'Emoji'      => '📰',
                'Chapeu'     => self::achar($txt, 'blog') && !self::achar($txt, 'revista') ? 'Blog' : 'Revista',
                'Titulo'     => 'Histórias, ideias e boas leituras',
                'Texto'      => 'Textos novos toda semana: reportagens, ensaios e notas curtas, escritos com cuidado para quem gosta de ler.',
                'Chamada'    => 'Tem uma pauta ou quer escrever com a gente?',
                'BlogRotulo' => 'Publicações',
                'Numeros'    => [],
                'Cats'       => ['Cultura', 'Ensaios', 'Notas'],
                'Posts'      => [
                    ['Titulo' => 'O que a cidade lê quando ninguém está olhando', 'Cat' => 'Cultura', 'Resumo' => 'Uma volta pelas bibliotecas de bairro, sebos e clubes de leitura que mantêm o hábito vivo.', 'Tags' => 'leitura, cidade'],
                    ['Titulo' => 'Elogio da conversa demorada', 'Cat' => 'Ensaios', 'Resumo' => 'Por que vale a pena desligar o telefone e escutar até o fim.', 'Tags' => 'ensaio, cotidiano'],
                    ['Titulo' => 'Cinco discos para ouvir inteiros neste mês', 'Cat' => 'Notas', 'Resumo' => 'Uma seleção curta, do samba ao jazz, para ouvir do começo ao fim.', 'Tags' => 'música, lista'],
                ],
            ]);
        }
        $p['notas'][]  = 'Template: ' . self::TEMPLATES[$tpl][0] . ' (feito para ' . self::TEMPLATES[$tpl][1] . ')';

        // cor
        $cor = $p['setor']['ID'] === 'generico' ? self::TEMPLATES[$tpl][2] : $p['setor']['Cor'];

        if (preg_match('/#[0-9a-fA-F]{6}\b/', $desc, $m)) {
            $cor          = strtolower($m[0]);
            $p['notas'][] = 'Cor: ' . $cor;
        } else {
            $achou = false;

            foreach ($r['cores'] as $c) {
                if (self::achar($txt, $c['Chave'])) {
                    $cor          = $c['Hex'];
                    $p['notas'][] = 'Cor: ' . $c['Nome'];
                    $achou        = true;
                    break;
                }
            }

            if (!$achou) {
                $p['notas'][] = 'Cor: a do ramo (' . $cor . ')';
            }
        }
        $p['cor'] = $cor;

        // estilo -> fonte
        $p['fonte'] = $p['setor']['Fonte'];

        foreach ($r['estilos'] as $e) {
            if (self::achar($txt, $e['Chave'])) {
                $p['fonte']   = $e['Fonte'];
                $p['notas'][] = 'Estilo: ' . $e['Nome'];
                break;
            }
        }

        foreach ($p['modulos'] as $mod) {
            if (!\in_array($mod, self::NATIVOS, true) && \in_array($mod, $p['pedidos'], true)) {
                $p['avisos'][] = ucfirst($r['nomesModulo'][$mod] ?? $mod) . ': o Mirabilis não faz isso sozinho; instale uma extensão ou use um site Nepenthes (.nep.html), que já tem.';
            }
        }

        return $p;
    }

    /**
     * Nome próprio no meio da frase: "Escritório de advocacia Duarte & Lemos em
     * Belo Horizonte" -> "Duarte & Lemos". Igual a nomeNoMeio do Nepenthes.
     */
    private static function nomeNoMeio(string $desc): string
    {
        $re = "/(\\p{Lu}[\\p{L}'’-]*(?:\\s+(?:&|e|de|da|do|dos|das)\\s+\\p{Lu}[\\p{L}'’-]*|\\s+\\p{Lu}[\\p{L}'’-]*){0,4})(?:\\s+(?:em|no|na|com|so|só|sem|de cor|cor)\\b|\\s*[,.;!?(]|\\s*$)/u";

        if (!preg_match_all($re, $desc, $mm, PREG_SET_ORDER | PREG_OFFSET_CAPTURE)) {
            return '';
        }

        foreach ($mm as $m) {
            [$txt, $ini] = $m[1];
            $fim         = $ini + \strlen($txt);

            if ($ini === 0) { // a 1ª palavra é maiúscula só por abrir a frase
                $sp = strpos(substr($desc, 0, $fim), ' ');

                if ($sp === false) {
                    continue;
                }
                $ini = $sp + 1;
            }
            $antes = preg_split('/\\s+/', trim(self::semAcento(substr($desc, 0, $ini))));
            $ult   = end($antes);

            if (\in_array($ult, ['em', 'no', 'na', 'de', 'da', 'do', 'para', 'pra', 'cor', 'estilo'], true)) {
                continue;
            }
            $n = self::limparNome(substr($desc, $ini, $fim - $ini));

            if ($n !== '') {
                return $n;
            }
        }

        return '';
    }

    private static function completar(array $s, array $g): array
    {
        foreach (['Servicos', 'Equipe', 'Posts', 'Produtos', 'Faq', 'Planos', 'Depo', 'Eventos', 'Galeria', 'Numeros'] as $k) {
            if (empty($s[$k])) {
                $s[$k] = $g[$k];
            }
        }

        foreach (['ServRotulo', 'ServIntro', 'EquipeRotulo', 'BlogRotulo', 'LojaRotulo', 'AgendaVerbo', 'AgendaTitulo'] as $k) {
            if (empty($s[$k])) {
                $s[$k] = $g[$k];
            }
        }

        if (empty($s['Cats'])) {
            $s['Cats'] = $g['Cats'];
        }

        return $s;
    }
}
