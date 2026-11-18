create table clientes (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null default auth.uid() references auth.users on delete cascade,
  razao_social text not null,
  cnpj char(14) not null,
  regime text not null check (regime in ('regular','simples')),
  uf char(2) not null,
  municipio_ibge int not null,
  c_ativ_sn char(2),
  p_ibs_sn numeric,
  p_cbs_sn numeric,
  criado_em timestamptz not null default now(),
  unique (user_id, cnpj),
  unique (id, user_id)
);

create table notas_importadas (
  user_id uuid not null default auth.uid() references auth.users on delete cascade,
  cliente_id uuid not null,
  chave text not null,
  primary key (cliente_id, chave),
  foreign key (cliente_id, user_id) references clientes (id, user_id) on delete cascade
);

create table importacoes (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null default auth.uid() references auth.users on delete cascade,
  cliente_id uuid not null,
  tipo text not null,
  arquivos int not null default 0,
  itens_novos int not null default 0,
  itens_atualizados int not null default 0,
  ignorados jsonb not null default '[]',
  criado_em timestamptz not null default now(),
  foreign key (cliente_id, user_id) references clientes (id, user_id) on delete cascade
);

create table itens (
  id uuid primary key default gen_random_uuid(),
  user_id uuid not null default auth.uid() references auth.users on delete cascade,
  cliente_id uuid not null,
  codigo text not null,
  descricao text not null default '',
  ncm text not null default '',
  nbs text not null default '',
  c_trib_nac text not null default '',
  ocorrencias int not null default 1,
  ultimo_valor numeric not null default 0,
  situacao text not null default 'revisar' check (situacao in ('ok','provavel','revisar','invalido')),
  origem text not null default 'sugerido' check (origem in ('sugerido','confirmado','manual')),
  cst text not null default '',
  cclasstrib text not null default '',
  is_cst text not null default '',
  is_cclasstrib text not null default '',
  condicao text not null default '',
  opcoes jsonb not null default '[]',
  calculos jsonb not null default '{}', -- chave = ano
  atualizado_em timestamptz not null default now(),
  unique (cliente_id, codigo, ncm, nbs),
  foreign key (cliente_id, user_id) references clientes (id, user_id) on delete cascade
);

create table aliquotas_estimadas (
  user_id uuid primary key default auth.uid() references auth.users on delete cascade,
  cbs numeric not null default 8.80,
  ibs_uf numeric not null default 15.00,
  ibs_mun numeric not null default 2.70,
  is_ad_valorem numeric not null default 0
);

do $$
declare t text;
begin
  foreach t in array array['clientes','notas_importadas','importacoes','itens','aliquotas_estimadas'] loop
    execute format('alter table %I enable row level security', t);
    execute format('create policy %I on %I for all using (user_id = auth.uid()) with check (user_id = auth.uid())', t || '_dono', t);
    execute format('revoke all on %I from anon', t);
    execute format('revoke truncate, references, trigger on %I from authenticated', t);
  end loop;
end $$;
