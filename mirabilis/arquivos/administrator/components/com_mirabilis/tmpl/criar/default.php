<?php

/**
 * @package     Mirabilis.Administrator
 * @subpackage  com_mirabilis
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

\defined('_JEXEC') or die;

use Joomla\CMS\HTML\HTMLHelper;
use Joomla\CMS\Uri\Uri;

/** @var \Mirabilis\Component\Mirabilis\Administrator\View\Criar\HtmlView $this */

$e     = static fn ($s) => htmlspecialchars((string) $s, ENT_QUOTES, 'UTF-8');
$p     = $this->plano;
$rnome = $p ? (\Mirabilis\Component\Mirabilis\Administrator\Helper\Entender::regras()['nomesModulo'] ?? []) : [];
$exemplos = [
    'Clínica veterinária chamada Pata Amiga em Curitiba, com agenda e equipe, cor verde',
    'Restaurante italiano "Nonna Rosa" com cardápio, galeria e reservas, estilo elegante',
    'Blog de viagens chamado Mapa Aberto, só blog e contato',
    'Loja de roupas Ateliê Ipê com catálogo, depoimentos e perguntas frequentes, cor roxa',
    'Escola de música em Recife com eventos, planos e equipe',
];
?>
<div class="mira-painel">
	<form action="index.php?option=com_mirabilis" method="post" id="adminForm" name="adminForm">
		<div class="mira-p-topo">
			<h2>Diga com suas palavras como quer o site</h2>
			<p>O Mirabilis lê o que você escreveu, escolhe o melhor template e monta páginas, menus, contatos e a página inicial. Use <b>com</b> para acrescentar uma seção, <b>sem</b> para tirar e <b>só</b> para deixar apenas o que disser.</p>
			<label for="mira-descricao" class="visually-hidden">Descrição do site</label>
			<textarea id="mira-descricao" name="descricao" rows="4" class="form-control" placeholder="Ex.: Pousada chamada Maré Mansa em Paraty, com quartos, galeria, depoimentos e reservas, cor azul"><?php echo $e($this->descricao); ?></textarea>
			<div class="mira-p-exemplos">
				<span>Exemplos:</span>
				<?php foreach ($exemplos as $x) : ?>
					<button type="button" class="btn btn-sm btn-outline-secondary" onclick="document.getElementById('mira-descricao').value=this.textContent"><?php echo $e($x); ?></button>
				<?php endforeach; ?>
			</div>
			<div class="mira-p-acoes">
				<button type="submit" class="btn btn-secondary" onclick="this.form.task.value='planejar'">Ver o plano</button>
				<button type="submit" class="btn btn-success" onclick="this.form.task.value='montar'"><?php echo $this->montado ? 'Montar e trocar o site atual' : 'Montar o site'; ?></button>
				<?php if ($this->montado) : ?>
					<input type="hidden" name="refazer" value="1">
					<a class="btn btn-outline-primary" href="<?php echo $e($this->site); ?>" target="_blank" rel="noopener">Abrir o site</a>
				<?php endif; ?>
			</div>
			<?php if ($this->montado) : ?>
				<p class="mira-p-nota">Este site já foi montado pelo Mirabilis. Montar de novo apaga só o conteúdo de exemplo que ele criou e põe o novo; o que você escreveu à mão fica.</p>
			<?php endif; ?>
		</div>

		<?php if ($p) : ?>
			<div class="mira-p-plano">
				<h3>O que eu entendi</h3>
				<dl>
					<dt>Nome</dt><dd><?php echo $e($p['nome']); ?></dd>
					<dt>Ramo</dt><dd><?php echo $e($p['setor']['Rotulo']); ?></dd>
					<?php if ($p['cidade']) : ?><dt>Cidade</dt><dd><?php echo $e($p['cidade']); ?></dd><?php endif; ?>
					<dt>Template</dt><dd><b><?php echo $e($this->templates[$p['template']][0]); ?></b> (feito para <?php echo $e($this->templates[$p['template']][1]); ?>)</dd>
					<dt>Cor</dt><dd><span class="mira-p-cor" style="background:<?php echo $e($p['cor']); ?>"></span> <?php echo $e($p['cor']); ?></dd>
					<dt>Seções</dt><dd><?php echo $e(implode(', ', array_map(static fn ($m) => $rnome[$m] ?? $m, $p['modulos'])) ?: 'só a página inicial e o contato'); ?></dd>
				</dl>
				<?php if ($p['notas']) : ?>
					<ul class="mira-p-notas"><?php foreach ($p['notas'] as $n) : ?><li><?php echo $e($n); ?></li><?php endforeach; ?></ul>
				<?php endif; ?>
				<?php foreach ($p['avisos'] as $a) : ?>
					<div class="alert alert-warning"><?php echo $e($a); ?></div>
				<?php endforeach; ?>
			</div>
		<?php endif; ?>

		<h3 class="mira-p-titulo">Os seis templates do Mirabilis</h3>
		<p>Você não precisa escolher: a descrição decide. Mas se escrever o nome de um deles (por exemplo "template truncata"), ele é usado.</p>
		<div class="mira-p-galeria">
			<?php foreach ($this->templates as $id => [$nome, $para, $cor]) : ?>
				<figure class="<?php echo $p && $p['template'] === $id ? 'escolhido' : ''; ?>">
					<img src="<?php echo Uri::root(true); ?>/media/templates/site/mirabilis_<?php echo $id; ?>/images/template_thumbnail.png" alt="" loading="lazy">
					<figcaption><b><?php echo $e($nome); ?></b><span><?php echo $e($para); ?></span></figcaption>
				</figure>
			<?php endforeach; ?>
		</div>

		<input type="hidden" name="task" value="">
		<?php echo HTMLHelper::_('form.token'); ?>
	</form>
</div>
