<?php

/**
 * @package     Mirabilis.Administrator
 * @subpackage  com_mirabilis
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

namespace Mirabilis\Component\Mirabilis\Administrator\Controller;

use Joomla\CMS\MVC\Controller\BaseController;
use Joomla\Database\DatabaseInterface;
use Mirabilis\Component\Mirabilis\Administrator\Helper\Construtor;
use Mirabilis\Component\Mirabilis\Administrator\Helper\Entender;

\defined('_JEXEC') or die;

/**
 * "Descreva seu site": a pessoa escreve o que quer, o Mirabilis mostra o plano
 * (template, cor, seções) e, ao confirmar, monta o site.
 */
class DisplayController extends BaseController
{
    protected $default_view = 'criar';

    /** Mostra o plano sem mudar nada. */
    public function planejar()
    {
        $this->checkToken();
        $this->exigir();

        $desc = trim($this->input->post->getString('descricao', ''));
        $this->app->setUserState('com_mirabilis.descricao', $desc);
        $this->setRedirect('index.php?option=com_mirabilis&view=criar');

        return $this;
    }

    /** Monta (ou refaz) o site a partir da descrição. */
    public function montar()
    {
        $this->checkToken();
        $this->exigir();

        $desc    = trim($this->input->post->getString('descricao', ''));
        $refazer = $this->input->post->getBool('refazer', false);
        $this->app->setUserState('com_mirabilis.descricao', $desc);
        $volta = 'index.php?option=com_mirabilis&view=criar';

        if ($desc === '') {
            $this->setRedirect($volta, 'Escreva como quer o site.', 'warning');

            return $this;
        }

        try {
            $plano = Entender::plano($desc);
            $c     = new Construtor($this->app, \Joomla\CMS\Factory::getContainer()->get(DatabaseInterface::class));
            $r     = $c->aplicar($plano, $refazer);
        } catch (\Throwable $e) {
            $this->setRedirect($volta, $e->getMessage(), 'error');

            return $this;
        }

        $t = Entender::TEMPLATES[$r['template']][0] ?? $r['template'];
        $n = $r['criados'];
        $this->setRedirect(
            $volta,
            sprintf(
                'Site montado com o template %s: %d páginas, %d categorias, %d contatos, %d blocos e %d itens de menu. Abra o site para ver.',
                $t,
                $n['artigos'],
                $n['categorias'],
                $n['contatos'],
                $n['modulos'],
                $n['itens']
            )
        );

        foreach ($plano['avisos'] as $a) {
            $this->app->enqueueMessage($a, 'notice');
        }

        return $this;
    }

    private function exigir(): void
    {
        $u = $this->app->getIdentity();

        if (!$u || !$u->authorise('core.admin')) {
            throw new \Joomla\CMS\Access\Exception\NotAllowed('Só o administrador do site pode montar o site.', 403);
        }
    }
}
