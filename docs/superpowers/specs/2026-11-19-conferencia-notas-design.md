# rtc-hub — Módulo 2: Conferência de notas fiscais — Design

Data: 2026-11-19 · Status: v2 (builder check incorporado), aguardando aprovação

## Intenção e porquê

O público mudou de novo. O rtc-hub deixa de ser só uma vitrine para entrevista e
passa a ser um sistema que **um contador de verdade usa com os clientes dele**. O
Lucas quer que ele esteja pronto para mostrar a um contador ainda indefinido
e que atenda **o maior número possível de escritórios**, usando o que os repositórios
da Receita oferecem.

O módulo 1 classifica o **cadastro** (o que cada item *deveria* ter). O módulo 2
confere o que **foi emitido de fato**: o escritório sobe os XMLs de saída do mês de
um cliente e o sistema aponta, nota por nota e item por item, IBS/CBS/IS faltando,
com classificação errada ou com valor errado.

Isso serve a qualquer escritório. Desde 2026 as NF-e/NFC-e/NFS-e precisam trazer
o grupo IBS/CBS (alíquotas de teste de 0,9% de CBS e 0,1% de IBS), e o contador
precisa saber se o ERP do cliente está preenchendo certo. A mensagem do módulo 1
continua valendo: **"o que vocês fariam em horas, o sistema faz em minutos, com as
ferramentas oficiais da Receita"**.

## O que os motores oferecem (verificado em 2026-11-19)

- **Calculadora regime regular** (já em uso para cálculo e tabelas):
  - `POST /calculadora/xml/validate?tipo=NFE|NFCE|NFSE…&subtipo=NOTA|GRUPO` valida o
    XML contra os XSD oficiais embarcados. Devolve `true` ou um erro com linha,
    coluna e mensagem do validador. Os exemplos válidos em
    `src/test/resources/xml/nfe/valido/nota/` têm raiz `enviNFe`; conferir se um
    `nfeProc` (o que o contador tem em mãos) passa ou se é preciso validar só o
    grupo (`subtipo=GRUPO`).
  - `POST /calculadora/xml/generate` gera o grupo IBS/CBS correto a partir de um cálculo.
  - `/calculadora/nfse/*`: base de cálculo da NFS-e, validação do indicador de
    operação, local de incidência.
  - `/classificacoes-tributarias/cbs-ibs/{siglaDfe}/{cClassTrib}` e
    `/nomenclatura/{siglaDfe}/{cClassTrib}` dizem se um cClassTrib vale para aquele
    tipo de documento.
- **Calculadora Simples**: só calcula, a partir de 2027-01-01.
- **Gerador DANFSe**: continua fora (módulo 3).

Formato do grupo na NF-e (exemplo oficial): por item, `imposto/IBSCBS` com `CST`,
`cClassTrib` e `gIBSCBS` (`vBC`; `gIBSUF` com `pIBSUF`/`gRed`/`vIBSUF`; `gIBSMun`;
`vIBS`; `gCBS` com `pCBS`/`gRed`/`vCBS`; opcional `gTribRegular`). O grupo de totais
fica em `total/IBSCBSTot`, e o IS em `imposto/IS` e `total/ISTot`.

## Objetivos

1. Na página do cliente, uma aba nova **"Conferência de notas"**. O contador sobe os
   XMLs de saída (um por um ou em ZIP, com os mesmos limites e regras da importação
   do módulo 1): NF-e, NFC-e e NFS-e nacional.
2. O sistema lê de cada nota a data de emissão, o número, o modelo e, por item, o
   grupo IBS/CBS e o de IS. Também lê os totais da nota.
3. Para cada nota, roda estas conferências e mostra cada achado em linguagem de contador:
   - **Estrutura:** o XML passa no schema oficial do motor (`/xml/validate`). O erro
     do validador é traduzido ("o campo pCBS está com 4 casas decimais; o limite é 2")
     sempre que der, e senão é mostrado como veio, com a linha.
   - **Grupo ausente:** em nota de **2026**, item sem `IBSCBS` é **⚠ aviso** para todo
     regime. É o ano de testes, e marcar ✗ sem certeza de penalidade seria o falso ✗
     que os anti-objetivos proíbem. A partir de 2027 vira **✗** no regime regular. No
     Simples continua ⚠ até que a obrigatoriedade para o Simples esteja confirmada.
     A regra fica num único lugar do código, com a data e a fonte anotadas, para ser
     ajustada quando a Receita ou o Comitê Gestor do IBS publicarem o calendário definitivo.
   - **Classificação válida:** o CST/cClassTrib existe, está vigente na data de emissão
     e vale para aquele tipo de documento (endpoints de nomenclatura/DFe do motor ou
     SQLite oficial).
   - **Classificação × cadastro:** se o item já está no cadastro do cliente (módulo 1),
     compara o cClassTrib da nota com o do cadastro. A divergência só é **erro** se o
     item estiver *confirmado* ou *manual* no cadastro; se estiver só *sugerido* ou
     *provável*, é **aviso** ("a nota usa X; o sistema sugeriu Y, ainda não confirmado").
   - **Valores (NF-e/NFC-e do regime regular):** recalcula IBS UF, IBS município e CBS (e IS quando houver) com o
     **motor oficial**, usando a classificação **da nota**, o `vBC` da nota e a data de
     emissão. Compara alíquota, redução e valor destacados, com tolerância de R$ 0,01
     por item. Em 2026 as alíquotas de teste são as do motor e a comparação é real. De
     2027 em diante o motor exige alíquotas nominais: o sistema confere a **conta**
     com as alíquotas que a própria nota declara e mostra como **aviso** quando elas
     diferem das estimadas do escritório. Esse aviso só vale para operação interna
     (destino na mesma UF e no mesmo município do cliente), porque as estimadas são
     uma linha só por escritório; em venda para outra UF ou outro município, confere só a conta.
   - **NFS-e nacional:** confere estrutura, classificação válida e classificação ×
     cadastro. **Não confere valores nem totais**, porque quem os calcula é o Ambiente
     Nacional (a DPS traz `infDPS/IBSCBS` só com a classificação, e os valores e o
     `totCIBS` vêm em `infNFSe/IBSCBS`). Conferir isso seria conferir a conta do governo.
   - **Simples Nacional:** o regime vem do **cadastro do cliente**. Se o `CRT` da nota
     disser outra coisa, isso vira ⚠ "regime na nota difere do cadastro". Em 2026
     confere só a estrutura e a classificação, e os valores ficam "não conferidos (Simples
     em 2026)". De 2027 em diante recalcula no **motor do Simples** com `cAtivSN`, `pIBSSN`
     e `pCBSSN` do cadastro. Se faltarem no cadastro, a nota fica "não consegui conferir:
     preencha a atividade e as alíquotas do Simples no cadastro do cliente".
   - **Totais (NF-e/NFC-e):** a soma dos itens bate com `IBSCBSTot`/`ISTot`.
4. Cada nota fica com uma situação: **✓ sem divergências**, **⚠ avisos** ou
   **✗ divergências**. Há ainda um quarto estado, **"não consegui conferir"**, com o
   motivo (motor fora, campo que o sistema não sabe ler etc.). Esse estado nunca vira ✗.
5. A tela de resultado é organizada por **competência** (mês de emissão): um resumo
   (N notas, quantas ✓/⚠/✗) e a lista de notas filtrável pela situação. Ao abrir uma
   nota, aparecem os achados por item, cada um com o que está na nota, o que deveria
   estar e por quê (regra e fundamentação legal, no mesmo padrão do módulo 1).
6. **Exporta XLSX** das divergências do período: nota, data, item, tipo de achado,
   valor na nota, valor esperado, diferença em R$ e fundamentação.
7. **Um upload só de XML.** A importação de XML do módulo 1 e a conferência viram o
   mesmo fluxo: todo XML de saída enviado alimenta o cadastro e gera a conferência da
   nota. A importação por planilha continua separada, porque planilha não é nota. Isso
   muda a tela de importação que já está no ar: o botão de XML passa a levar a esse
   fluxo único.
   - **Item novo no cadastro** entra como hoje: com a **sugestão do sistema** (origem
     *sugerido*), e não com o cClassTrib da nota. Assim um erro do ERP não contamina o
     cadastro. Quando a nota diverge da sugestão, aparece ⚠ e o cClassTrib da nota é
     mostrado como alternativa na revisão do item.
   - Ordem: grava ou atualiza o cadastro primeiro e depois confere, para que a
     comparação classificação × cadastro sempre tenha o item.
   - **Reenvio da mesma nota** (mesma chave em `notas_importadas`): **não** soma
     ocorrências no cadastro de novo, só refaz a conferência dela e substitui a anterior.
   - Notas importadas pelo módulo 1 antes deste módulo não têm conferência. Reenviar o
     XML cria a conferência sem duplicar o cadastro.
8. Notas de outro emitente (entrada) e eventos (cancelamento etc.) são ignorados com
   aviso, como no módulo 1.
9. **Demo:** o escritório demo ganha notas de saída prontas para os três clientes,
   algumas certas e algumas com erros típicos (grupo ausente, cClassTrib de alíquota
   cheia num item com redução, valor de CBS calculado sobre a base errada, total que
   não bate). Assim a conferência mostra as quatro situações sem preparo.

## Não-objetivos

- Notas de entrada e apuração de crédito.
- CT-e, BP-e, NF3-e (o motor valida, mas o público do escritório típico é NF-e/NFS-e;
  fica para depois).
- Baixar notas da SEFAZ ou do Ambiente Nacional com certificado digital.
- Corrigir ou reemitir a nota. Gerar o grupo IBS/CBS corrigido com `/xml/generate`
  fica anotado como próxima evolução.
- Conferir PIS/Cofins/ICMS/ISS.
- Vários usuários por escritório (continua um usuário = um escritório).

## Anti-objetivos

- **Falso ✗.** Apontar erro onde o problema é uma limitação do sistema destrói a
  confiança do contador. Na dúvida: "não consegui conferir" e o motivo.
- Achado sem explicação. Todo achado diz o que está na nota, o que era esperado e por quê.
- Jargão de programador ou mensagem crua de validador quando dá para traduzir.
- Conferência lenta a ponto de o contador desistir (veja Restrições).
- Dados de um escritório visíveis para outro.

## Restrições

- Mesma stack, mesma separação: o parse e as regras ficam na api Go; os motores
  são chamados por HTTP na rede interna; o SQLite oficial é lido só para leitura.
- RLS por `user_id` em toda tabela nova, no mesmo padrão do schema atual
  (`foreign key (cliente_id, user_id)`, política `_dono`, revoke de `anon`).
- Upload: mesmos limites do módulo 1 (50 MB, 2000 arquivos no ZIP).
- Desempenho: um ZIP de 500 notas de um cliente médio precisa terminar em cerca de
  1 minuto, ou a tela mostra progresso. O motor regular já calcula em lote; vale
  agrupar as chamadas por nota ou por combinação distinta (cClassTrib, data, UF,
  município).
- A VPS é compartilhada: nada de serviço novo pesado. Se for preciso processar em
  segundo plano, isso fica dentro da própria api.
- Datas: a regra de cada nota usa a **data de emissão** da nota, não a data de hoje.

## Abordagens consideradas

| | Próximo passo | Trade-off | Decisão |
|---|---|---|---|
| A | **Conferência de notas (módulo 2)** | atende todo escritório, que é o problema do mês em 2026; usa os motores ainda parados; é o maior volume de trabalho | **escolhida** (o Lucas: "pode prosseguir" com a recomendação) |
| B | Uso diário (multiusuário, importação mais robusta, LGPD, acabamento) | mais confiável, mas nada novo para mostrar | só o que a A exigir |
| C | Relatório da reforma para o cliente (módulo 3, PDF + DANFSe) | vende bem, mas é entrega ao cliente e não o trabalho diário | depois |

Dentro da A, a conferência poderia ser uma tela separada do cadastro ou o mesmo
upload alimentando os dois. Escolhi **o mesmo upload alimentando os dois** (objetivo 7),
porque o contador já recebe os XMLs do mês uma vez e não deve subir duas vezes.

## Decisões do Lucas

- O sistema é para contadores de verdade usarem com os clientes deles, abrangendo o
  máximo de escritórios com base nos repositórios da Receita.
- O próximo passo é o módulo 2 (conferência de notas).

## Decisões minhas (podem ser corrigidas)

- Só notas de saída; entrada fica de fora.
- NF-e, NFC-e e NFS-e nacional; os outros DFe ficam para depois.
- NFS-e: só estrutura e classificação; valores e totais são do Ambiente Nacional.
- Simples: regime pelo cadastro (CRT divergente = ⚠); 2026 sem conferir valores; 2027+ no motor do Simples.
- Grupo ausente em 2026 = ⚠ para todos; ✗ no regular a partir de 2027.
- Um upload único de XML (substitui o botão de XML do módulo 1); item novo entra com a sugestão do sistema.
- Reenvio não soma ocorrências, só refaz a conferência.
- Aviso de alíquota diferente da estimada só em operação interna.
- Divergência com o cadastro só vira erro quando o item está confirmado ou é manual.
- Tolerância de R$ 0,01 por item.
- 2027+: confere a conta com as alíquotas da nota e avisa quando diferem das estimadas.
- Organização da tela por competência (mês de emissão).
- Reenvio da mesma nota substitui a conferência anterior.
- Notas demo com erros típicos plantados.
- Correção automática (gerar o grupo certo) fica para depois.

## Deixado para quem implementar

- O modelo das tabelas novas (notas conferidas, achados), respeitando o padrão de RLS.
- Como chamar `/xml/validate` com um `nfeProc` real (raiz, namespace, `subtipo`).
- Tradução das mensagens mais comuns do validador XSD.
- Agrupamento das chamadas ao motor e se o processamento é síncrono ou em segundo
  plano dentro da api, desde que cumpra a meta de desempenho.
- Componentes de UI, seguindo os do módulo 1.
- Fixtures de teste: partir dos XMLs válidos do repo da Receita
  (`src/test/resources/xml/{nfe,nfce,nfse}/valido/`) e derivar as versões com erro.
