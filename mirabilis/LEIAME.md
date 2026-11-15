# Mirabilis

CMS baseado no Joomla 6.2 (GPL). A pessoa escreve com as próprias palavras
como quer o site e o Mirabilis escolhe o template, a cor e as seções, e monta
páginas, menus, contatos e a página inicial.

## Instalar

Igual ao Joomla: suba o conteúdo do zip para a pasta do site (no cPanel,
Gerenciador de Arquivos > Enviar > Extrair), crie um banco MySQL ou
PostgreSQL e abra o endereço do site no navegador. O instalador já registra
os seis templates e deixa o Rajah como padrão.

Pelo terminal:

    php installation/joomla.php install

## Descrever o site

No painel, o primeiro quadro é **Descreva seu site**. Escreva, por exemplo:

> Clínica veterinária chamada Pata Amiga em Curitiba, com agenda e equipe, cor verde

Clique em **Ver o plano** para conferir o que foi entendido e em **Montar o
site**. Use **com** para acrescentar uma seção, **sem** para tirar e **só**
para deixar apenas o que disser. Montar de novo troca o conteúdo de exemplo
anterior pelo novo; o que você escreveu à mão fica.

Pelo terminal:

    php cli/joomla.php mirabilis:criar "Restaurante italiano Nonna Rosa com cardápio e galeria" [--plano] [--refazer]

## Templates

| Template | Feito para |
|---|---|
| Rajah | empresas, escritórios, consultorias, tecnologia, academias |
| Villosa | blogs, revistas, portais de notícia |
| Ampullaria | lojas e catálogos |
| Lowii | saúde, bem-estar, beleza, pets |
| Truncata | restaurantes, cafés, pousadas, hotéis |
| Edwardsiana | escolas, associações, ONGs, comunidades |

Todos são filhos do Cassiopeia e têm, em Sistema > Estilos de template, a
cor principal e o tipo de letra para trocar.

## O que não é nativo

Fórum, wiki, newsletter, loja com carrinho e agenda com horários marcados
não existem no Joomla sem extensões. Quando a descrição pede uma dessas
coisas, o Mirabilis avisa em vez de fingir. A agenda vira um formulário de
contato e o catálogo vira páginas com botão de pedido.

## Construir o pacote

    python3 mirabilis/construir.py Joomla_6.2.0-Stable-Full_Package.zip saida/

Veja CREDITOS.txt para licença e marca.
