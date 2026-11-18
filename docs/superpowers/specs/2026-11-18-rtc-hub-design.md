# rtc-hub — Design

Data: 2026-11-18 · Status: v2 revisado (builder check), aguardando aprovação

## Intenção e porquê

O Lucas quer se candidatar a vagas de **auxiliar de contabilidade** e mostrar,
na entrevista, que pode ajudar o escritório a se adaptar à Reforma Tributária
do Consumo (LC 214/2025: CBS, IBS e Imposto Seletivo) e a trabalhar mais rápido.

Quem avalia é um **contador ou dono de escritório**, olhando a tela por poucos
minutos. A pergunta que o projeto responde é: *"essa pessoa resolve um problema
meu?"* A mensagem é: **"faço em minutos, com as ferramentas oficiais da Receita,
o que vocês fariam em horas"**. A qualidade do código importa como bônus, não como vitrine.

O sistema é web e usa os motores open source (MIT) da Receita Federal:

| Motor | Repo | Uso no rtc-hub |
|---|---|---|
| Calculadora regime regular | `ReceitaFederal/calculadora-rtc-regime-regular` (Java 21, SQLite) | cálculo CBS/IBS/IS; **banco SQLite com as tabelas oficiais** (NCM/NBS aplicáveis, cClassTrib, fundamentação legal) |
| Calculadora Simples Nacional | `ReceitaFederal/calculadora-rtc-simples-nacional` (Java 21) | cálculo CBS/IBS para clientes do Simples |
| Gerador DANFSe | `ReceitaFederal/calculadora-rtc-gerador-danfse` (Go) | fora do MVP (fica para o módulo 3) |

## Módulos (visão geral)

1. **Classificação do cadastro do cliente** ← **MVP, este documento**
2. Conferência de notas fiscais (XML com IBS/CBS errado ou ausente): próxima fase.
3. Relatório da reforma para o cliente (PDF explicativo, exemplo de DANFSe): fase posterior.

## Objetivos do MVP (módulo 1)

1. O usuário (escritório) cadastra **clientes** (razão social, CNPJ, regime
   `regular|simples`, UF, município). Para o Simples: atividade do catálogo oficial
   (`cAtivSN`, 15 atividades, V0049) e as alíquotas efetivas `pIBSSN`/`pCBSSN`
   **digitadas pelo contador** (ele já as conhece pelo cálculo do DAS; o sistema
   não calcula RBT12, faixas nem Fator R).
2. Para cada cliente, importa itens por **qualquer uma** destas fontes, que podem ser combinadas:
   - **XMLs de NF-e/NFC-e** (produtos: código, descrição, NCM, CFOP, valor), um por um ou em ZIP;
   - **XMLs de NFS-e nacional** (serviços: descrição, NBS, código de tributação, valor).
     Se a nota não tiver `cNBS`, só `cTribNac` (LC 116), o item vira ⚠ com os NBS
     candidatos via `CLASSIF_NBS_INDOP_LC`;
   - **só notas de saída**: o emitente precisa ser o CNPJ do cliente. Notas de outro
     emitente (entrada) são ignoradas com aviso na importação;
   - **planilha CSV/XLSX**, com tela para mapear as colunas (descrição, NCM ou NBS, código, valor opcional).
3. Os itens são **deduplicados** por cliente (mesmo código do produto + NCM/NBS)
   e guardam de quais notas vieram (quantidade de ocorrências, último valor).
4. Para cada item, o sistema **sugere CST + cClassTrib** consultando as tabelas oficiais:
   - NCM/NBS sem tratamento especial → tributação integral (CST 000 / cClassTrib 000001) → **✓ ok**
   - um único tratamento especial aplicável (por prefixo de NCM, descontadas as
     exceções, vigente na data) → **✓ provável**, com a condição escrita ao lado
     quando houver (comprador específico `TOMADOR_ESPECIFICO`, tipo de documento
     `TIPO_DFE_CLASSIFICACAO`, requisito legal da fundamentação). A tributação
     integral continua disponível como alternativa. O contador **confirma em lote**
     (selecionar todos → confirmar). "Provável" não conta como pronto até ser confirmado.
   - mais de um tratamento especial possível → **⚠ revisar**, listando as opções com descrição e fundamentação legal
   - NCM/NBS inexistente ou não vigente → **✗**
   - **Imposto Seletivo**: itens cujo NCM tem IS recebem uma **segunda classificação**
     (CST/cClassTrib de IS), separada da de IBS/CBS. A ambiguidade é avaliada por tributo.
5. O usuário **revisa** os itens ⚠ (escolhe a opção) e pode sobrescrever qualquer sugestão.
   A escolha manual fica registrada e não é apagada por uma nova importação.
6. Com tudo classificado, o sistema **calcula** CBS/IBS/IS por item no motor do
   regime do cliente (valor base = último valor ou 100,00 de referência) e mostra
   a redução aplicada e a memória de cálculo. **Seletor de ano** (decisão do usuário):
   - **2026**: alíquotas de teste reais do motor (CBS 0,9% / IBS 0,1%), sem
     `aliquotasNominais` (o motor proíbe antes de 2027). O Simples não calcula em
     2026 (a parametrização começa em 2027-01-01): a tela explica isso em vez de mostrar erro.
   - **2027 em diante**: o motor exige `aliquotasNominais`. O sistema envia
     **estimativas editáveis** (tela "Alíquotas" por escritório: CBS, IBS estadual
     por UF, IBS municipal), pré-preenchidas com uma estimativa documentada no
     código. Todo resultado é marcado **"simulação"** (o motor devolve `CALCULO_SIMULADO`).
   - Padrão do seletor: 2027 (é onde aparecem as reduções da reforma).
   - O IS também só tem dados a partir de 2027.
7. **Exporta XLSX** do cadastro classificado: item, NCM/NBS, CST, cClassTrib,
   descrição da classificação, % de redução, fundamentação legal, origem
   (sugerido/confirmado/manual), classificação de IS quando houver, ano e alíquotas usadas.
8. **Demo pronta**: botão "entrar como demo" cria um **usuário anônimo do Supabase**
   com uma **cópia própria** do escritório pré-carregado (padaria no Simples,
   farmácia e escola no regime regular). Assim um visitante não bagunça a demo de
   outro. As cópias anônimas são apagadas depois de X dias.

## Não-objetivos

- Não emite nota, não transmite nada à Receita, não assina XML.
- Não calcula os tributos atuais (PIS/Cofins/ICMS/ISS); os motores só fazem os novos.
- Não decide sozinho os casos ambíguos: a decisão final é do profissional.
- Sem cobrança, planos, convites entre usuários (um usuário = um escritório no MVP).
- Não modifica os motores (entram fixados num commit).

## Anti-objetivos

- Sugestão errada apresentada com cara de certa. Toda sugestão mostra **por que**
  foi feita (regra e fundamentação) e todo caso ambíguo vira ⚠.
- Tela com jargão de programador. A linguagem é a do contador.
- Demo que quebra ou exige preparo na hora da entrevista.
- Dados de um escritório visíveis para outro.

## Restrições e fatos verificados

- Não existe endpoint "NCM → classificações possíveis". A ligação está nas tabelas
  `NCM_APLICAVEL` / `NBS_APLICAVEL` (+ `EXCECAO_*`) do **SQLite gerado pelo Flyway** do
  motor regular. Os códigos lá são **prefixos** (exemplo: `100620`), com vigência. O rtc-hub
  lê esse arquivo direto, em modo somente leitura (mesmo arquivo que o motor usa).
- Para serviços existe o caminho inverso no motor (`/dados-abertos/nbs-aplicaveis?cClassTrib=`).
  Usar o SQLite direto deixa NCM e NBS no mesmo caminho.
- Os repos Java não têm Dockerfile: o rtc-hub fornece Dockerfiles multi-stage que
  rodam o Flyway e geram o `.db`. A pasta `flyway/` que o Simples pede está no repo regular.
- A calculadora do Simples só calcula (não gera nem valida XML).
- As duas JVMs pesam (ordem de 300–500 MB cada). A VPS roda n8n, Evolution e
  multivozes no Easypanel: o deploy não pode afetá-los (projeto separado, limite de memória).
- Conferir no build se os motores pedem internet ao subir (o Simples tem `VERSAO_URL`/`DB_DOWNLOAD_URL`).
- Banco do rtc-hub: **Supabase cloud** (Auth, Postgres, Storage para os uploads).

## Abordagens consideradas

**Histórico:** o primeiro design mirava recrutadores de backend (Sefin simulada com
assinatura XMLDSig, fila pgmq, tracing, emissão de NFS-e). O público mudou para
escritório contábil e esse escopo saiu do MVP. A stack foi mantida.

| | Abordagem | Trade-off | Decisão |
|---|---|---|---|
| A | **Go (api) + Next.js + Supabase** | uma api pequena e rápida, combina com os motores via HTTP e SQLite | **escolhida pelo usuário** |
| B | TypeScript ponta a ponta (NestJS) | uma linguagem só | descartada |
| — | Java | usuário não gosta | descartada |

Fonte dos itens: XML **e** planilha (decisão do usuário: "nunca sabemos").

## Arquitetura

```
 navegador ──▶ web (Next.js) ──JWT──▶ api (Go) ──HTTP──▶ calc-regular (Java)
                    │                   │   └──HTTP──▶ calc-simples (Java)
                    │                   ├── lê SQLite oficial (volume compartilhado, read-only)
                    ▼                   ▼
              Supabase Auth      Supabase Postgres (RLS) + Storage
```

- **web**: login (Supabase Auth), clientes, importação, tabela de revisão, exportação.
- **api** (Go): valida o JWT, parse de XML (NF-e/NFC-e/NFS-e) e planilha, deduplicação,
  motor de sugestão (consulta ao SQLite), chamada aos motores de cálculo, geração do XLSX.
- **motores**: só na rede interna, sem porta publicada. O `.db` gerado no build do
  calc-regular é compartilhado com a api por volume read-only.
- Importação síncrona no MVP (lotes de até algumas centenas de XMLs). Se ficar lento,
  evolui para processamento em background.

### Dados (Postgres)

- `clientes` (escritório = user_id dono, cnpj, razão social, regime, uf, município, atividade_simples)
- `importacoes` (cliente_id, tipo `nfe|nfse|planilha`, arquivo no Storage, totais, erros por arquivo)
- `clientes` ganha: c_ativ_sn, p_ibs_sn, p_cbs_sn (Simples)
- `aliquotas_estimadas` (user_id, uf, municipio, cbs, ibs_uf, ibs_mun)
- `itens` (cliente_id, código, descrição, ncm|nbs, ocorrências, último_valor,
  cst, cclasstrib, is_cst, is_cclasstrib, origem `sugerido|confirmado|manual`,
  situação `ok|provavel|revisar|invalido`, condição text, opções jsonb, cálculo jsonb por ano)

RLS por `user_id` em todas as tabelas; a api aplica as claims do JWT na transação.

### Qualidade

- Testes do motor de sugestão com NCMs reais (cesta básica, Imposto Seletivo,
  ambíguo, provável com condição, extinto) contra o SQLite oficial.
- Formato real do request do Simples: `OperacaoSimplesInput.java` / `ItemSimplesInput.java`
  (o `docs/exemplos.md` do Simples é uma cópia do regular e não serve).
- Testes de parse com XMLs de exemplo de NF-e e NFS-e (incluir fixtures no repo).
- `docker compose up` sobe tudo localmente; README com prints e um GIF do fluxo
  em 30 segundos (é o que o entrevistador vai ver).
- Repositório **público** no GitHub (exceção consciente à regra de repos privados).
- Deploy no Easypanel, projeto separado, imagens do GHCR. Domínio definido no deploy.

## Decisões do usuário

- Objetivo: vaga de auxiliar de contabilidade; mostrar que ajuda o escritório na reforma.
- Começar pelo módulo 1 (classificação do cadastro).
- Fontes: XML **e** planilha.
- Sistema web; Supabase; Go + Next.js.
- Demo aberta com botão "entrar como demo"; repo público.
- Cálculo com **seletor de ano** (2026 = alíquotas de teste; 2027+ = estimativas editáveis, marcadas como simulação).
- Item com um único benefício possível = **✓ provável**, com a condição visível e confirmação em lote.

## Decisões minhas (podem ser corrigidas)

- Incluir XML de NFS-e (serviços/NBS) além de NF-e/NFC-e.
- Ler o SQLite oficial direto para as sugestões (não há endpoint para isso).
- Escolha manual não é sobrescrita por nova importação.
- Valor base do cálculo = último valor visto ou 100,00 de referência.
- Exportação em XLSX.
- Clientes demo: padaria (Simples), farmácia e escola (regime regular).
- Importação síncrona no MVP.
- Imposto Seletivo classificado à parte (segunda classificação por item).
- Simples: o contador digita `cAtivSN`, `pIBSSN` e `pCBSSN` (sem cálculo de RBT12/Fator R).
- Só notas de saída do cliente entram; as de entrada são ignoradas com aviso.
- NFS-e sem cNBS → ⚠ com NBS candidatos.
- Demo por visitante (usuário anônimo + cópia da semente).
- Seletor de ano começa em 2027.

## Deixado para quem implementar

- Router HTTP, bibliotecas de XML/XLSX em Go, geração de queries.
- UI e componentes.
- Valores iniciais das alíquotas estimadas (documentar a fonte no código).
- Limites de upload e rate limit da demo.
