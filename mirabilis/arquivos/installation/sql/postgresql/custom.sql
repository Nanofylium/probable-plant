-- Mirabilis: registra as extensões próprias (gerado por gerar_sql.py)

INSERT INTO "#__extensions" ("package_id", "name", "type", "element", "folder", "client_id", "enabled", "access", "protected", "locked", "manifest_cache", "params", "custom_data", "ordering", "state") VALUES
(0, 'com_mirabilis', 'component', 'com_mirabilis', '', 1, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'plg_console_mirabilis', 'plugin', 'mirabilis', 'console', 0, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'mirabilis_rajah', 'template', 'mirabilis_rajah', '', 0, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'mirabilis_villosa', 'template', 'mirabilis_villosa', '', 0, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'mirabilis_ampullaria', 'template', 'mirabilis_ampullaria', '', 0, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'mirabilis_lowii', 'template', 'mirabilis_lowii', '', 0, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'mirabilis_truncata', 'template', 'mirabilis_truncata', '', 0, 1, 1, 0, 1, '', '{}', '', 0, 0),
(0, 'mirabilis_edwardsiana', 'template', 'mirabilis_edwardsiana', '', 0, 1, 1, 0, 1, '', '{}', '', 0, 0);

UPDATE "#__extensions" SET "enabled" = 0 WHERE "type" = 'plugin' AND "folder" = 'system' AND "element" = 'stats';

INSERT INTO "#__extensions" ("package_id", "name", "type", "element", "folder", "client_id", "enabled", "access", "protected", "locked", "manifest_cache", "params", "custom_data", "ordering", "state") VALUES
(0, 'Portuguese (Brazil) Language Pack', 'package', 'pkg_pt-BR', '', 0, 1, 1, 0, 1, '', '', '', 0, 0);

INSERT INTO "#__extensions" ("package_id", "name", "type", "element", "folder", "client_id", "enabled", "access", "protected", "locked", "manifest_cache", "params", "custom_data", "ordering", "state")
SELECT "extension_id", 'Portuguese (Brazil)', 'language', 'pt-BR', '', 0, 1, 1, 0, 1, '', '', '', 0, 0 FROM "#__extensions" WHERE "element" = 'pkg_pt-BR';

INSERT INTO "#__extensions" ("package_id", "name", "type", "element", "folder", "client_id", "enabled", "access", "protected", "locked", "manifest_cache", "params", "custom_data", "ordering", "state")
SELECT "extension_id", 'Portuguese (Brazil)', 'language', 'pt-BR', '', 1, 1, 1, 0, 1, '', '', '', 0, 0 FROM "#__extensions" WHERE "element" = 'pkg_pt-BR';

INSERT INTO "#__extensions" ("package_id", "name", "type", "element", "folder", "client_id", "enabled", "access", "protected", "locked", "manifest_cache", "params", "custom_data", "ordering", "state")
SELECT "extension_id", 'Portuguese (Brazil)', 'language', 'pt-BR', '', 3, 1, 1, 0, 1, '', '', '', 0, 0 FROM "#__extensions" WHERE "element" = 'pkg_pt-BR';

UPDATE "#__extensions" SET "params" = '{"administrator":"pt-BR","site":"pt-BR"}' WHERE "element" = 'com_languages';

INSERT INTO "#__languages" ("asset_id", "lang_code", "title", "title_native", "sef", "image", "description", "metakey", "metadesc", "sitename", "published", "access", "ordering") VALUES
(0, 'pt-BR', 'Português (Brasil)', 'Português (Brasil)', 'pt', 'pt_br', '', '', '', '', 1, 1, 2);

UPDATE "#__template_styles" SET "home" = '0' WHERE "client_id" = 0;

INSERT INTO "#__template_styles" ("template", "client_id", "home", "title", "inheritable", "parent", "params") VALUES
('mirabilis_rajah', 0, '1', 'Mirabilis Rajah - Padrão', 0, 'cassiopeia', '{"brand":"1","logoFile":"","siteTitle":"","siteDescription":"","cor":"#2f6b4f","fonte":"sem-serifa","fluidContainer":"0","stickyHeader":"1","backTop":"1"}'),
('mirabilis_villosa', 0, '0', 'Mirabilis Villosa - Padrão', 0, 'cassiopeia', '{"brand":"1","logoFile":"","siteTitle":"","siteDescription":"","cor":"#9b2335","fonte":"titulo-serifa","fluidContainer":"0","stickyHeader":"1","backTop":"1"}'),
('mirabilis_ampullaria', 0, '0', 'Mirabilis Ampullaria - Padrão', 0, 'cassiopeia', '{"brand":"1","logoFile":"","siteTitle":"","siteDescription":"","cor":"#7c3aed","fonte":"sem-serifa","fluidContainer":"0","stickyHeader":"1","backTop":"1"}'),
('mirabilis_lowii', 0, '0', 'Mirabilis Lowii - Padrão', 0, 'cassiopeia', '{"brand":"1","logoFile":"","siteTitle":"","siteDescription":"","cor":"#0f766e","fonte":"sem-serifa","fluidContainer":"0","stickyHeader":"1","backTop":"1"}'),
('mirabilis_truncata', 0, '0', 'Mirabilis Truncata - Padrão', 0, 'cassiopeia', '{"brand":"1","logoFile":"","siteTitle":"","siteDescription":"","cor":"#b45309","fonte":"titulo-serifa","fluidContainer":"0","stickyHeader":"1","backTop":"1"}'),
('mirabilis_edwardsiana', 0, '0', 'Mirabilis Edwardsiana - Padrão', 0, 'cassiopeia', '{"brand":"1","logoFile":"","siteTitle":"","siteDescription":"","cor":"#1d4ed8","fonte":"sem-serifa","fluidContainer":"0","stickyHeader":"1","backTop":"1"}');

INSERT INTO "#__modules" ("title", "note", "content", "ordering", "position", "published", "module", "access", "showtitle", "params", "client_id", "language") VALUES
('Mirabilis: descreva seu site', '', '<div class="mira-dash" style="display:flex;flex-wrap:wrap;gap:1rem;align-items:center;justify-content:space-between"><p style="margin:0;font-size:1.05rem">Escreva com suas palavras como quer o site e o Mirabilis escolhe o template e monta páginas, menus e a página inicial.</p><a class="btn btn-success btn-lg" href="index.php?option=com_mirabilis">Descreva seu site</a></div>', 0, 'cpanel', 1, 'mod_custom', 1, 1, '{"prepare_content":"0","layout":"_:default","bootstrap_size":"12","header_tag":"h2","style":"Card-default"}', 1, '*');

INSERT INTO "#__modules_menu" ("moduleid", "menuid") SELECT MAX("id"), 0 FROM "#__modules";
