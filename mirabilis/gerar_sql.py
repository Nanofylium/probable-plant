#!/usr/bin/env python3
"""Gera installation/sql/<banco>/custom.sql: registra o componente, o plugin e os
seis templates do Mirabilis e põe o botão "Descreva seu site" no painel."""
import json, os
aqui = os.path.dirname(os.path.abspath(__file__))
tpls = ['rajah', 'villosa', 'ampullaria', 'lowii', 'truncata', 'edwardsiana']
nomes = {'rajah': 'Rajah', 'villosa': 'Villosa', 'ampullaria': 'Ampullaria', 'lowii': 'Lowii', 'truncata': 'Truncata', 'edwardsiana': 'Edwardsiana'}
cores = {'rajah': '#2f6b4f', 'villosa': '#9b2335', 'ampullaria': '#7c3aed', 'lowii': '#0f766e', 'truncata': '#b45309', 'edwardsiana': '#1d4ed8'}
fonte = {'villosa': 'titulo-serifa', 'truncata': 'titulo-serifa'}
painel = ('<div class="mira-dash" style="display:flex;flex-wrap:wrap;gap:1rem;align-items:center;justify-content:space-between">'
          '<p style="margin:0;font-size:1.05rem">Escreva com suas palavras como quer o site e o Mirabilis escolhe o template e monta páginas, menus e a página inicial.</p>'
          '<a class="btn btn-success btn-lg" href="index.php?option=com_mirabilis">Descreva seu site</a></div>')

def q(s):  # literal SQL
    return "'" + s.replace("\\", "\\\\").replace("'", "''") + "'"

def gerar(banco):
    pg = banco == 'postgresql'
    n = (lambda x: '"' + x + '"') if pg else (lambda x: '`' + x + '`')
    def cols(*c): return '(' + ', '.join(n(x) for x in c) + ')'
    ext = cols('package_id', 'name', 'type', 'element', 'folder', 'client_id', 'enabled', 'access', 'protected', 'locked', 'manifest_cache', 'params', 'custom_data', 'ordering', 'state')
    out = ['-- Mirabilis: registra as extensões próprias (gerado por gerar_sql.py)', '']
    linhas = ["(0, 'com_mirabilis', 'component', 'com_mirabilis', '', 1, 1, 1, 0, 1, '', '{}', '', 0, 0)",
              "(0, 'plg_console_mirabilis', 'plugin', 'mirabilis', 'console', 0, 1, 1, 0, 1, '', '{}', '', 0, 0)"]
    for t in tpls:
        linhas.append(f"(0, 'mirabilis_{t}', 'template', 'mirabilis_{t}', '', 0, 1, 1, 0, 1, '', '{{}}', '', 0, 0)")
    out.append(f'INSERT INTO {n("#__extensions")} {ext} VALUES\n' + ',\n'.join(linhas) + ';\n')
    # estatísticas vão para o servidor do Joomla: desligadas, para não pedir dados em nome errado
    out.append(f"UPDATE {n('#__extensions')} SET {n('enabled')} = 0 WHERE {n('type')} = 'plugin' AND {n('folder')} = 'system' AND {n('element')} = 'stats';\n")
    # português do Brasil instalado e padrão no site e no painel
    out.append(f"INSERT INTO {n('#__extensions')} {ext} VALUES\n(0, 'Portuguese (Brazil) Language Pack', 'package', 'pkg_pt-BR', '', 0, 1, 1, 0, 1, '', '', '', 0, 0);\n")
    for cli in (0, 1, 3):
        out.append(f"INSERT INTO {n('#__extensions')} {ext}\nSELECT {n('extension_id')}, 'Portuguese (Brazil)', 'language', 'pt-BR', '', {cli}, 1, 1, 0, 1, '', '', '', 0, 0 FROM {n('#__extensions')} WHERE {n('element')} = 'pkg_pt-BR';\n")
    out.append(f"UPDATE {n('#__extensions')} SET {n('params')} = '{{\"administrator\":\"pt-BR\",\"site\":\"pt-BR\"}}' WHERE {n('element')} = 'com_languages';\n")
    out.append(f"INSERT INTO {n('#__languages')} {cols('asset_id', 'lang_code', 'title', 'title_native', 'sef', 'image', 'description', 'metakey', 'metadesc', 'sitename', 'published', 'access', 'ordering')} VALUES\n(0, 'pt-BR', 'Português (Brasil)', 'Português (Brasil)', 'pt', 'pt_br', '', '', '', '', 1, 1, 2);\n")
    out.append(f"UPDATE {n('#__template_styles')} SET {n('home')} = '0' WHERE {n('client_id')} = 0;\n")
    st = cols('template', 'client_id', 'home', 'title', 'inheritable', 'parent', 'params')
    linhas = []
    for t in tpls:
        p = json.dumps({'brand': '1', 'logoFile': '', 'siteTitle': '', 'siteDescription': '', 'cor': cores[t], 'fonte': fonte.get(t, 'sem-serifa'),
                        'fluidContainer': '0', 'stickyHeader': '1', 'backTop': '1'}, separators=(',', ':'))
        linhas.append(f"('mirabilis_{t}', 0, '{'1' if t == 'rajah' else '0'}', {q('Mirabilis ' + nomes[t] + ' - Padrão')}, 0, 'cassiopeia', {q(p)})")
    out.append(f'INSERT INTO {n("#__template_styles")} {st} VALUES\n' + ',\n'.join(linhas) + ';\n')
    md = cols('title', 'note', 'content', 'ordering', 'position', 'published', 'module', 'access', 'showtitle', 'params', 'client_id', 'language')
    pr = '{"prepare_content":"0","layout":"_:default","bootstrap_size":"12","header_tag":"h2","style":"Card-default"}'
    out.append(f"INSERT INTO {n('#__modules')} {md} VALUES\n({q('Mirabilis: descreva seu site')}, '', {q(painel)}, 0, 'cpanel', 1, 'mod_custom', 1, 1, {q(pr)}, 1, '*');\n")
    out.append(f"INSERT INTO {n('#__modules_menu')} {cols('moduleid', 'menuid')} SELECT MAX({n('id')}), 0 FROM {n('#__modules')};\n")
    caminho = os.path.join(aqui, 'arquivos', 'installation', 'sql', banco, 'custom.sql')
    with open(caminho, 'w') as f:
        f.write('\n'.join(out))
    print(caminho)

for b in ('postgresql', 'mysql'):
    gerar(b)
