<?php

/**
 * @package     Mirabilis.Administrator
 * @subpackage  com_mirabilis
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

namespace Mirabilis\Component\Mirabilis\Administrator\View\Criar;

use Joomla\CMS\Factory;
use Joomla\CMS\MVC\View\HtmlView as BaseHtmlView;
use Joomla\CMS\Toolbar\ToolbarHelper;
use Joomla\CMS\Uri\Uri;
use Mirabilis\Component\Mirabilis\Administrator\Helper\Construtor;
use Mirabilis\Component\Mirabilis\Administrator\Helper\Entender;

\defined('_JEXEC') or die;

class HtmlView extends BaseHtmlView
{
    public string $descricao = '';
    public ?array $plano     = null;
    public bool $montado     = false;
    public array $templates  = [];
    public string $site      = '';

    public function display($tpl = null): void
    {
        $app             = Factory::getApplication();
        $this->descricao = (string) $app->getUserState('com_mirabilis.descricao', '');
        $this->plano     = $this->descricao !== '' ? Entender::plano($this->descricao) : null;
        $this->montado   = Construtor::jaMontado();
        $this->templates = Entender::TEMPLATES;
        $this->site      = Uri::root();

        ToolbarHelper::title('Descreva seu site', 'magic');

        $this->getDocument()->getWebAssetManager()
            ->registerAndUseStyle('mirabilis.painel', 'media/com_mirabilis/css/painel.css');

        parent::display($tpl);
    }
}
