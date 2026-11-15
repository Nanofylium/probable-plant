<?php

/**
 * @package     Mirabilis.Plugin
 * @subpackage  Console.mirabilis
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

namespace Mirabilis\Plugin\Console\Mirabilis\Extension;

use Joomla\Application\ApplicationEvents;
use Joomla\Application\Event\ApplicationEvent;
use Joomla\CMS\Plugin\CMSPlugin;
use Joomla\Event\SubscriberInterface;
use Mirabilis\Plugin\Console\Mirabilis\Command\CriarCommand;

\defined('_JEXEC') or die;

final class Mirabilis extends CMSPlugin implements SubscriberInterface
{
    public static function getSubscribedEvents(): array
    {
        return [ApplicationEvents::BEFORE_EXECUTE => 'registrar'];
    }

    public function registrar(ApplicationEvent $event): void
    {
        $event->getApplication()->addCommand(new CriarCommand());
    }
}
