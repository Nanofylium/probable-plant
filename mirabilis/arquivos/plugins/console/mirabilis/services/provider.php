<?php

/**
 * @package     Mirabilis.Plugin
 * @subpackage  Console.mirabilis
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

\defined('_JEXEC') or die;

use Joomla\CMS\Extension\PluginInterface;
use Joomla\CMS\Factory;
use Joomla\CMS\Plugin\PluginHelper;
use Joomla\DI\Container;
use Joomla\DI\ServiceProviderInterface;
use Joomla\Event\DispatcherInterface;
use Mirabilis\Plugin\Console\Mirabilis\Extension\Mirabilis;

return new class () implements ServiceProviderInterface {
    public function register(Container $container)
    {
        $container->set(
            PluginInterface::class,
            function (Container $container) {
                $plugin = new Mirabilis($container->get(DispatcherInterface::class), (array) PluginHelper::getPlugin('console', 'mirabilis'));
                $plugin->setApplication(Factory::getApplication());

                return $plugin;
            }
        );
    }
};
