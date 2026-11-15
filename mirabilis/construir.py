#!/usr/bin/env python3
"""Monta o pacote do Mirabilis a partir do pacote oficial do Joomla.

    python3 mirabilis/construir.py Joomla_6.2.0-Stable-Full_Package.zip saida/

O que faz:
  1. abre o zip do Joomla numa pasta nova;
  2. copia por cima mirabilis/arquivos/ (componente "Descreva seu site",
     comando de terminal, seis templates, custom.sql do instalador);
  3. troca a marca do produto: nome nas frases da interface, logotipos,
     ícone, meta "generator" e Version::PRODUCT;
  4. mantém todos os avisos de copyright e o LICENSE.txt (GPL) e acrescenta
     CREDITOS.txt;
  5. grava saida/Mirabilis_<versão>.zip.

Nomes de serviços reais do Joomla (joomla.org, JED, documentação, fórum,
comunidade, projeto, marca registrada) continuam com o nome original: trocar
seria dizer que esses serviços são nossos.
"""
import os
import re
import shutil
import sys
import zipfile

AQUI = os.path.dirname(os.path.abspath(__file__))
PRODUTO = 'Mirabilis'

# frases com estes trechos falam do projeto Joomla de verdade: ficam como estão
MANTER = re.compile(
    r'joomla\.org|joomla\.invalid|https?://|Extensions? Directory|\bJED\b|Forum|Documentation|Magazine|'
    r'Community|Volunteer|Trademark|Open Source Matters|Project|Translation|Joomla!? ?\d|J\d|'
    r'Platform|Update|Atualiza|Statist|Estat|Stats|Joomla! Help|Joomla Help|Joomla! Framework|Joomla Framework|Joomla! Core|LIB_JOOMLA',
    re.I)

PASTAS_IDIOMA = ['language', 'administrator/language', 'api/language', 'installation/language']


def trocar_ini(caminho):
    linhas = open(caminho, encoding='utf-8').read().split('\n')
    mudou = 0
    for i, l in enumerate(linhas):
        if l.startswith(';') or '=' not in l or 'Joomla' not in l:
            continue
        chave, valor = l.split('=', 1)
        if 'Joomla' not in valor or MANTER.search(valor):
            continue
        novo = re.sub(r'Joomla!?(?![\w/.])', PRODUTO, valor)
        if novo != valor:
            linhas[i] = chave + '=' + novo
            mudou += 1
    if mudou:
        open(caminho, 'w', encoding='utf-8').write('\n'.join(linhas))
    return mudou


def trocar_texto(caminho, pares):
    s = open(caminho, encoding='utf-8').read()
    for a, b in pares:
        if a not in s:
            raise SystemExit(f'não achei {a!r} em {caminho}: o Joomla mudou?')
        s = s.replace(a, b)
    open(caminho, 'w', encoding='utf-8').write(s)


def main():
    if len(sys.argv) != 3:
        raise SystemExit(__doc__)
    zip_joomla, saida = sys.argv[1], os.path.abspath(sys.argv[2])
    os.makedirs(saida, exist_ok=True)
    raiz = os.path.join(saida, 'Mirabilis')
    if os.path.exists(raiz):
        shutil.rmtree(raiz)
    with zipfile.ZipFile(zip_joomla) as z:
        z.extractall(raiz)

    # 2. arquivos próprios
    shutil.copytree(os.path.join(AQUI, 'arquivos'), raiz, dirs_exist_ok=True)
    # media do componente fica também onde o Joomla a procura
    # (já está em arquivos/media/com_mirabilis)

    # 2b. tradução pt-BR (Use Joomla! - Comunidade Brasileira de Joomla!, GPL), idioma padrão
    t = os.path.join(AQUI, 'idioma', 'pt-BR')
    for de, para in (('site', 'language/pt-BR'), ('admin', 'administrator/language/pt-BR'), ('api', 'api/language/pt-BR')):
        shutil.copytree(os.path.join(t, de), os.path.join(raiz, para), dirs_exist_ok=True)
    shutil.copyfile(os.path.join(t, 'pkg_pt-BR.xml'), os.path.join(raiz, 'administrator/manifests/packages/pkg_pt-BR.xml'))

    # 3a. frases da interface
    total = 0
    for pasta in PASTAS_IDIOMA:
        for dirpath, _, nomes in os.walk(os.path.join(raiz, pasta)):
            for n in nomes:
                if n.endswith('.ini'):
                    total += trocar_ini(os.path.join(dirpath, n))
    print(f'frases trocadas: {total}')

    # 3b. nome do produto no código
    trocar_texto(os.path.join(raiz, 'libraries/src/Version.php'),
                 [("public const PRODUCT = 'Joomla!';", f"public const PRODUCT = '{PRODUTO}';")])
    gerador = f'{PRODUTO} - CMS baseado no Joomla (GPL)'
    trocar_texto(os.path.join(raiz, 'libraries/src/Document/Document.php'),
                 [("'Joomla! - Open Source Content Management'", f"'{gerador}'")])
    trocar_texto(os.path.join(raiz, 'libraries/src/Application/SiteApplication.php'),
                 [("'Joomla! - Open Source Content Management - Version '", f"'{gerador} - versão '"),
                  ("'Joomla! - Open Source Content Management'", f"'{gerador}'")])
    trocar_texto(os.path.join(raiz, 'libraries/src/Application/AdministratorApplication.php'),
                 [("'Joomla! - Open Source Content Management'", f"'{gerador}'")])

    trocar_texto(os.path.join(raiz, 'installation/src/Console/InstallCommand.php'),
                 [("$cfg['language']             = 'en-GB';", "$cfg['language']             = 'pt-BR';"),
                  ("title('Install Joomla')", f"title('Instalar o {PRODUTO}')"),
                  ("success('Joomla has been installed')", f"success('{PRODUTO} instalado')"),
                  ("setDescription('Install the Joomla CMS')", f"setDescription('Instala o {PRODUTO}')")])

    # 3c. logotipos e ícones (mesmos nomes de arquivo, desenho novo)
    m = os.path.join(AQUI, 'marca')
    copias = {
        'logo-branco.svg': ['media/templates/administrator/atum/images/logos/brand-large.svg',
                            'installation/template/images/Joomla-logo-monochrome-horizontal-white.svg'],
        'simbolo-branco.svg': ['media/templates/administrator/atum/images/logos/brand-small.svg',
                               'installation/template/images/Joomla-brandmark-monochrome-white-RGB.svg'],
        'logo-verde.svg': ['installation/template/images/logo.svg', 'installation/template/images/logo-blue.svg',
                           'installation/template/images/logo-joomla-blue.svg'],
        'simbolo.svg': ['media/templates/administrator/atum/images/logos/login.svg',
                        'media/system/images/joomla-favicon.svg'],
        'simbolo-preto.svg': ['media/system/images/joomla-favicon-pinned.svg'],
        'carregando.svg': ['media/system/images/joomla-loader.svg'],
        'favicon.ico': ['media/system/images/favicon.ico', 'installation/favicon.ico'],
    }
    for origem, destinos in copias.items():
        for d in destinos:
            alvo = os.path.join(raiz, d)
            if not os.path.exists(alvo):
                raise SystemExit(f'falta {d} no Joomla')
            shutil.copyfile(os.path.join(m, origem), alvo)
    # versões comprimidas antigas dos svgs não podem sobrar com o desenho velho
    for d in sum(copias.values(), []):
        for ext in ('.gz',):
            if os.path.exists(os.path.join(raiz, d + ext)):
                os.remove(os.path.join(raiz, d + ext))

    # 4. créditos (LICENSE.txt e os cabeçalhos de copyright ficam intactos)
    shutil.copyfile(os.path.join(AQUI, 'CREDITOS.txt'), os.path.join(raiz, 'CREDITOS.txt'))
    shutil.copyfile(os.path.join(AQUI, 'LEIAME.md'), os.path.join(raiz, 'LEIAME.md'))

    # 5. zip
    versao = re.search(r"MAJOR_VERSION = (\d+);.*?MINOR_VERSION = (\d+);.*?PATCH_VERSION = (\d+);",
                       open(os.path.join(raiz, 'libraries/src/Version.php')).read(), re.S)
    nome = os.path.join(saida, f'Mirabilis_{".".join(versao.groups())}.zip')
    with zipfile.ZipFile(nome, 'w', zipfile.ZIP_DEFLATED) as z:
        for dirpath, _, nomes in os.walk(raiz):
            for n in sorted(nomes):
                c = os.path.join(dirpath, n)
                z.write(c, os.path.relpath(c, raiz))
    print(nome)


if __name__ == '__main__':
    main()
