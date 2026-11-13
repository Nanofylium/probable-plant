package gerador

// Cada setor traz o que um site desse ramo costuma ter: módulos, textos de
// partida, cor, fonte e conteúdo de exemplo. Quem pede manda: o que a
// descrição disser (com, só, sem) vale mais que o padrão do setor.

type Item struct{ Icone, Titulo, Texto, Preco string }

type Pessoa struct{ Nome, Cargo, Texto string }

type Produto struct {
	Titulo, Cat, Texto, Icone string
	Preco                     float64
}

type PlanoPreco struct {
	Nome, Preco, Periodo string
	Itens                []string
	Destaque             bool
}

type Evento struct {
	Titulo, Hora, Local, Resumo string
	Dias, Vagas                 int
}

type Post struct{ Titulo, Cat, Resumo, Tags string }

type Setor struct {
	ID, Rotulo              string
	Chaves                  []string // radicais, já sem acento; "x$" = palavra inteira
	Nome, Emoji, Cor, Fonte string
	Modulos                 []string

	Chapeu, Titulo, Texto, Chamada string
	ServRotulo, ServIntro          string
	Servicos                       []Item
	EquipeRotulo                   string
	Equipe                         []Pessoa
	BlogRotulo                     string
	Cats                           []string
	Posts                          []Post
	LojaRotulo                     string
	Produtos                       []Produto
	AgendaVerbo, AgendaTitulo      string   // "Agendar", "Agende sua consulta"
	AgendaModo                     string   // "horarios" (padrão) ou "diarias"
	Horarios                       []string // horários de atendimento
	PorHorario                     int      // quantas reservas cabem no mesmo horário
	Numeros                        [][2]string
	Faq                            [][2]string
	Planos                         []PlanoPreco
	Depo                           [][2]string
	Eventos                        []Evento
	Galeria                        []string
}

var generico = Setor{
	ID: "generico", Rotulo: "empresa / negócio", Nome: "Sua Empresa", Emoji: "✦", Cor: "#2f6b4f", Fonte: "titulo-serifa",
	Modulos: []string{"servicos", "blog", "depoimentos", "faq"},
	Chapeu:  "Bem-vindo", Titulo: "Um texto curto que diz o que fazemos de melhor",
	Texto:      "Uma ou duas frases para quem chegou agora entender por que vale a pena falar com a gente.",
	Chamada:    "Vamos conversar sobre o que você precisa?",
	ServRotulo: "Serviços", ServIntro: "O que fazemos, explicado sem rodeio. Cada serviço tem a sua página.",
	Servicos: []Item{
		{"⚡", "Atendimento rápido", "Resposta no mesmo dia e prazo combinado por escrito.", ""},
		{"🎯", "Solução sob medida", "Entendemos o seu caso antes de propor qualquer coisa.", ""},
		{"🤝", "Acompanhamento", "Do primeiro contato até depois da entrega, com a mesma pessoa.", ""},
		{"🛡️", "Garantia", "Se algo não sair como combinado, a gente resolve.", ""},
	},
	EquipeRotulo: "Equipe",
	Equipe: []Pessoa{
		{"Ana Ribeiro", "Fundadora", "Cuida do atendimento e de cada projeto desde o começo."},
		{"Bruno Castro", "Operações", "Garante que prazo combinado é prazo cumprido."},
		{"Carla Nunes", "Relacionamento", "É quem responde quando você chama."},
	},
	BlogRotulo: "Blog", Cats: []string{"Novidades", "Dicas", "Bastidores"},
	Posts: []Post{
		{"Bem-vindo ao nosso novo site", "Novidades", "Agora você encontra tudo num lugar só: serviços, contato e novidades.", "site, novidades"},
		{"Três perguntas para fazer antes de contratar", "Dicas", "Um guia curto para escolher bem, com a gente ou com qualquer outro.", "dicas, guia"},
	},
	LojaRotulo: "Produtos",
	Produtos: []Produto{
		{"Vale-presente", "Presentes", "Para dar de presente: vale em qualquer serviço ou produto.", "🎁", 100},
		{"Kit de boas-vindas", "Kits", "O essencial para começar, num pacote só.", "📦", 149.9},
		{"Consultoria de 1 hora", "Serviços", "Uma hora com um especialista, online ou presencial.", "🗓️", 180},
		{"Assinatura mensal", "Planos", "Atendimento contínuo, com prioridade.", "🔁", 89.9},
	},
	AgendaVerbo: "Agendar", AgendaTitulo: "Marque seu horário", Horarios: []string{"09:00", "10:00", "11:00", "14:00", "15:00", "16:00", "17:00"}, PorHorario: 1,
	Numeros: [][2]string{{"10", "anos de estrada"}, {"1200", "clientes atendidos"}, {"98", "% recomendam"}},
	Faq: [][2]string{
		{"Como faço um orçamento?", "Mande uma mensagem pela página de contato contando o que precisa. Respondemos em até um dia útil."},
		{"Quais as formas de pagamento?", "Pix, cartão de crédito em até 6 vezes e boleto."},
		{"Vocês atendem fora da cidade?", "Sim. Boa parte do atendimento pode ser feita online."},
	},
	Planos: []PlanoPreco{
		{"Básico", "R$ 49", "/mês", []string{"Atendimento por e-mail", "1 pedido por mês", "Relatório simples"}, false},
		{"Completo", "R$ 99", "/mês", []string{"Atendimento por WhatsApp", "Pedidos ilimitados", "Relatório detalhado", "Prioridade"}, true},
		{"Empresa", "Sob consulta", "", []string{"Tudo do Completo", "Gerente dedicado", "Contrato personalizado"}, false},
	},
	Depo: [][2]string{
		{"Atendimento rápido e resultado acima do esperado. Recomendo de olhos fechados.", "Ana Souza, cliente desde 2021"},
		{"Explicaram tudo com paciência e cumpriram o prazo. Voltarei a contratar.", "Carlos Lima, empresário"},
		{"Profissionais de verdade. Dá gosto ver o cuidado com cada detalhe.", "Juliana Prates"},
	},
	Eventos: []Evento{
		{"Encontro aberto com clientes", "19:00", "Nosso espaço", "Uma conversa para apresentar as novidades do ano e ouvir sugestões.", 12, 40},
		{"Oficina prática", "14:00", "Online", "Duas horas mão na massa, com material para levar.", 26, 25},
	},
	Galeria: []string{"Nosso espaço", "Equipe em ação", "Bastidores", "Projeto entregue", "Detalhes", "Clientes felizes"},
}

var setores = []Setor{
	{
		ID: "veterinaria", Rotulo: "clínica veterinária / pet", Chaves: []string{"veterinari", "pet shop", "petshop", "pet$", "pets$", "cachorro", "gato", "animais", "banho e tosa"},
		Nome: "Patas & Cia", Emoji: "🐾", Cor: "#0e7490", Fonte: "arredondada",
		Modulos: []string{"servicos", "equipe", "agenda", "blog", "faq", "depoimentos"},
		Chapeu:  "Clínica veterinária", Titulo: "Cuidado de verdade para quem é da família",
		Texto:      "Consultas, vacinas, exames e banho e tosa com hora marcada e uma equipe que entende de bicho com medo.",
		Chamada:    "Seu pet merece um check-up. Que tal marcar hoje?",
		ServRotulo: "Serviços", ServIntro: "Do check-up à cirurgia, tudo num lugar só.",
		Servicos: []Item{
			{"🩺", "Consulta", "Avaliação completa e plano de cuidado para cães e gatos.", "150"},
			{"💉", "Vacinação", "Calendário de vacinas com lembrete da próxima dose.", "90"},
			{"🔬", "Exames", "Hemograma e ultrassom com resultado no mesmo dia.", "80"},
			{"🛁", "Banho e tosa", "Produtos hipoalergênicos e hora marcada, sem gaiola de espera.", "60"},
		},
		EquipeRotulo: "Equipe",
		Equipe: []Pessoa{
			{"Dra. Helena Prado", "Clínica geral e felinos", "Atende gatos há 12 anos."},
			{"Dr. Rafael Moura", "Cirurgia", "Responsável pelo centro cirúrgico."},
			{"Lívia Santos", "Banho e tosa", "Paciência infinita com pet ansioso."},
		},
		BlogRotulo: "Dicas", Cats: []string{"Cães", "Gatos", "Saúde", "Comportamento"},
		Posts: []Post{
			{"Calendário de vacinas do filhote", "Saúde", "Quando começar, quantas doses e quando o filhote pode passear na rua.", "vacina, filhote"},
			{"Seu gato bebe pouca água? 5 truques", "Gatos", "Gatos que bebem pouca água têm mais problema nos rins. Veja como ajudar.", "gatos, hidratação"},
		},
		LojaRotulo: "Pet shop",
		Produtos: []Produto{
			{"Ração premium 10 kg", "Alimentação", "Ração super premium para cães adultos de porte médio.", "🦴", 219.9},
			{"Areia higiênica", "Higiene", "Areia de sílica, sem cheiro, rende 30 dias.", "🐈", 49.9},
			{"Coleira peitoral", "Acessórios", "Peitoral ajustável e acolchoado, do P ao GG.", "🦮", 79.9},
			{"Brinquedo mordedor", "Brinquedos", "Borracha natural, resistente e fácil de lavar.", "🎾", 29.9},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende a consulta do seu pet", Horarios: []string{"08:00", "09:00", "10:00", "11:00", "13:00", "14:00", "15:00", "16:00", "17:00"}, PorHorario: 1,
		Numeros: [][2]string{{"12", "anos cuidando de pets"}, {"8500", "pets atendidos"}, {"24", "horas de plantão"}},
		Faq: [][2]string{
			{"Vocês atendem emergência?", "Sim, 24 horas. Ligue antes de vir para a equipe se preparar."},
			{"Preciso levar a carteirinha de vacinação?", "Sim, e exames anteriores se tiver."},
			{"Atendem animais silvestres?", "Atendemos cães, gatos, coelhos e pequenos roedores."},
		},
	},
	{
		ID: "saude", Rotulo: "clínica / saúde", Chaves: []string{"clinica", "consultorio", "medic", "dentist", "odonto", "psicolog", "fisioterap", "nutricion", "saude", "hospital", "terapia", "terapeut", "pediatr", "dermatolog"},
		Nome: "Clínica Vida Plena", Emoji: "✚", Cor: "#0f766e", Fonte: "humanista",
		Modulos: []string{"servicos", "equipe", "agenda", "blog", "faq", "depoimentos"},
		Chapeu:  "Saúde e bem-estar", Titulo: "Atendimento humano, sem pressa e sem fila",
		Texto:      "Especialistas que escutam, consultas no horário marcado e resultado explicado em português claro.",
		Chamada:    "Cuidar da saúde começa com uma consulta. Agende a sua.",
		ServRotulo: "Especialidades", ServIntro: "Profissionais de várias áreas trabalhando juntos pelo seu cuidado.",
		Servicos: []Item{
			{"🩺", "Clínica geral", "Check-up, acompanhamento e encaminhamento quando preciso.", "220"},
			{"🦷", "Odontologia", "Limpeza, restauração e clareamento.", "180"},
			{"🧠", "Psicologia", "Terapia individual e de casal, presencial ou online.", "200"},
			{"🥗", "Nutrição", "Plano alimentar possível de seguir no dia a dia.", "190"},
		},
		EquipeRotulo: "Profissionais",
		Equipe: []Pessoa{
			{"Dra. Marina Costa", "Clínica geral · CRM 00000", "Medicina de família há 15 anos."},
			{"Dr. Paulo Sena", "Odontologia · CRO 00000", "Especialista em reabilitação oral."},
			{"Júlia Fontes", "Psicologia · CRP 00/00000", "Terapia cognitivo-comportamental."},
		},
		BlogRotulo: "Artigos", Cats: []string{"Prevenção", "Alimentação", "Saúde mental", "Novidades"},
		Posts: []Post{
			{"Check-up anual: quais exames fazer", "Prevenção", "A lista básica por idade e o que cada exame mostra.", "check-up, exames"},
			{"Ansiedade: quando procurar ajuda", "Saúde mental", "Sinais de que é hora de conversar com um profissional.", "ansiedade, terapia"},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende sua consulta", Horarios: []string{"08:00", "09:00", "10:00", "11:00", "14:00", "15:00", "16:00", "17:00"}, PorHorario: 1,
		Numeros: [][2]string{{"15", "anos de clínica"}, {"20000", "consultas realizadas"}, {"9", "especialidades"}},
		Faq: [][2]string{
			{"Vocês aceitam convênio?", "Atendemos particular e emitimos recibo para reembolso do seu plano."},
			{"Posso remarcar a consulta?", "Pode, até 24 horas antes, respondendo o e-mail de confirmação."},
			{"Tem atendimento online?", "Sim, para psicologia, nutrição e retornos."},
		},
	},
	{
		ID: "beleza", Rotulo: "salão / beleza / estética", Chaves: []string{"salao", "barbearia", "barbeiro", "estetica", "beleza", "manicure", "cabeleir", "spa$", "maquiag", "sobrancelha", "unhas"},
		Nome: "Studio Bela", Emoji: "✂️", Cor: "#be185d", Fonte: "titulo-serifa",
		Modulos: []string{"servicos", "equipe", "agenda", "galeria", "depoimentos"},
		Chapeu:  "Beleza e cuidado", Titulo: "Seu momento, no seu horário",
		Texto:      "Corte, cor, unhas e estética com profissionais experientes e agenda online.",
		Chamada:    "Bora se cuidar? Escolha o horário que cabe na sua semana.",
		ServRotulo: "Serviços", ServIntro: "Tudo com produto de qualidade e hora marcada.",
		Servicos: []Item{
			{"✂️", "Corte", "Feminino, masculino e infantil, com lavagem.", "70"},
			{"🎨", "Coloração", "Mechas, luzes e cor global com produtos sem amônia.", "180"},
			{"💅", "Manicure e pedicure", "Esmaltação tradicional ou em gel.", "55"},
			{"✨", "Limpeza de pele", "Limpeza profunda com hidratação.", "140"},
		},
		EquipeRotulo: "Profissionais",
		Equipe: []Pessoa{
			{"Bia Moreira", "Cabeleireira e colorista", "Especialista em loiros."},
			{"Dani Alves", "Manicure", "Unhas em gel e nail art."},
			{"Rô Carvalho", "Esteticista", "Limpeza de pele e massagem."},
		},
		BlogRotulo: "Dicas", Cats: []string{"Cabelo", "Unhas", "Pele", "Tendências"},
		Posts: []Post{
			{"Como manter o loiro em casa", "Cabelo", "Matizador, hidratação e o que evitar entre uma visita e outra.", "loiro, cabelo"},
			{"Unha em gel: mitos e verdades", "Unhas", "Estraga a unha? Quanto dura? Respondemos tudo.", "unhas, gel"},
		},
		LojaRotulo: "Produtos",
		Produtos: []Produto{
			{"Shampoo matizador", "Cabelo", "Tira o amarelado do loiro, 300 ml.", "🧴", 69.9},
			{"Óleo reparador", "Cabelo", "Pontas secas nunca mais.", "💧", 59.9},
			{"Kit esmaltes", "Unhas", "Seis cores da estação.", "💅", 49.9},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende seu horário", Horarios: []string{"09:00", "10:00", "11:00", "13:00", "14:00", "15:00", "16:00", "17:00", "18:00"}, PorHorario: 2,
		Numeros: [][2]string{{"8", "anos de salão"}, {"4", "profissionais"}, {"4.9", "nota no Google"}},
		Faq: [][2]string{
			{"Preciso agendar?", "Recomendamos: com hora marcada você não espera."},
			{"Fazem teste de mecha?", "Sim, sem custo, antes de qualquer coloração."},
			{"Atendem noivas?", "Sim, com pacote de prova e dia da festa."},
		},
		Planos: []PlanoPreco{
			{"Mãos", "R$ 150", "/mês", []string{"4 manicures", "1 pedicure"}, false},
			{"Cabelo em dia", "R$ 220", "/mês", []string{"1 corte", "2 hidratações", "Escova ilimitada"}, true},
			{"Completo", "R$ 390", "/mês", []string{"Tudo do Cabelo em dia", "Manicure semanal", "1 limpeza de pele"}, false},
		},
		Galeria: []string{"Antes e depois: loiro", "Corte bob", "Nail art", "Nosso espaço", "Noiva", "Barba"},
	},
	{
		ID: "restaurante", Rotulo: "restaurante / padaria / café", Chaves: []string{"restaurante", "padaria", "cafeteria", "cafe$", "pizzaria", "lanchonete", "hamburgueria", "confeitaria", "bar$", "doceria", "bistro", "comida", "sorveteria", "acai", "cozinha", "marmita", "buffet"},
		Nome: "Sabor da Casa", Emoji: "🍽️", Cor: "#b45309", Fonte: "titulo-serifa",
		Modulos: []string{"catalogo", "agenda", "galeria", "depoimentos", "faq"},
		Chapeu:  "Comida de verdade", Titulo: "Feito na hora, com ingrediente bom e tempero de casa",
		Texto:      "Veja o cardápio, faça o pedido pelo site e retire ou receba em casa. Ou reserve uma mesa.",
		Chamada:    "Bateu a fome? O cardápio está a um clique.",
		ServRotulo: "Serviços", ServIntro: "Além do salão, a gente vai até você.",
		Servicos: []Item{
			{"🛵", "Delivery", "Entregamos num raio de 5 km.", ""},
			{"🎉", "Eventos", "Buffet para aniversários e confraternizações.", ""},
			{"☕", "Café da manhã", "De segunda a sábado, das 7h às 10h.", ""},
		},
		BlogRotulo: "Novidades", Cats: []string{"Cardápio", "Receitas", "Eventos"},
		Posts: []Post{
			{"Prato novo no cardápio", "Cardápio", "Chegou o prato que vocês pediram. Vem provar!", "cardápio, novidade"},
			{"A receita do nosso pão de queijo", "Receitas", "Contamos o segredo (quase todo).", "receita"},
		},
		LojaRotulo: "Cardápio",
		Produtos: []Produto{
			{"Prato executivo", "Pratos", "Arroz, feijão, salada e a carne do dia.", "🍛", 32},
			{"Lasanha à bolonhesa", "Pratos", "Massa fresca e molho de 6 horas.", "🍝", 42},
			{"Salada da casa", "Pratos", "Folhas, grão-de-bico, tomate e molho de mostarda e mel.", "🥗", 28},
			{"Pão de queijo (6 un.)", "Lanches", "Assado na hora.", "🧀", 15},
			{"Suco natural", "Bebidas", "Laranja, limão ou maracujá, 500 ml.", "🍊", 12},
			{"Pudim de leite", "Sobremesas", "A receita da vó.", "🍮", 14},
		},
		AgendaVerbo: "Reservar", AgendaTitulo: "Reserve sua mesa", Horarios: []string{"12:00", "12:30", "13:00", "13:30", "19:00", "19:30", "20:00", "20:30", "21:00"}, PorHorario: 6,
		Numeros: [][2]string{{"20", "anos de casa"}, {"300", "refeições por dia"}, {"100", "% feito aqui"}},
		Faq: [][2]string{
			{"Vocês entregam?", "Sim, num raio de 5 km. Faça o pedido pelo cardápio do site."},
			{"Tem opção vegetariana?", "Sempre. Pergunte também pelo prato do dia sem carne."},
			{"Aceitam reserva para grupos?", "Sim, até 20 pessoas. Para mais, fale com a gente."},
		},
		Eventos: []Evento{
			{"Noite de massas", "19:30", "Salão principal", "Rodízio de massas frescas com música ao vivo.", 9, 60},
			{"Oficina de pão caseiro", "15:00", "Nossa cozinha", "Aprenda a fazer pão de fermentação natural.", 20, 12},
		},
		Galeria: []string{"Nosso salão", "Prato do dia", "Cozinha", "Sobremesas", "Varanda", "Equipe"},
	},
	{
		ID: "hotel", Rotulo: "pousada / hotel", Chaves: []string{"hotel", "pousada", "hostel", "chale", "hospedag", "resort", "camping", "albergue"},
		Nome: "Pousada Maré Mansa", Emoji: "🌊", Cor: "#0e7490", Fonte: "titulo-serifa",
		Modulos: []string{"servicos", "agenda", "galeria", "depoimentos", "faq"},
		Chapeu:  "Hospedagem", Titulo: "Descanso de verdade, a poucos passos do mar",
		Texto:      "Quartos confortáveis, café da manhã caprichado e reserva online com confirmação na hora.",
		Chamada:    "Suas férias começam aqui. Veja as datas livres.",
		ServRotulo: "Acomodações", ServIntro: "Todas com ar-condicionado, Wi-Fi e café da manhã incluso.",
		Servicos: []Item{
			{"🛏️", "Quarto Standard", "Cama de casal, ventilador de teto e varanda.", "280"},
			{"🌅", "Quarto Vista Mar", "Varanda de frente para o mar e banheira.", "420"},
			{"👨‍👩‍👧", "Suíte Família", "Até 4 pessoas, com dois ambientes.", "520"},
		},
		BlogRotulo: "Roteiros", Cats: []string{"Passeios", "Gastronomia", "Novidades"},
		Posts: []Post{
			{"5 passeios imperdíveis na região", "Passeios", "Praias, trilhas e mirantes a menos de 30 minutos daqui.", "passeios, praia"},
			{"Onde comer bem perto da pousada", "Gastronomia", "Nossa lista de restaurantes testados e aprovados.", "restaurantes"},
		},
		AgendaVerbo: "Reservar", AgendaTitulo: "Reserve sua estadia", AgendaModo: "diarias",
		Numeros: [][2]string{{"50", "metros da praia"}, {"12", "acomodações"}, {"4.9", "nota dos hóspedes"}},
		Faq: [][2]string{
			{"Qual o horário de check-in e check-out?", "Check-in a partir das 14h e check-out até as 12h."},
			{"Aceitam pets?", "Sim, pets de pequeno porte, com aviso prévio."},
			{"Tem estacionamento?", "Sim, gratuito e coberto."},
		},
		Galeria: []string{"Piscina", "Café da manhã", "Quarto Vista Mar", "Jardim", "Pôr do sol", "Recepção"},
	},
	{
		ID: "escola", Rotulo: "escola / cursos", Chaves: []string{"escola", "curso", "colegio", "faculdade", "aulas", "ensino", "creche", "professor", "educac", "treinamento", "conservatorio", "idiomas"},
		Nome: "Escola Aprender", Emoji: "🎓", Cor: "#1d4ed8", Fonte: "geometrica",
		Modulos: []string{"servicos", "equipe", "eventos", "blog", "faq", "depoimentos"},
		Chapeu:  "Matrículas abertas", Titulo: "Aprender com gosto, do primeiro dia ao certificado",
		Texto:      "Turmas pequenas, professores que acompanham de perto e aulas presenciais ou online.",
		Chamada:    "Venha fazer uma aula experimental, sem compromisso.",
		ServRotulo: "Cursos", ServIntro: "Do básico ao avançado, para todas as idades.",
		Servicos: []Item{
			{"🌱", "Iniciante", "Para quem nunca estudou: base sólida e sem pressa.", "290"},
			{"📘", "Intermediário", "Aprofunde o que já sabe com projetos práticos.", "320"},
			{"🏆", "Avançado", "Preparação para provas, certificações e palco.", "380"},
			{"🧒", "Infantil", "Aprender brincando, de 6 a 11 anos.", "260"},
		},
		EquipeRotulo: "Professores",
		Equipe: []Pessoa{
			{"Prof.ª Renata Lopes", "Coordenação pedagógica", "Mestre em educação, 18 anos de sala de aula."},
			{"Prof. André Silva", "Turmas avançadas", "Prepara alunos para provas e concursos."},
			{"Prof.ª Camila Rocha", "Turmas infantis", "Especialista em ensino para crianças."},
		},
		BlogRotulo: "Notícias", Cats: []string{"Escola", "Dicas de estudo", "Eventos", "Alunos"},
		Posts: []Post{
			{"Matrículas abertas para o próximo semestre", "Escola", "Turmas novas de manhã, tarde e noite. Veja como garantir a vaga.", "matrícula"},
			{"Como estudar 30 minutos por dia e render mais", "Dicas de estudo", "Um método simples que nossos alunos usam.", "estudo, rotina"},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende uma aula experimental", Horarios: []string{"09:00", "10:00", "14:00", "15:00", "18:00", "19:00"}, PorHorario: 3,
		Numeros: [][2]string{{"15", "anos ensinando"}, {"3000", "alunos formados"}, {"12", "alunos por turma, no máximo"}},
		Faq: [][2]string{
			{"Tem aula experimental?", "Sim, gratuita. Agende pelo site ou WhatsApp."},
			{"Os cursos dão certificado?", "Sim, ao fim de cada nível, com carga horária."},
			{"Posso trancar a matrícula?", "Pode, uma vez por ano, sem multa."},
		},
		Planos: []PlanoPreco{
			{"Mensal", "R$ 320", "/mês", []string{"2 aulas por semana", "Material digital", "Sem fidelidade"}, false},
			{"Semestral", "R$ 280", "/mês", []string{"2 aulas por semana", "Material impresso", "Aula de reforço"}, true},
			{"Anual", "R$ 250", "/mês", []string{"Tudo do Semestral", "Certificado internacional", "Evento de formatura"}, false},
		},
		Eventos: []Evento{
			{"Aula aberta para a comunidade", "10:00", "Auditório", "Venha conhecer a escola e assistir a uma aula de verdade.", 10, 50},
			{"Apresentação de fim de semestre", "19:00", "Teatro municipal", "Os alunos mostram o que aprenderam. Entrada gratuita com inscrição.", 35, 200},
		},
		Galeria: []string{"Sala de aula", "Formatura", "Apresentação", "Biblioteca", "Laboratório", "Pátio"},
	},
	{
		ID: "academia", Rotulo: "academia / estúdio", Chaves: []string{"academia", "crossfit", "pilates", "yoga", "ioga", "musculac", "personal", "danca", "luta", "jiu", "funcional", "natacao", "treino"},
		Nome: "Academia Movimento", Emoji: "💪", Cor: "#c2410c", Fonte: "geometrica",
		Modulos: []string{"servicos", "planos", "equipe", "agenda", "depoimentos", "faq"},
		Chapeu:  "Treine com quem entende", Titulo: "Mais saúde, mais disposição, no seu ritmo",
		Texto:      "Musculação, funcional e aulas coletivas com acompanhamento de verdade. A primeira aula é por nossa conta.",
		Chamada:    "Agende sua aula experimental gratuita.",
		ServRotulo: "Modalidades", ServIntro: "Escolha a sua ou combine várias no mesmo plano.",
		Servicos: []Item{
			{"🏋️", "Musculação", "Treino montado por professor e revisto todo mês.", ""},
			{"🔥", "Funcional", "Aulas de 45 minutos em grupo, cheias de energia.", ""},
			{"🧘", "Pilates e yoga", "Postura, força e respiração.", ""},
			{"🥊", "Lutas", "Muay thai e boxe para iniciantes e avançados.", ""},
		},
		EquipeRotulo: "Professores",
		Equipe: []Pessoa{
			{"Léo Martins", "Musculação · CREF 000000", "Especialista em hipertrofia e reabilitação."},
			{"Fernanda Dias", "Funcional e pilates", "Aulas que passam voando."},
			{"Diego Ramos", "Lutas", "Faixa preta e paciência de professor."},
		},
		BlogRotulo: "Blog", Cats: []string{"Treino", "Alimentação", "Motivação"},
		Posts: []Post{
			{"Treino de 30 minutos para quem não tem tempo", "Treino", "Um circuito completo que cabe no horário de almoço.", "treino, rápido"},
			{"O que comer antes de treinar", "Alimentação", "Opções simples para ter energia sem pesar.", "alimentação"},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende sua aula experimental", Horarios: []string{"06:00", "07:00", "08:00", "12:00", "17:00", "18:00", "19:00", "20:00"}, PorHorario: 4,
		Numeros: [][2]string{{"1500", "alunos ativos"}, {"60", "aulas por semana"}, {"6", "às 23h, todo dia útil"}},
		Faq: [][2]string{
			{"Preciso de exame médico?", "Pedimos um atestado simples ou o questionário de prontidão na matrícula."},
			{"Tem fidelidade?", "Só no plano anual, que tem o melhor preço."},
			{"Posso congelar o plano?", "Sim, até 30 dias por ano."},
		},
		Planos: []PlanoPreco{
			{"Mensal", "R$ 119", "/mês", []string{"Musculação", "Avaliação física", "Sem fidelidade"}, false},
			{"Total", "R$ 159", "/mês", []string{"Musculação", "Todas as aulas coletivas", "App de treino", "Avaliação mensal"}, true},
			{"Anual", "R$ 99", "/mês", []string{"Tudo do Total", "2 meses de bônus", "Leve um amigo 1x por mês"}, false},
		},
		Eventos: []Evento{
			{"Aulão beneficente", "09:00", "Praça central", "Aula aberta de funcional. Doe 1 kg de alimento.", 15, 120},
			{"Desafio 30 dias", "07:00", "Na academia", "Um mês de metas com prêmio para quem completar.", 5, 80},
		},
		Galeria: []string{"Sala de musculação", "Aula de funcional", "Estúdio de pilates", "Ringue", "Vestiário", "Recepção"},
	},
	{
		ID: "advocacia", Rotulo: "escritório / serviços profissionais", Chaves: []string{"advoca", "advogad", "juridic", "escritorio", "contabil", "contador", "consultoria", "arquitet", "engenharia", "imobiliaria", "corretor", "despachante"},
		Nome: "Moura & Prado Advogados", Emoji: "⚖️", Cor: "#1e3a5f", Fonte: "classica",
		Modulos: []string{"servicos", "equipe", "blog", "faq", "depoimentos"},
		Chapeu:  "Escritório", Titulo: "Orientação clara para decisões importantes",
		Texto:      "Atendimento próximo, linguagem sem juridiquês e acompanhamento de cada etapa do seu caso.",
		Chamada:    "Conte o seu caso. A primeira conversa é sem compromisso.",
		ServRotulo: "Áreas de atuação", ServIntro: "Cada área com um profissional responsável.",
		Servicos: []Item{
			{"👪", "Família", "Divórcio, guarda, pensão e inventário.", ""},
			{"💼", "Trabalhista", "Para empresas e trabalhadores.", ""},
			{"🏠", "Imobiliário", "Contratos, usucapião e regularização.", ""},
			{"🏢", "Empresarial", "Abertura de empresa, contratos e recuperação de crédito.", ""},
		},
		EquipeRotulo: "Equipe",
		Equipe: []Pessoa{
			{"Dra. Luísa Moura", "Sócia · OAB/SP 000.000", "Direito de família e sucessões."},
			{"Dr. Henrique Prado", "Sócio · OAB/SP 000.001", "Direito empresarial e contratos."},
			{"Dra. Paula Teixeira", "Associada · OAB/SP 000.002", "Direito do trabalho."},
		},
		BlogRotulo: "Artigos", Cats: []string{"Família", "Trabalho", "Empresas", "Imóveis"},
		Posts: []Post{
			{"Inventário: quanto tempo leva e quanto custa", "Família", "O passo a passo do inventário em cartório e na Justiça.", "inventário, herança"},
			{"Demissão: quais direitos você tem", "Trabalho", "Aviso prévio, FGTS e seguro-desemprego explicados.", "demissão, direitos"},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende uma conversa", Horarios: []string{"09:00", "10:00", "11:00", "14:00", "15:00", "16:00"}, PorHorario: 1,
		Numeros: [][2]string{{"20", "anos de atuação"}, {"2500", "casos acompanhados"}, {"4", "áreas do direito"}},
		Faq: [][2]string{
			{"A primeira consulta é paga?", "A primeira conversa, de até 30 minutos, é sem custo."},
			{"Atendem online?", "Sim, por videochamada, em todo o Brasil."},
			{"Como acompanho meu processo?", "Você recebe atualizações por e-mail a cada movimentação."},
		},
	},
	{
		ID: "tecnologia", Rotulo: "tecnologia / agência", Chaves: []string{"tecnologia", "software", "startup", "agencia", "aplicativo", "app$", "sistema", "desenvolviment", "marketing digital", "ti$", "design", "saas"},
		Nome: "Nuvem Azul", Emoji: "◆", Cor: "#4338ca", Fonte: "geometrica",
		Modulos: []string{"servicos", "galeria", "planos", "blog", "depoimentos", "faq"},
		Chapeu:  "Tecnologia que resolve", Titulo: "Software sob medida para a sua empresa crescer",
		Texto:      "Sites, sistemas e aplicativos feitos com método, prazo claro e suporte depois da entrega.",
		Chamada:    "Tem um projeto em mente? Vamos tirar do papel.",
		ServRotulo: "Soluções", ServIntro: "Do primeiro protótipo à operação no ar.",
		Servicos: []Item{
			{"🌐", "Sites", "Rápidos, bonitos e fáceis de atualizar.", ""},
			{"📱", "Aplicativos", "Android e iPhone com um código só.", ""},
			{"⚙️", "Sistemas", "Automatize o que hoje vive em planilha.", ""},
			{"📈", "Marketing digital", "Anúncios e conteúdo que trazem cliente.", ""},
		},
		EquipeRotulo: "Time",
		Equipe: []Pessoa{
			{"Rafa Teles", "CEO", "15 anos construindo produtos digitais."},
			{"Nina Okada", "Design", "Interfaces que qualquer pessoa entende."},
			{"Igor Brandão", "Engenharia", "Código limpo e sistemas que não caem."},
		},
		BlogRotulo: "Blog", Cats: []string{"Produto", "Engenharia", "Marketing", "Cases"},
		Posts: []Post{
			{"Quanto custa fazer um aplicativo", "Produto", "Os fatores que mudam o preço e como economizar sem perder qualidade.", "aplicativo, custo"},
			{"Planilha ou sistema? Como saber a hora de mudar", "Engenharia", "Cinco sinais de que a planilha já não dá conta.", "sistemas, automação"},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende uma conversa", Horarios: []string{"09:00", "10:00", "11:00", "14:00", "15:00", "16:00", "17:00"}, PorHorario: 1,
		Numeros: [][2]string{{"120", "projetos entregues"}, {"40", "clientes ativos"}, {"99.9", "% de disponibilidade"}},
		Faq: [][2]string{
			{"Quanto tempo leva um projeto?", "Um site, de 2 a 4 semanas. Sistemas, combinamos por etapas."},
			{"Vocês dão suporte depois?", "Sim, com planos mensais de manutenção."},
			{"O código fica comigo?", "Fica. Você é dono de tudo o que pagamos para fazer."},
		},
		Planos: []PlanoPreco{
			{"Site", "R$ 2.900", "uma vez", []string{"Até 8 páginas", "Painel para editar", "Hospedagem no 1º ano"}, false},
			{"Crescimento", "R$ 1.490", "/mês", []string{"Site + blog", "Gestão de anúncios", "Relatório mensal"}, true},
			{"Sob medida", "Sob consulta", "", []string{"Sistemas e aplicativos", "Equipe dedicada", "Contrato por etapas"}, false},
		},
		Galeria: []string{"App de delivery", "Sistema de estoque", "Site institucional", "Painel de vendas", "Loja virtual", "Identidade visual"},
	},
	{
		ID: "loja", Rotulo: "loja / vendas", Chaves: []string{"loja", "e-commerce", "ecommerce", "boutique", "moda", "roupa", "artesanato", "presente", "vender", "venda", "brecho", "produtos", "acessorios"},
		Nome: "Loja Ipê", Emoji: "🛍️", Cor: "#7c3aed", Fonte: "geometrica",
		Modulos: []string{"catalogo", "blog", "faq", "depoimentos", "newsletter"},
		Chapeu:  "Coleção nova", Titulo: "Peças escolhidas a dedo, entregues na sua porta",
		Texto:      "Compre pelo site, pague como preferir e receba em casa ou retire na loja.",
		Chamada:    "Frete grátis acima de R$ 199. Aproveite.",
		ServRotulo: "Serviços", ServIntro: "Comprar com a gente é simples.",
		Servicos: []Item{
			{"🚚", "Entrega rápida", "Em até 2 dias úteis na cidade.", ""},
			{"🔁", "Troca fácil", "30 dias para trocar, sem burocracia.", ""},
			{"🎁", "Embrulho de presente", "Grátis em qualquer compra.", ""},
		},
		BlogRotulo: "Blog", Cats: []string{"Coleção", "Estilo", "Novidades"},
		Posts: []Post{
			{"Chegou a coleção de verão", "Coleção", "Tecidos leves e cores da estação.", "coleção, verão"},
			{"Como montar um guarda-roupa cápsula", "Estilo", "Menos peças, mais combinações.", "estilo, dicas"},
		},
		LojaRotulo: "Loja",
		Produtos: []Produto{
			{"Camisa de linho", "Roupas", "Linho puro, corte reto, do P ao GG.", "👕", 189.9},
			{"Vestido midi", "Roupas", "Viscose estampada com bolsos.", "👗", 229.9},
			{"Bolsa de couro", "Acessórios", "Couro legítimo, feita à mão.", "👜", 349},
			{"Brinco de prata", "Acessórios", "Prata 925, hipoalergênico.", "💍", 99.9},
			{"Vela aromática", "Casa", "Cera vegetal, 40 horas de queima.", "🕯️", 59.9},
			{"Caneca artesanal", "Casa", "Cerâmica feita à mão; cada uma é única.", "☕", 69.9},
		},
		Numeros: [][2]string{{"5000", "clientes"}, {"2", "dias para entregar"}, {"30", "dias para trocar"}},
		Faq: [][2]string{
			{"Qual o prazo de entrega?", "Na cidade, até 2 dias úteis. Para outras cidades, de 3 a 8 dias úteis."},
			{"Como faço uma troca?", "Fale com a gente em até 30 dias com a peça sem uso e com etiqueta."},
			{"Quais as formas de pagamento?", "Pix com 5% de desconto, cartão em até 6 vezes ou boleto."},
		},
		Galeria: []string{"Vitrine", "Coleção verão", "Provador", "Embalagem", "Acessórios", "Nossa loja"},
	},
	{
		ID: "associacao", Rotulo: "associação / ONG / igreja", Chaves: []string{"associac", "ong$", "igreja", "paroquia", "moradores", "condominio", "clube", "sindicato", "cooperativa", "instituto", "projeto social", "voluntari", "fundacao", "coletivo"},
		Nome: "Associação Viva o Bairro", Emoji: "🤝", Cor: "#15803d", Fonte: "humanista",
		Modulos: []string{"blog", "eventos", "downloads", "galeria", "faq", "newsletter"},
		Chapeu:  "Juntos somos mais", Titulo: "Gente que se junta para cuidar do lugar onde vive",
		Texto:      "Notícias, assembleias, eventos e documentos da associação num lugar só, aberto a todos.",
		Chamada:    "Participe. Toda ajuda conta.",
		ServRotulo: "O que fazemos", ServIntro: "Nossas frentes de trabalho.",
		Servicos: []Item{
			{"🌳", "Meio ambiente", "Mutirões de limpeza e plantio de árvores.", ""},
			{"📚", "Educação", "Reforço escolar e biblioteca comunitária.", ""},
			{"🛡️", "Segurança", "Rede de vizinhos e diálogo com o poder público.", ""},
			{"🎭", "Cultura", "Festas, feiras e oficinas para todas as idades.", ""},
		},
		EquipeRotulo: "Diretoria",
		Equipe: []Pessoa{
			{"Maria das Graças", "Presidente", "Moradora há 30 anos e voz da associação."},
			{"João Batista", "Tesoureiro", "Presta contas todo mês, com recibo."},
			{"Sônia Freitas", "Secretária", "Cuida das atas e da comunicação."},
		},
		BlogRotulo: "Notícias", Cats: []string{"Assembleias", "Conquistas", "Avisos", "Eventos"},
		Posts: []Post{
			{"Praça reformada: conquista dos moradores", "Conquistas", "Depois de dois anos de pedidos, a praça ganhou iluminação e brinquedos novos.", "praça, conquista"},
			{"Resumo da última assembleia", "Assembleias", "O que foi decidido e os próximos passos.", "assembleia"},
		},
		AgendaVerbo: "Agendar", AgendaTitulo: "Agende um atendimento", Horarios: []string{"09:00", "10:00", "14:00", "15:00"}, PorHorario: 2,
		Numeros: [][2]string{{"25", "anos de história"}, {"800", "famílias associadas"}, {"40", "voluntários"}},
		Faq: [][2]string{
			{"Como me associo?", "Preencha a ficha de associação (está em Documentos) e entregue na sede ou pelo site."},
			{"Quanto custa?", "A contribuição é voluntária, a partir de R$ 10 por mês."},
			{"Onde vejo a prestação de contas?", "Todo mês publicamos o balancete em Documentos."},
		},
		Planos: []PlanoPreco{
			{"Amigo", "R$ 10", "/mês", []string{"Carteirinha", "Voto na assembleia"}, false},
			{"Apoiador", "R$ 30", "/mês", []string{"Tudo do Amigo", "Nome no mural de apoiadores"}, true},
			{"Padrinho", "R$ 100", "/mês", []string{"Apoia uma criança no reforço escolar", "Relatório trimestral"}, false},
		},
		Eventos: []Evento{
			{"Assembleia geral", "19:00", "Salão da associação", "Pauta: prestação de contas e eleição da nova diretoria.", 8, 150},
			{"Mutirão de plantio", "08:00", "Praça central", "Traga luvas e disposição. Mudas por nossa conta.", 18, 50},
			{"Festa junina do bairro", "17:00", "Rua de lazer", "Quadrilha, comidas típicas e sorteios.", 40, 400},
		},
		Galeria: []string{"Mutirão", "Festa junina", "Assembleia", "Praça reformada", "Biblioteca", "Voluntários"},
	},
	{
		ID: "comunidade", Rotulo: "comunidade / documentação / wiki", Chaves: []string{"wiki", "documentac", "base de conhecimento", "forum", "enciclopedia", "open source", "codigo aberto", "comunidade", "fa clube", "fas$", "projeto livre", "manual"},
		Nome: "Comunidade Jarro Aberto", Emoji: "📖", Cor: "#0f766e", Fonte: "humanista",
		Modulos: []string{"wiki", "forum", "blog", "downloads", "faq"},
		Chapeu:  "Feito por muita gente", Titulo: "O conhecimento da comunidade, organizado e aberto",
		Texto:      "Uma wiki que todo mundo pode melhorar, um fórum para tirar dúvidas e as novidades do projeto.",
		Chamada:    "Encontrou algo para melhorar? Sugira uma edição.",
		ServRotulo: "O que tem aqui", ServIntro: "Comece por onde fizer mais sentido para você.",
		Servicos: []Item{
			{"📖", "Wiki", "Guias, tutoriais e referência, escritos pela comunidade.", ""},
			{"💬", "Fórum", "Pergunte, responda e compartilhe.", ""},
			{"📦", "Downloads", "Versões, manuais e materiais.", ""},
		},
		EquipeRotulo: "Mantenedores",
		Equipe: []Pessoa{
			{"Tainá Lopes", "Coordenação", "Organiza a wiki e os encontros."},
			{"Marcos Vieira", "Moderação", "Mantém o fórum acolhedor."},
			{"Yuri Sato", "Documentação", "Revisa as sugestões de edição."},
		},
		BlogRotulo: "Novidades", Cats: []string{"Lançamentos", "Comunidade", "Tutoriais"},
		Posts: []Post{
			{"Versão nova no ar", "Lançamentos", "O que mudou e como atualizar.", "versão, lançamento"},
			{"Como contribuir pela primeira vez", "Comunidade", "Do primeiro comentário à primeira página da wiki.", "contribuir"},
		},
		Numeros: [][2]string{{"350", "páginas na wiki"}, {"2000", "membros"}, {"15", "mil respostas no fórum"}},
		Faq: [][2]string{
			{"Preciso me cadastrar para ler?", "Não. Tudo é aberto. Para sugerir edições basta informar nome e e-mail."},
			{"Quem aprova as edições?", "Os mantenedores revisam cada sugestão, normalmente em até dois dias."},
			{"Posso usar o conteúdo?", "Sim, citando a fonte."},
		},
		Eventos: []Evento{
			{"Encontro mensal online", "20:00", "Online", "Uma hora para conversar sobre os rumos do projeto.", 7, 300},
			{"Maratona de documentação", "10:00", "Online", "Um sábado inteiro melhorando a wiki juntos.", 21, 100},
		},
	},
}
