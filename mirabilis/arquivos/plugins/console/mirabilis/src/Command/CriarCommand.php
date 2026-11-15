<?php

/**
 * @package     Mirabilis.Plugin
 * @subpackage  Console.mirabilis
 *
 * @copyright   (C) 2026 Mirabilis
 * @license     GNU General Public License version 2 or later; see LICENSE.txt
 */

namespace Mirabilis\Plugin\Console\Mirabilis\Command;

use Joomla\CMS\Factory;
use Joomla\CMS\User\User;
use Joomla\Console\Command\AbstractCommand;
use Joomla\Database\DatabaseInterface;
use Mirabilis\Component\Mirabilis\Administrator\Helper\Construtor;
use Mirabilis\Component\Mirabilis\Administrator\Helper\Entender;
use Symfony\Component\Console\Input\InputArgument;
use Symfony\Component\Console\Input\InputInterface;
use Symfony\Component\Console\Input\InputOption;
use Symfony\Component\Console\Output\OutputInterface;
use Symfony\Component\Console\Style\SymfonyStyle;

\defined('_JEXEC') or die;

/** php cli/joomla.php mirabilis:criar "descrição do site" [--plano] [--refazer] */
final class CriarCommand extends AbstractCommand
{
    protected static $defaultName = 'mirabilis:criar';

    protected function configure(): void
    {
        $this->setDescription('Monta o site a partir de uma descrição em português');
        $this->addArgument('descricao', InputArgument::REQUIRED, 'Como você quer o site');
        $this->addOption('plano', null, InputOption::VALUE_NONE, 'Só mostra o que seria feito');
        $this->addOption('refazer', null, InputOption::VALUE_NONE, 'Troca o site montado antes por um novo');
        $this->setHelp("Exemplo:\n  php cli/joomla.php mirabilis:criar \"Clínica veterinária Pata Amiga em Curitiba, com agenda e equipe, cor verde\"");
    }

    protected function doExecute(InputInterface $input, OutputInterface $output): int
    {
        $io = new SymfonyStyle($input, $output);
        $p  = Entender::plano((string) $input->getArgument('descricao'));
        $t  = Entender::TEMPLATES[$p['template']];
        $nm = Entender::regras()['nomesModulo'] ?? [];

        $io->title('Mirabilis: ' . $p['nome']);
        $io->definitionList(
            ['Ramo' => $p['setor']['Rotulo']],
            ['Template' => $t[0] . ' (' . $t[1] . ')'],
            ['Cor' => $p['cor']],
            ['Seções' => implode(', ', array_map(static fn ($m) => $nm[$m] ?? $m, $p['modulos'])) ?: '-']
        );

        foreach ($p['notas'] as $n) {
            $io->writeln('  · ' . $n);
        }

        foreach ($p['avisos'] as $a) {
            $io->warning($a);
        }

        if ($input->getOption('plano')) {
            return 0;
        }

        $app = $this->getApplication();

        // no terminal não há componente ativo; os modelos do CMS esperam um
        foreach (['JPATH_COMPONENT', 'JPATH_COMPONENT_ADMINISTRATOR'] as $c) {
            \defined($c) || \define($c, JPATH_ADMINISTRATOR . '/components/com_mirabilis');
        }
        \defined('JPATH_COMPONENT_SITE') || \define('JPATH_COMPONENT_SITE', JPATH_SITE . '/components/com_content');
        $db  = Factory::getContainer()->get(DatabaseInterface::class);

        // os modelos do CMS precisam de um usuário: usa o primeiro super usuário
        $id = (int) $db->setQuery(
            $db->getQuery(true)->select('m.user_id')->from('#__user_usergroup_map AS m')->where('m.group_id = 8')->order('m.user_id')
        , 0, 1)->loadResult();
        $app->loadIdentity(new User($id));
        $app->getLanguage()->load('lib_joomla', JPATH_ADMINISTRATOR);
        $app->getLanguage()->load('com_content', JPATH_ADMINISTRATOR);

        try {
            $r = (new Construtor($app, $db))->aplicar($p, (bool) $input->getOption('refazer'));
        } catch (\Throwable $e) {
            $io->error($e->getMessage());

            return 1;
        }

        $n = $r['criados'];
        $io->success(sprintf('Site montado com o template %s: %d páginas, %d categorias, %d contatos, %d blocos, %d itens de menu.', $t[0], $n['artigos'], $n['categorias'], $n['contatos'], $n['modulos'], $n['itens']));

        return 0;
    }
}
