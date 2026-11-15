<?php

/**
 * @package     Mirabilis.Administrator
 * @subpackage  com_mirabilis
 *
 * Monta o site a partir do plano do Entender: escolhe e configura o template,
 * cria categorias, artigos, contatos, menus e os blocos da página inicial,
 * tudo com os modelos do próprio CMS (o mesmo caminho do painel). Guarda o que
 * criou para poder desfazer ("refazer" troca o site inteiro por um novo).
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

namespace Mirabilis\Component\Mirabilis\Administrator\Helper;

use Joomla\CMS\Application\ApplicationHelper;
use Joomla\CMS\Component\ComponentHelper;
use Joomla\CMS\Factory;
use Joomla\CMS\Helper\ModuleHelper;
use Joomla\CMS\Table\Table;
use Joomla\Database\DatabaseInterface;
use Joomla\Registry\Registry;

\defined('_JEXEC') or die;

final class Construtor
{
    private array $criados = ['artigos' => [], 'categorias' => [], 'contatos' => [], 'itens' => [], 'modulos' => [], 'estilos' => [], 'menus' => []];
    private array $p;
    private array $s;
    private int $home  = 0;
    private int $ordem = 0;
    private array $links = [];

    public function __construct(private $app, private DatabaseInterface $db)
    {
    }

    public static function jaMontado(): bool
    {
        return !empty(ComponentHelper::getParams('com_mirabilis')->get('criados'));
    }

    /** Monta o site. Devolve um resumo do que foi feito. */
    public function aplicar(array $p, bool $refazer = false): array
    {
        if (self::jaMontado()) {
            if (!$refazer) {
                throw new \RuntimeException('Este site já foi montado. Use "Refazer" para trocar pelo novo (o conteúdo de exemplo anterior vai para o lixo).');
            }
            $this->desfazer();
        }

        $this->p = $p;
        $this->s = $p['setor'];
        $this->app->getLanguage()->setDebug(false);

        $this->home = (int) $this->db->setQuery(
            $this->db->getQuery(true)->select('id')->from('#__menu')->where('home = 1')->where('client_id = 0')
        )->loadResult();

        $antes = $this->maiores();

        try {
            $this->estilo();
            $this->menus();
            $this->conteudo();
            $this->blocos();
            $this->nomeDoSite();
            $this->guardar();
        } catch (\Throwable $e) {
            // não deixa meio site para trás: guarda o que já criou e desfaz,
            // inclusive o que um modelo gravou antes de falhar (ainda sem id conhecido)
            try {
                foreach ($antes as $tabela => $max) {
                    $ids = $this->db->setQuery($this->db->getQuery(true)->select('id')->from($tabela)->where('id > ' . $max))->loadColumn();
                    $chave = ['#__content' => 'artigos', '#__categories' => 'categorias', '#__contact_details' => 'contatos', '#__menu' => 'itens', '#__modules' => 'modulos'][$tabela];
                    $this->criados[$chave] = array_values(array_unique(array_merge($this->criados[$chave], array_map('intval', $ids))));
                }
                $this->guardar();
                $this->desfazer();
            } catch (\Throwable) {
            }

            throw $e;
        }

        return ['criados' => array_map('count', $this->criados), 'template' => $p['template']];
    }

    /** Maior id de cada tabela antes de montar. */
    private function maiores(): array
    {
        $r = [];
        foreach (['#__content', '#__categories', '#__contact_details', '#__menu', '#__modules'] as $t) {
            $r[$t] = (int) $this->db->setQuery($this->db->getQuery(true)->select('MAX(id)')->from($t))->loadResult();
        }

        return $r;
    }

    // ------------------------------------------------------------------ template

    private function estilo(): void
    {
        $p      = $this->p;
        $params = [
            'cor' => $p['cor'], 'fonte' => $p['fonte'], 'brand' => '1', 'logoFile' => '',
            'siteTitle' => $p['nome'], 'siteDescription' => $p['cidade'] ? $p['setor']['Rotulo'] === 'empresa / negócio' ? $p['cidade'] : ucfirst(explode(' / ', $p['setor']['Rotulo'])[0]) . ' em ' . $p['cidade'] : '',
            'fluidContainer' => '0', 'stickyHeader' => 1, 'backTop' => 1,
        ];
        $this->db->setQuery($this->db->getQuery(true)->update('#__template_styles')->set($this->db->quoteName('home') . ' = ' . $this->db->quote('0'))->where('client_id = 0'))->execute();
        $o = (object) [
            'template' => 'mirabilis_' . $p['template'], 'client_id' => 0, 'home' => '1', 'inheritable' => 0, 'parent' => 'cassiopeia',
            'title'    => \Mirabilis\Component\Mirabilis\Administrator\Helper\Entender::TEMPLATES[$p['template']][0] . ' - ' . $p['nome'],
            'params'   => json_encode($params),
        ];
        $this->db->insertObject('#__template_styles', $o, 'id');
        $this->criados['estilos'][] = (int) $o->id;
    }

    // ------------------------------------------------------------------ menus

    private function menus(): void
    {
        // menu do rodapé
        $t = Table::getInstance('MenuType', 'Joomla\\CMS\\Table\\', ['dbo' => $this->db]);
        $t->bind(['menutype' => 'mirabilis-rodape', 'title' => 'Rodapé', 'description' => 'Links do rodapé (Mirabilis)', 'client_id' => 0]);
        if ($t->check() && $t->store()) {
            $this->criados['menus'][] = (int) $t->id;
        }

        // a página inicial passa a se chamar Início
        if ($this->home) {
            $this->db->setQuery($this->db->getQuery(true)->update('#__menu')->set('title = ' . $this->db->quote('Início'))->where('id = ' . $this->home))->execute();
        }

        // menus padrão da instalação: tira o menu lateral e o login da frente
        $this->db->setQuery($this->db->getQuery(true)->update('#__modules')->set('published = 0')
            ->where('client_id = 0')->where($this->db->quoteName('module') . ' IN (' . $this->db->quote('mod_menu') . ',' . $this->db->quote('mod_login') . ')')
            ->where('position = ' . $this->db->quote('sidebar-right')))->execute();

        // trilha de navegação: começa em "Início" e sem o "Você está aqui" em inglês
        $q = $this->db->getQuery(true)->select('id, params')->from('#__modules')->where('client_id = 0')
            ->where($this->db->quoteName('module') . ' = ' . $this->db->quote('mod_breadcrumbs'));
        foreach ($this->db->setQuery($q)->loadObjectList() as $b) {
            $r = new Registry($b->params);
            $r->set('showHere', 0);
            $r->set('showHome', 1);
            $r->set('homeText', 'Início');
            $this->db->setQuery($this->db->getQuery(true)->update('#__modules')->set('params = ' . $this->db->quote($r->toString()))->where('id = ' . (int) $b->id))->execute();
        }

        // artigos sem a caixa de detalhes (autor, categoria, data, acessos), como num site de empresa
        $q = $this->db->getQuery(true)->select('extension_id, params')->from('#__extensions')
            ->where($this->db->quoteName('element') . ' = ' . $this->db->quote('com_content'))->where('type = ' . $this->db->quote('component'));
        if ($c = $this->db->setQuery($q)->loadObject()) {
            $r = new Registry($c->params);
            foreach (['show_author', 'show_category', 'show_parent_category', 'show_create_date', 'show_modify_date', 'show_publish_date', 'show_hits', 'info_block_show_title', 'show_readmore_title', 'show_icons', 'show_print_icon', 'show_email_icon'] as $k) {
                $r->set($k, '0');
            }
            $this->db->setQuery($this->db->getQuery(true)->update('#__extensions')->set('params = ' . $this->db->quote($r->toString()))->where('extension_id = ' . (int) $c->extension_id))->execute();
        }
    }

    private function item(string $titulo, string $link, array $params = [], string $menutype = 'mainmenu', string $tipo = 'component'): int
    {
        $model = $this->app->bootComponent('com_menus')->getMVCFactory()->createModel('Item', 'Administrator', ['ignore_request' => true]);
        $comp  = 0;

        if ($tipo === 'component' && preg_match('/option=(com_\w+)/', $link, $m)) {
            $comp = (int) \Joomla\CMS\Extension\ExtensionHelper::getExtensionRecord($m[1], 'component')->extension_id;
        }
        $alias = ApplicationHelper::stringURLSafe($titulo) ?: 'item';
        $dados = [
            'id' => 0, 'menutype' => $menutype, 'title' => $titulo, 'alias' => $alias, 'link' => $link, 'type' => $tipo,
            'component_id' => $comp, 'published' => 1, 'parent_id' => 1, 'level' => 1, 'language' => '*', 'access' => 1,
            'browserNav' => 0, 'template_style_id' => 0, 'home' => 0, 'client_id' => 0, 'note' => '', 'img' => '', 'associations' => [],
            'params' => $params + ['show_page_heading' => 1, 'menu_show' => 1],
        ];

        for ($i = 0; $i < 4; $i++) {
            $model->setState('item.id', 0);
            if ($model->save($dados)) {
                $id                        = (int) $model->getState('item.id');
                $this->criados['itens'][] = $id;

                return $id;
            }
            $dados['alias'] = $alias . '-' . ($i + 2);
        }

        throw new \RuntimeException('Menu "' . $titulo . '": ' . $model->getError());
    }

    // ------------------------------------------------------------------ conteúdo

    private function categoria(string $titulo, string $desc = ''): int
    {
        $model = $this->app->bootComponent('com_categories')->getMVCFactory()->createModel('Category', 'Administrator', ['ignore_request' => true]);
        $alias = ApplicationHelper::stringURLSafe($titulo);
        $dados = ['id' => 0, 'title' => $titulo, 'alias' => $alias, 'parent_id' => 1, 'extension' => 'com_content', 'published' => 1, 'access' => 1,
            'language' => '*', 'description' => $desc, 'params' => '{}', 'associations' => [], 'level' => 1];

        return $this->salvarCategoria($model, $dados, 'categorias');
    }

    private function salvarCategoria($model, array $dados, string $onde): int
    {
        $alias = $dados['alias'];

        for ($i = 0; $i < 4; $i++) {
            $model->setState('category.id', 0);
            if ($model->save($dados)) {
                $id                     = (int) $model->getState('category.id');
                $this->criados[$onde][] = $id;

                return $id;
            }
            $dados['alias'] = $alias . '-' . ($i + 2);
        }

        throw new \RuntimeException('Categoria "' . $dados['title'] . '": ' . $model->getError());
    }

    private function artigo(string $titulo, string $intro, string $resto, int $cat, bool $destaque = false, array $tags = []): int
    {
        $model = $this->app->bootComponent('com_content')->getMVCFactory()->createModel('Article', 'Administrator', ['ignore_request' => true]);
        $alias = ApplicationHelper::stringURLSafe($titulo);
        $dados = [
            'id' => 0, 'title' => $titulo, 'alias' => $alias, 'introtext' => $intro, 'fulltext' => $resto, 'catid' => $cat,
            'state' => 1, 'featured' => $destaque ? 1 : 0, 'language' => '*', 'access' => 1, 'images' => '', 'urls' => '',
            'metakey' => '', 'metadesc' => strip_tags($intro), 'associations' => [], 'created_by_alias' => '',
            'tags' => array_map(fn ($t) => '#new#' . $t, $tags),
        ];

        for ($i = 0; $i < 4; $i++) {
            $model->setState('article.id', 0);
            if ($model->save($dados)) {
                $id                          = (int) $model->getState('article.id');
                $this->criados['artigos'][] = $id;

                // o fluxo de publicação põe todo artigo novo na página inicial; só os destaques ficam
                if (!$destaque) {
                    $this->db->setQuery($this->db->getQuery(true)->delete('#__content_frontpage')->where('content_id = ' . $id))->execute();
                    $this->db->setQuery($this->db->getQuery(true)->update('#__content')->set('featured = 0')->where('id = ' . $id))->execute();
                }

                return $id;
            }
            $dados['alias'] = $alias . '-' . ($i + 2);
        }

        throw new \RuntimeException('Artigo "' . $titulo . '": ' . $model->getError());
    }

    private function semCategoria(): int
    {
        return (int) $this->db->setQuery($this->db->getQuery(true)->select('id')->from('#__categories')
            ->where('extension = ' . $this->db->quote('com_content'))->where('alias = ' . $this->db->quote('uncategorised')))->loadResult() ?: 2;
    }

    private function contato(string $nome, string $cargo, string $texto, int $cat, string $email = '', bool $destaque = false): int
    {
        $model = $this->app->bootComponent('com_contact')->getMVCFactory()->createModel('Contact', 'Administrator', ['ignore_request' => true]);
        $alias = ApplicationHelper::stringURLSafe($nome);
        $dados = [
            'id' => 0, 'name' => $nome, 'alias' => $alias, 'con_position' => $cargo, 'misc' => '<p>' . self::e($texto) . '</p>',
            'catid' => $cat, 'published' => 1, 'access' => 1, 'language' => '*', 'email_to' => $email, 'featured' => $destaque ? 1 : 0,
            'params' => ['show_email_form' => $email ? 1 : 0], 'associations' => [], 'address' => '', 'suburb' => $this->p['cidade'], 'telephone' => '',
        ];

        for ($i = 0; $i < 4; $i++) {
            $model->setState('contact.id', 0);
            if ($model->save($dados)) {
                $id                           = (int) $model->getState('contact.id');
                $this->criados['contatos'][] = $id;

                return $id;
            }
            $dados['alias'] = $alias . '-' . ($i + 2);
        }

        throw new \RuntimeException('Contato "' . $nome . '": ' . $model->getError());
    }

    private function catContato(string $titulo): int
    {
        $model = $this->app->bootComponent('com_categories')->getMVCFactory()->createModel('Category', 'Administrator', ['ignore_request' => true]);

        return $this->salvarCategoria($model, ['id' => 0, 'title' => $titulo, 'alias' => ApplicationHelper::stringURLSafe($titulo), 'parent_id' => 1,
            'extension' => 'com_contact', 'published' => 1, 'access' => 1, 'language' => '*', 'description' => '', 'params' => '{}', 'associations' => [], 'level' => 1], 'categorias');
    }

    private static function e(string $s): string
    {
        return htmlspecialchars($s, ENT_QUOTES, 'UTF-8');
    }

    private function tem(string $m): bool
    {
        return \in_array($m, $this->p['modulos'], true);
    }

    private function conteudo(): void
    {
        $s    = $this->s;
        $p    = $this->p;
        $nome = $p['nome'];
        $semC = $this->semCategoria();

        // e-mail de quem administra, para os formulários de contato
        $email = (string) $this->app->get('mailfrom');

        // ---- serviços
        if ($this->tem('servicos')) {
            $cat = $this->categoria($s['ServRotulo'], '<p>' . self::e($s['ServIntro']) . '</p>');
            foreach ($s['Servicos'] as $it) {
                $preco = $it['Preco'] !== '' ? '<p class="mira-preco">A partir de R$ ' . number_format((float) $it['Preco'], 2, ',', '.') . '</p>' : '';
                $id    = $this->artigo($it['Titulo'], '<p>' . self::e($it['Texto']) . '</p>' . $preco,
                    '<h2>Como funciona</h2><ul><li>Atendimento com hora marcada</li><li>Explicamos tudo antes de começar</li><li>Acompanhamento depois do serviço</li></ul>', $cat);
                $this->links['servicos'][] = [$it, $id, $cat];
            }
            $this->links['menu']['servicos'] = $this->item($s['ServRotulo'], 'index.php?option=com_content&view=category&layout=blog&id=' . $cat,
                ['layout_type' => 'blog', 'num_leading_articles' => 0, 'num_intro_articles' => 12, 'num_columns' => 3, 'blog_class' => 'mira-colunas', 'show_category_description' => 1, 'show_intro' => 1, 'show_pagination' => 2]);
        }

        // ---- loja / cardápio (catálogo; o pedido vai pelo contato)
        if ($this->tem('catalogo')) {
            $cat = $this->categoria($s['LojaRotulo']);
            foreach ($s['Produtos'] as $it) {
                $id = $this->artigo($it['Titulo'], '<p>' . self::e($it['Texto']) . '</p><p class="mira-preco">R$ ' . number_format((float) $it['Preco'], 2, ',', '.') . '</p>', '', $cat);
                $this->links['produtos'][] = [$it, $id, $cat];
            }
            $this->links['menu']['catalogo'] = $this->item($s['LojaRotulo'], 'index.php?option=com_content&view=category&layout=blog&id=' . $cat,
                ['layout_type' => 'blog', 'num_leading_articles' => 0, 'num_intro_articles' => 24, 'num_columns' => 4, 'show_pagination' => 2]);
        }

        // ---- eventos
        if ($this->tem('eventos')) {
            $cat = $this->categoria('Eventos');
            foreach ($s['Eventos'] as $ev) {
                $data = date('d/m/Y', strtotime('+' . (int) $ev['Dias'] . ' days'));
                $this->artigo($ev['Titulo'], '<p class="mira-chapeu">' . $data . ' · ' . self::e($ev['Hora']) . ' · ' . self::e($ev['Local']) . '</p><p>' . self::e($ev['Resumo']) . '</p>',
                    '<p>Inscrições pelo formulário de contato. Vagas: ' . (int) $ev['Vagas'] . '.</p>', $cat);
            }
            $this->links['menu']['eventos'] = $this->item('Eventos', 'index.php?option=com_content&view=category&layout=blog&id=' . $cat, ['layout_type' => 'blog', 'num_leading_articles' => 0, 'num_intro_articles' => 10, 'num_columns' => 2]);
        }

        // ---- blog
        if ($this->tem('blog')) {
            $cat = $this->categoria($s['BlogRotulo']);
            foreach ($s['Posts'] as $po) {
                $tags = array_values(array_filter(array_map('trim', explode(',', (string) $po['Tags']))));
                $this->artigo($po['Titulo'], '<p>' . self::e($po['Resumo']) . '</p>',
                    '<h2>O que você precisa saber</h2><p>Este é um texto de exemplo criado junto com o site. Edite ou apague no painel, em Conteúdo &gt; Artigos.</p><ul><li>Escreva como fala com o cliente</li><li>Use intertítulos para facilitar a leitura</li><li>Termine com um convite para o próximo passo</li></ul>',
                    $cat, true, $tags);
            }
            $this->links['menu']['blog'] = $this->item($s['BlogRotulo'], 'index.php?option=com_content&view=category&layout=blog&id=' . $cat, ['layout_type' => 'blog', 'num_leading_articles' => 1, 'num_intro_articles' => 6, 'num_columns' => 2, 'show_pagination' => 2]);
        }

        // ---- equipe (contatos do próprio CMS)
        if ($this->tem('equipe')) {
            $cat = $this->catContato($s['EquipeRotulo']);
            foreach ($s['Equipe'] as $pe) {
                $this->contato($pe['Nome'], $pe['Cargo'], $pe['Texto'], $cat);
            }
            $this->links['menu']['equipe'] = $this->item($s['EquipeRotulo'], 'index.php?option=com_contact&view=category&id=' . $cat, ['show_position_headings' => 1]);
        }

        // ---- galeria
        if ($this->tem('galeria')) {
            $h = '<div class="mira-fotos">';
            foreach ($s['Galeria'] as $g) {
                $h .= '<figure>' . self::e($s['Emoji']) . '<figcaption>' . self::e($g) . '</figcaption></figure>';
            }
            $h .= '</div><p>Troque estes quadros por fotos de verdade: edite este artigo e use o botão de imagem.</p>';
            $id = $this->artigo('Galeria', $h, '', $semC);
            $this->links['menu']['galeria'] = $this->item('Galeria', 'index.php?option=com_content&view=article&id=' . $id);
        }

        // ---- downloads
        if ($this->tem('downloads')) {
            $h = '<p>Documentos e materiais para baixar. Envie os arquivos pela Mídia e troque os links abaixo.</p><ul>';
            foreach (['Catálogo completo (PDF)', 'Tabela de preços (PDF)', 'Política de privacidade (PDF)'] as $d) {
                $h .= '<li>' . self::e($d) . ' <em>(arquivo em breve)</em></li>';
            }
            $id = $this->artigo('Downloads', $h . '</ul>', '', $semC);
            $this->links['menu']['downloads'] = $this->item('Downloads', 'index.php?option=com_content&view=article&id=' . $id);
        }

        // ---- perguntas frequentes
        if ($this->tem('faq')) {
            $h = '';
            foreach ($s['Faq'] as $f) {
                $h .= '<details><summary>' . self::e($f[0]) . '</summary><p>' . self::e($f[1]) . '</p></details>';
            }
            $id = $this->artigo('Perguntas frequentes', $h, '', $semC);
            $this->links['faq'] = $id;
        }

        // ---- vagas
        if ($this->tem('vagas')) {
            $cat = $this->categoria('Trabalhe conosco');
            $this->artigo('Atendimento ao cliente', '<p>Atender clientes no balcão, por telefone e WhatsApp. Mande seu currículo pelo contato.</p>', '<h2>O que esperamos</h2><ul><li>Gostar de gente</li><li>Ensino médio completo</li></ul>', $cat);
            $this->links['vagas'] = $cat;
        }

        // ---- páginas de sempre
        $sobre = $this->artigo('Sobre', '<p>Somos <strong>' . self::e($nome) . '</strong>' . ($p['cidade'] ? ' em ' . self::e($p['cidade']) : '') . '. ' . self::e($s['Texto']) . '</p>',
            '<h2>Nossa história</h2><p>Conte aqui como tudo começou, o que move a equipe e o que torna o seu trabalho diferente.</p>', $semC, !$this->tem('blog'));
        $priv = $this->artigo('Política de privacidade', '<p>Esta política explica como <strong>' . self::e($nome) . '</strong> trata os dados pessoais de quem usa este site, conforme a Lei Geral de Proteção de Dados (Lei 13.709/2018).</p>',
            '<h2>Que dados coletamos</h2><p>Só o que você digita nos formulários (nome, e-mail, telefone e mensagem) e dados técnicos de acesso.</p><h2>Para que usamos</h2><p>Para responder, prestar o serviço pedido e cumprir obrigações legais. Não vendemos seus dados.</p><h2>Seus direitos</h2><p>Você pode pedir para ver, corrigir ou apagar seus dados pela página de contato.</p>', $semC);

        // contato principal (formulário do próprio CMS)
        $catC  = $this->catContato('Contato');
        $cId   = $this->contato($nome, $p['cidade'] ? 'Atendimento em ' . $p['cidade'] : 'Atendimento', 'Mande sua mensagem: respondemos em até um dia útil.', $catC, $email, true);
        if ($this->tem('agenda')) {
            $this->links['menu']['agenda'] = $this->item($s['AgendaVerbo'], 'index.php?option=com_contact&view=contact&id=' . $cId, ['page_heading' => $s['AgendaTitulo'], 'show_page_heading' => 1]);
        }
        if ($this->tem('planos')) {
            $this->links['menu']['planos'] = $this->item('Planos', '#planos', [], 'mainmenu', 'url');
        }
        $this->links['menu']['contato'] = $this->item('Contato', 'index.php?option=com_contact&view=contact&id=' . $cId);

        if (isset($this->links['faq'])) {
            $this->links['rodape']['faq'] = $this->item('Perguntas frequentes', 'index.php?option=com_content&view=article&id=' . $this->links['faq'], [], 'mirabilis-rodape');
        }
        $this->links['rodape']['sobre'] = $this->item('Sobre', 'index.php?option=com_content&view=article&id=' . $sobre, [], 'mirabilis-rodape');
        if (isset($this->links['vagas'])) {
            $this->links['rodape']['vagas'] = $this->item('Trabalhe conosco', 'index.php?option=com_content&view=category&layout=blog&id=' . $this->links['vagas'], ['layout_type' => 'blog'], 'mirabilis-rodape');
        }
        $this->links['rodape']['priv'] = $this->item('Política de privacidade', 'index.php?option=com_content&view=article&id=' . $priv, [], 'mirabilis-rodape');

        // página inicial: título da lista de destaques
        if ($this->home) {
            $reg = new Registry($this->db->setQuery($this->db->getQuery(true)->select('params')->from('#__menu')->where('id = ' . $this->home))->loadResult());
            $reg->set('show_page_heading', 1);
            $reg->set('page_heading', $this->tem('blog') ? 'Últimas publicações' : 'Quem somos');
            // com blog: três cartões; sem blog: o texto "Sobre" inteiro, em largura total
            $reg->set('num_leading_articles', $this->tem('blog') ? 0 : 1);
            $reg->set('num_intro_articles', $this->tem('blog') ? 3 : 0);
            $reg->set('num_columns', 3);
            $reg->set('num_links', 0);
            $reg->set('show_pagination', 0);
            $this->db->setQuery($this->db->getQuery(true)->update('#__menu')->set('params = ' . $this->db->quote($reg->toString()))->where('id = ' . $this->home))->execute();
        }
    }

    // ------------------------------------------------------------------ blocos (módulos)

    private function modulo(string $titulo, string $posicao, string $html, bool $soInicio = true, string $tipo = 'mod_custom', array $params = []): void
    {
        $model = $this->app->bootComponent('com_modules')->getMVCFactory()->createModel('Module', 'Administrator', ['ignore_request' => true]);
        $dados = [
            'id' => 0, 'asset_id' => 0, 'title' => $titulo, 'position' => $posicao, 'module' => $tipo, 'content' => $html, 'showtitle' => 0,
            'published' => 1, 'access' => 1, 'language' => '*', 'note' => 'Mirabilis', 'client_id' => 0, 'ordering' => ++$this->ordem,
            'params' => $params + ['prepare_content' => 0, 'layout' => '_:default', 'moduleclass_sfx' => '', 'cache' => 1, 'cache_time' => 900, 'module_tag' => 'div', 'bootstrap_size' => 0, 'header_tag' => 'h3', 'style' => 'System-none'],
        ];
        if ($soInicio && $this->home) {
            $dados['assignment'] = 1;
            $dados['assigned']   = [$this->home];
        } else {
            $dados['assignment'] = 0;
        }
        $model->setState('module.id', 0);
        if (!$model->save($dados)) {
            throw new \RuntimeException('Bloco "' . $titulo . '": ' . $model->getError());
        }
        $this->criados['modulos'][] = (int) $model->getState('module.id');
    }

    private function link(?int $itemid): string
    {
        return $itemid ? 'index.php?Itemid=' . $itemid : 'index.php';
    }

    private function artigoLink(int $id, int $cat): string
    {
        return 'index.php?option=com_content&amp;view=article&amp;id=' . $id . '&amp;catid=' . $cat;
    }

    private function blocos(): void
    {
        $s = $this->s;
        $p = $this->p;
        $e = fn ($x) => self::e((string) $x);
        $m = $this->links['menu'] ?? [];

        // menu principal no cabeçalho e rodapé em todas as páginas
        $this->modulo('Menu principal', 'menu', '', false, 'mod_menu', ['menutype' => 'mainmenu', 'layout' => 'cassiopeia:collapse-metismenu', 'startLevel' => 1, 'endLevel' => 0, 'showAllChildren' => 1, 'module_tag' => 'nav', 'style' => 'System-none']);

        // capa
        [$cta, $ctaU] = ['Fale com a gente', $this->link($m['contato'] ?? null)];
        if (isset($m['agenda'])) {
            [$cta, $ctaU] = [$s['AgendaTitulo'], $this->link($m['agenda'])];
        } elseif (isset($m['catalogo'])) {
            [$cta, $ctaU] = ['Ver ' . mb_strtolower($s['LojaRotulo']), $this->link($m['catalogo'])];
        } elseif (isset($m['eventos'])) {
            [$cta, $ctaU] = ['Ver os próximos eventos', $this->link($m['eventos'])];
        }
        if ($p['template'] === 'villosa' && isset($m['blog'])) {
            [$cta, $ctaU] = ['Ler as últimas', $this->link($m['blog'])];
        }
        [$cta2, $cta2U] = ['Fale com a gente', $this->link($m['contato'] ?? null)];
        if (isset($m['servicos'])) {
            [$cta2, $cta2U] = ['Conhecer ' . mb_strtolower($s['ServRotulo']), $this->link($m['servicos'])];
        } elseif (isset($m['blog'])) {
            [$cta2, $cta2U] = ['Ler ' . mb_strtolower($s['BlogRotulo']), $this->link($m['blog'])];
        }
        if ($cta2U === $ctaU) {
            $cta2 = 'Sobre nós';
            $cta2U = $this->link($this->links['rodape']['sobre'] ?? null);
        }
        $this->modulo('Capa', 'banner', '<section class="mira-capa"><div class="mira-faixa"><div>'
            . '<span class="mira-chapeu">' . $e($s['Chapeu']) . '</span><h1>' . $e($s['Titulo']) . '</h1><p>' . $e($s['Texto']) . '</p>'
            . '<div class="mira-acoes"><a class="mira-botao" href="' . $ctaU . '">' . $e($cta) . '</a><a class="mira-botao secundario" href="' . $cta2U . '">' . $e($cta2) . '</a></div>'
            . '</div><div class="mira-arte" aria-hidden="true"><span>' . $e($s['Emoji']) . '</span></div></div></section>');

        // serviços
        if (!empty($this->links['servicos'])) {
            $h = '<section class="mira-secao alt"><div class="mira-faixa"><div class="mira-centro"><span class="mira-chapeu">O que oferecemos</span><h2>' . $e($s['ServRotulo']) . '</h2><p class="mira-intro">' . $e($s['ServIntro']) . '</p></div><div class="mira-grade">';
            foreach ($this->links['servicos'] as [$it, $id, $cat]) {
                $preco = $it['Preco'] !== '' ? '<span class="mira-preco">a partir de R$ ' . number_format((float) $it['Preco'], 2, ',', '.') . '</span>' : '';
                $h .= '<a class="mira-cartao" href="' . $this->artigoLink($id, $cat) . '"><div class="mira-icone">' . $e($it['Icone']) . '</div><h3>' . $e($it['Titulo']) . '</h3><p>' . $e($it['Texto']) . '</p>' . $preco . '</a>';
            }
            $this->modulo($s['ServRotulo'], 'top-a', $h . '</div></div></section>');
        }

        // números
        if (!empty($s['Numeros'])) {
            $h = '<section class="mira-secao"><div class="mira-faixa mira-numeros">';
            foreach ($s['Numeros'] as $n) {
                $h .= '<div><strong>' . $e($n[0]) . '</strong><span>' . $e($n[1]) . '</span></div>';
            }
            $this->modulo('Números', 'top-a', $h . '</div></section>');
        }

        // vitrine
        if (!empty($this->links['produtos'])) {
            $h = '<section class="mira-secao alt"><div class="mira-faixa"><div class="mira-centro"><span class="mira-chapeu">Peça pelo site</span><h2>Em destaque</h2></div><div class="mira-grade">';
            foreach (\array_slice($this->links['produtos'], 0, 4) as [$it, $id, $cat]) {
                $h .= '<div class="mira-cartao"><div class="mira-icone">' . $e($it['Icone']) . '</div><h3>' . $e($it['Titulo']) . '</h3><span class="mira-preco">R$ ' . number_format((float) $it['Preco'], 2, ',', '.') . '</span>'
                    . '<a class="mira-botao" href="' . $this->link($m['contato'] ?? null) . '">Pedir</a></div>';
            }
            $this->modulo('Vitrine', 'top-b', $h . '</div><p class="mira-centro" style="margin-top:28px"><a class="mira-botao secundario" href="' . $this->link($m['catalogo']) . '">Ver tudo</a></p></div></section>');
        }

        // planos
        if ($this->tem('planos')) {
            $h = '<section class="mira-secao mira-planos" id="planos"><div class="mira-faixa"><div class="mira-centro"><span class="mira-chapeu">Planos</span><h2>Escolha o seu plano</h2><p class="mira-intro">Sem letra miúda: troque de plano quando quiser.</p></div><div class="mira-grade">';
            foreach ($s['Planos'] as $pl) {
                $h .= '<div class="mira-cartao' . ($pl['Destaque'] ? ' destaque' : '') . '">' . ($pl['Destaque'] ? '<span class="mira-selo">Mais escolhido</span>' : '')
                    . '<h3>' . $e($pl['Nome']) . '</h3><p class="valor"><b>' . $e($pl['Preco']) . '</b> ' . $e($pl['Periodo']) . '</p><ul>';
                foreach ($pl['Itens'] as $i) {
                    $h .= '<li>' . $e($i) . '</li>';
                }
                $h .= '</ul><a class="mira-botao' . ($pl['Destaque'] ? '' : ' secundario') . '" href="' . $this->link($m['contato'] ?? null) . '">Quero este</a></div>';
            }
            $this->modulo('Planos', 'top-b', $h . '</div></div></section>');
        }

        // depoimentos
        if ($this->tem('depoimentos')) {
            $h = '<section class="mira-secao"><div class="mira-faixa"><div class="mira-centro"><span class="mira-chapeu">Quem já conhece</span><h2>O que dizem</h2></div><div class="mira-grade">';
            foreach ($s['Depo'] as $d) {
                $h .= '<blockquote class="mira-cartao mira-depo"><span class="mira-estrelas">★★★★★</span><p>“' . $e($d[0]) . '”</p><footer>' . $e($d[1]) . '</footer></blockquote>';
            }
            $this->modulo('Depoimentos', 'bottom-a', $h . '</div></div></section>');
        }

        // perguntas
        if ($this->tem('faq')) {
            $h = '<section class="mira-secao alt"><div class="mira-faixa" style="max-width:820px"><span class="mira-chapeu">Dúvidas</span><h2>Perguntas frequentes</h2>';
            foreach ($s['Faq'] as $f) {
                $h .= '<details><summary>' . $e($f[0]) . '</summary><p>' . $e($f[1]) . '</p></details>';
            }
            $this->modulo('Perguntas frequentes', 'bottom-a', $h . '</div></section>');
        }

        // chamada final
        if ($p['template'] === 'villosa') {
            [$cta, $ctaU] = ['Escreva para a redação', $this->link($m['contato'] ?? null)];
        }
        $this->modulo('Chamada', 'bottom-b', '<section class="mira-chamada"><h2>' . $e($s['Chamada']) . '</h2><p>Respondemos rápido, por WhatsApp ou e-mail.</p><a class="mira-botao" href="' . $ctaU . '">' . $e($cta) . '</a></section>');

        // rodapé
        $nav = '<li><a href="index.php">Início</a></li>';
        foreach (['servicos' => $s['ServRotulo'], 'catalogo' => $s['LojaRotulo'], 'eventos' => 'Eventos', 'blog' => $s['BlogRotulo'], 'equipe' => $s['EquipeRotulo'], 'galeria' => 'Galeria', 'downloads' => 'Downloads', 'contato' => 'Contato'] as $k => $t) {
            if (isset($m[$k])) {
                $nav .= '<li><a href="' . $this->link($m[$k]) . '">' . $e($t) . '</a></li>';
            }
        }
        $inst = '';
        foreach (['sobre' => 'Sobre', 'faq' => 'Perguntas frequentes', 'vagas' => 'Trabalhe conosco', 'priv' => 'Política de privacidade'] as $k => $t) {
            if (isset($this->links['rodape'][$k])) {
                $inst .= '<li><a href="' . $this->link($this->links['rodape'][$k]) . '">' . $e($t) . '</a></li>';
            }
        }
        $this->modulo('Rodapé', 'footer', '<div class="mira-rodape"><div><h4>' . $e($p['nome']) . '</h4><p>' . $e($s['Texto']) . '</p></div>'
            . '<div><h4>Navegue</h4><ul>' . $nav . '</ul></div><div><h4>Institucional</h4><ul>' . $inst . '</ul></div>'
            . '<div><h4>Fale com a gente</h4><ul><li>(00) 00000-0000</li><li>' . $e($p['cidade'] ?: 'Seu endereço aqui') . '</li></ul></div></div>'
            . '<div class="mira-base"><span>© ' . date('Y') . ' ' . $e($p['nome']) . '. Todos os direitos reservados.</span><span>Feito com Mirabilis</span></div>', false);
    }

    // ------------------------------------------------------------------ nome e registro

    private function nomeDoSite(): void
    {
        $f = JPATH_CONFIGURATION . '/configuration.php';
        if (is_file($f) && is_writable($f)) {
            $src = (string) file_get_contents($f);
            $nome = str_replace(['\\', "'"], ['\\\\', "\\'"], $this->p['nome']);
            $novo = preg_replace("/public \\\$sitename = '.*?';/", "public \$sitename = '" . $nome . "';", $src, 1);
            if ($novo && $novo !== $src) {
                file_put_contents($f, $novo);
                if (\function_exists('opcache_invalidate')) {
                    opcache_invalidate($f, true);
                }
            }
        }
        $this->app->set('sitename', $this->p['nome']);
    }

    private function guardar(): void
    {
        $params = ComponentHelper::getParams('com_mirabilis');
        $params->set('criados', $this->criados);
        $params->set('descricao', $this->p['descricao']);
        $params->set('quando', date('c'));
        $this->db->setQuery($this->db->getQuery(true)->update('#__extensions')->set('params = ' . $this->db->quote($params->toString()))
            ->where('element = ' . $this->db->quote('com_mirabilis'))->where('type = ' . $this->db->quote('component')))->execute();
        $this->limparCache();
    }

    private function limparCache(): void
    {
        try {
            Factory::getCache('', 'callback')->clean();
            foreach (['com_modules', 'mod_menu', 'mod_custom', 'com_content', '_system', 'com_templates'] as $g) {
                Factory::getCache($g, '')->clean();
            }
        } catch (\Throwable) {
        }
    }

    /** Apaga o que a última montagem criou (os artigos vão de vez; é conteúdo de exemplo). */
    public function desfazer(): void
    {
        $c = (array) ComponentHelper::getParams('com_mirabilis')->get('criados');
        $c = json_decode(json_encode($c), true) ?: [];
        $q = fn () => $this->db->getQuery(true);
        $em = fn (array $ids) => implode(',', array_map('intval', $ids ?: [0]));

        if (!empty($c['modulos'])) {
            $this->db->setQuery($q()->delete('#__modules_menu')->where('moduleid IN (' . $em($c['modulos']) . ')'))->execute();
            $this->db->setQuery($q()->delete('#__modules')->where('id IN (' . $em($c['modulos']) . ')'))->execute();
        }
        foreach ($c['itens'] ?? [] as $id) {
            $t = Table::getInstance('Menu', 'Joomla\\CMS\\Table\\', ['dbo' => $this->db]);
            if ($t->load((int) $id)) {
                $t->delete((int) $id);
            }
        }
        if (!empty($c['menus'])) {
            $this->db->setQuery($q()->delete('#__menu_types')->where('id IN (' . $em($c['menus']) . ')'))->execute();
        }
        foreach ($c['artigos'] ?? [] as $id) {
            $t = Table::getInstance('Content', 'Joomla\\CMS\\Table\\', ['dbo' => $this->db]);
            if ($t->load((int) $id)) {
                $t->delete((int) $id);
            }
            $this->db->setQuery($q()->delete('#__content_frontpage')->where('content_id = ' . (int) $id))->execute();
            $this->db->setQuery($q()->delete('#__workflow_associations')->where('item_id = ' . (int) $id)->where('extension = ' . $this->db->quote('com_content.article')))->execute();
            $this->db->setQuery($q()->delete('#__contentitem_tag_map')->where('content_item_id = ' . (int) $id)->where('type_alias = ' . $this->db->quote('com_content.article')))->execute();
        }
        if (!empty($c['contatos'])) {
            $this->db->setQuery($q()->delete('#__contact_details')->where('id IN (' . $em($c['contatos']) . ')'))->execute();
        }
        foreach (array_reverse($c['categorias'] ?? []) as $id) {
            $t = Table::getInstance('Category', 'Joomla\\CMS\\Table\\', ['dbo' => $this->db]);
            if ($t->load((int) $id)) {
                $t->delete((int) $id);
            }
        }
        if (!empty($c['estilos'])) {
            $this->db->setQuery($q()->delete('#__template_styles')->where('id IN (' . $em($c['estilos']) . ')'))->execute();
            // volta um estilo como padrão, se nenhum ficou
            $tem = (int) $this->db->setQuery($q()->select('COUNT(*)')->from('#__template_styles')->where('client_id = 0')->where($this->db->quoteName('home') . ' = ' . $this->db->quote('1')))->loadResult();
            if (!$tem) {
                $this->db->setQuery($q()->update('#__template_styles')->set($this->db->quoteName('home') . ' = ' . $this->db->quote('1'))
                    ->where('client_id = 0')->where('template = ' . $this->db->quote('mirabilis_rajah')))->execute();
            }
        }
        $params = ComponentHelper::getParams('com_mirabilis');
        $params->set('criados', null);
        $this->db->setQuery($q()->update('#__extensions')->set('params = ' . $this->db->quote($params->toString()))
            ->where('element = ' . $this->db->quote('com_mirabilis'))->where('type = ' . $this->db->quote('component')))->execute();
        // a próxima leitura dos parâmetros precisa ver a mudança
        $r = new \ReflectionClass(ComponentHelper::class);
        if ($r->hasProperty('components')) {
            $pr = $r->getProperty('components');
            $pr->setValue(null, []);
        }
    }
}
