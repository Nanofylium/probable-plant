<?php

/**
 * @package     Mirabilis.Site
 * @subpackage  Templates
 *
 * Parte comum dos templates Mirabilis (rajah, villosa, ampullaria, lowii, truncata,
 * edwardsiana). Cada template é filho do Cassiopeia, o template livre do Joomla:
 * usa o HTML dele e troca o visual (cores, fontes, cabeçalho, cartões, rodapé).
 *
 * Incluído pelo index.php do template, dentro do contexto do documento ($this).
 *
 * @copyright   (C) 2026 Mirabilis. Derivado do Cassiopeia, (C) 2017 Open Source Matters, Inc.
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

defined('_JEXEC') or die;

/** @var Joomla\CMS\Document\HtmlDocument $this */

$miraDesenho = $miraDesenho ?? 'rajah';

ob_start();
require JPATH_THEMES . '/cassiopeia/index.php';
$miraHtml = ob_get_clean();

$miraCor = (string) $this->params->get('cor', '#2f6b4f');
if (!preg_match('/^#[0-9a-fA-F]{6}$/', $miraCor)) {
    $miraCor = '#2f6b4f';
}

$miraFontes = [
    'sem-serifa'    => ['system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif', ''],
    'humanista'     => ['"Segoe UI", "Gill Sans", "Trebuchet MS", Calibri, system-ui, sans-serif', ''],
    'geometrica'    => ['"Avenir Next", Avenir, Montserrat, "Century Gothic", "Segoe UI", system-ui, sans-serif', ''],
    'arredondada'   => ['"Nunito", "Varela Round", "Arial Rounded MT Bold", "Segoe UI", system-ui, sans-serif', ''],
    'serifa'        => ['Charter, "Bitstream Charter", "Sitka Text", Cambria, Georgia, serif', ''],
    'titulo-serifa' => ['system-ui, -apple-system, "Segoe UI", Roboto, Arial, sans-serif', '"Iowan Old Style", "Palatino Linotype", Palatino, "Book Antiqua", Georgia, serif'],
    'classica'      => ['Georgia, Cambria, "Times New Roman", serif', '"Didot", "Bodoni MT", "Playfair Display", Georgia, serif'],
];
[$miraCorpo, $miraTitulo] = $miraFontes[$this->params->get('fonte', '')] ?? $miraFontes[$miraDesenho === 'villosa' || $miraDesenho === 'truncata' ? 'titulo-serifa' : 'sem-serifa'];
$miraTitulo = $miraTitulo ?: $miraCorpo;

$wa = $this->getWebAssetManager();
$wa->registerAndUseStyle('mirabilis.base', 'com_mirabilis/base.css', ['version' => 'auto'], [], ['template.active'])
    ->addInlineStyle(':root {
        --mira-cor: ' . $miraCor . ';
        --cassiopeia-color-primary: ' . $miraCor . ';
        --cassiopeia-color-link: ' . $miraCor . ';
        --cassiopeia-color-hover: color-mix(in srgb, ' . $miraCor . ' 70%, #000);
        --link-color: ' . $miraCor . ';
        --link-hover-color: color-mix(in srgb, ' . $miraCor . ' 70%, #000);
        --cassiopeia-font-family-body: ' . $miraCorpo . ';
        --cassiopeia-font-family-headings: ' . $miraTitulo . ';
        --mira-fonte-titulo: ' . $miraTitulo . ';
    }');

// marca o corpo com o desenho, para o CSS do template
echo preg_replace('/<body([^>]*)class="site /', '<body$1class="site mira mira-' . $miraDesenho . ' ', $miraHtml, 1);
