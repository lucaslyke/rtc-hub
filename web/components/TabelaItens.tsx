"use client";

export type Opcao = { cst: string; cClassTrib: string; descricao: string; fundamentacao: string; condicao: string; percReducao: number };
export type Calculo = { estado: string; vCbs: string; vIbs: string; vIs: string; mensagem: string; memoria?: string };
export type Item = {
  id: string; codigo: string; descricao: string; ncm: string; nbs: string; cTribNac: string; ultimoValor: number;
  situacao: "ok" | "provavel" | "revisar" | "invalido"; cst: string; cClassTrib: string; isCst: string; isCClassTrib: string;
  condicao: string; opcoes: { ibsCbs: Opcao[]; is: Opcao[] }; calculos: Record<string, Calculo> | null;
};

export const SITUACOES = {
  ok: { rotulo: "Pronto", icone: "✓", cor: "bg-green-50 text-green-800 ring-green-200" },
  provavel: { rotulo: "Provável (confirmar)", icone: "?", cor: "bg-sky-50 text-sky-800 ring-sky-200" },
  revisar: { rotulo: "Revisar", icone: "⚠", cor: "bg-amber-50 text-amber-900 ring-amber-200" },
  invalido: { rotulo: "Inválido", icone: "✗", cor: "bg-red-50 text-red-800 ring-red-200" },
} as const;

export const reais = (v: string | number) => Number(v).toLocaleString("pt-BR", { style: "currency", currency: "BRL" });

// "Só em: NF-e, NFC-e, ..." não diz nada para quem emite NF-e e NFC-e; só mostra quando restringe.
export function condicao(c: string) {
  if (!c.startsWith("Só em: ")) return c;
  const docs = c.slice(7).split(", ");
  return docs.includes("NF-e") && docs.includes("NFC-e") ? "" : c;
}

// NFS-e sem NBS: a opção carrega a NBS no início da descrição ("NBS 123456789 — ...").
export const nbsDaOpcao = (o: Opcao) => o.descricao.match(/^NBS (\d{9}) — /)?.[1] ?? "";

function escolhida(i: Item) {
  return i.opcoes.ibsCbs.find((o) => o.cClassTrib === i.cClassTrib && (!nbsDaOpcao(o) || nbsDaOpcao(o) === i.nbs));
}

const corta = (t: string, n: number) => (t.length > n ? `${t.slice(0, n - 1)}…` : t);
// a base legal vai no texto da opção: o title não aparece no celular nem no leitor de tela
const rotulo = (o: Opcao) =>
  `${o.cClassTrib} — ${corta(o.descricao, 70)} · redução ${o.percReducao}%${o.fundamentacao ? ` · ${corta(o.fundamentacao, 60)}` : ""}`;
const seletor = "mt-1 w-full max-w-md rounded-md border border-slate-300 bg-white px-2 py-1.5 text-xs disabled:opacity-50";

type Acoes = { ocupado: boolean; onEscolher: (i: Item, o: Opcao) => void; onEscolherIS: (i: Item, o: Opcao | null) => void };

function PorQue({ item, ocupado, onEscolher, onEscolherIS }: { item: Item } & Acoes) {
  const o = escolhida(item);
  const extras = [o && condicao(o.condicao), condicao(item.condicao)].filter(Boolean);
  return (
    <div className="space-y-1">
      {o ? (
        <>
          <p className="text-xs tabular-nums text-slate-500">CST {o.cst} · cClassTrib {o.cClassTrib}{o.percReducao > 0 && ` · redução de ${o.percReducao}%`}</p>
          <p className="text-slate-800">{o.descricao}</p>
          {o.fundamentacao && <p className="text-xs text-slate-500">{o.fundamentacao}</p>}
        </>
      ) : item.situacao === "revisar" ? (
        <>
          <p className="text-slate-700">Há mais de uma classificação possível. Escolha a que se aplica:</p>
          <ul className="list-disc space-y-0.5 pl-4 text-xs text-slate-600">
            {item.opcoes.ibsCbs.map((op, k) => (
              <li key={k}>
                <span className="tabular-nums">{op.cClassTrib}</span> — {op.descricao} (redução {op.percReducao}%)
                {op.fundamentacao && <span className="text-slate-500"> · {op.fundamentacao}</span>}
                {condicao(op.condicao) && <span className="text-amber-800"> · {condicao(op.condicao)}</span>}
              </li>
            ))}
          </ul>
        </>
      ) : null}
      {item.isCClassTrib && <p className="text-xs text-slate-600">Imposto Seletivo: CST {item.isCst} · cClassTrib {item.isCClassTrib}</p>}
      {extras.map((t) => <p key={t} className="text-xs text-amber-800">{t}</p>)}
      {item.opcoes.ibsCbs.length > 0 && (
        <select aria-label={`Classificação de ${item.descricao}`} value="" disabled={ocupado}
          onChange={(e) => onEscolher(item, item.opcoes.ibsCbs[Number(e.target.value)])} className={seletor}>
          <option value="" disabled>{o ? "Trocar a classificação…" : "Escolher a classificação…"}</option>
          {item.opcoes.ibsCbs.map((op, k) => <option key={k} value={k}>{rotulo(op)}</option>)}
        </select>
      )}
      {item.opcoes.is.length > 0 && item.cClassTrib && ( // o PATCH exige a classificação de IBS/CBS junto
        <select aria-label={`Imposto Seletivo de ${item.descricao}`} value={item.isCClassTrib} disabled={ocupado}
          onChange={(e) => onEscolherIS(item, item.opcoes.is.find((x) => x.cClassTrib === e.target.value) ?? null)} className={seletor}>
          <option value="">Imposto Seletivo: não incide (revenda)</option>
          {item.opcoes.is.map((op) => <option key={op.cClassTrib} value={op.cClassTrib}>IS {rotulo(op)}</option>)}
        </select>
      )}
    </div>
  );
}

function Valores({ c, ano, pronto }: { c?: Calculo; ano: number; pronto: boolean }) {
  if (!c || !pronto) return <span className="text-slate-400">—</span>; // valor de item ainda não pronto não é resultado
  if (!c.vCbs && !c.vIbs) return <span className="text-xs text-slate-500">{c.mensagem || "Não calculado"}</span>;
  return (
    <div className="space-y-0.5 text-right tabular-nums">
      <p>CBS {reais(c.vCbs || 0)}</p>
      <p>IBS {reais(c.vIbs || 0)}</p>
      {Number(c.vIs) > 0 && <p>IS {reais(c.vIs)}</p>}
      {ano >= 2027 && <span className="inline-block rounded bg-violet-50 px-1.5 py-0.5 text-xs text-violet-800 ring-1 ring-violet-200">simulação</span>}
      {c.mensagem && <p className="max-w-56 whitespace-normal text-xs text-amber-800">{c.mensagem}</p>}
      {c.memoria && (
        <details className="max-w-64 whitespace-normal text-left text-xs text-slate-500">
          <summary className="cursor-pointer text-right">memória de cálculo</summary>
          {c.memoria}
        </details>
      )}
    </div>
  );
}

export default function TabelaItens({ itens, ano, regime, selecionados, onSelecionar, ...acoes }: {
  itens: Item[]; ano: number; regime: string; selecionados: Set<string>; onSelecionar: (ids: string[], marcar: boolean) => void;
} & Acoes) {
  const provaveis = itens.filter((i) => i.situacao === "provavel").map((i) => i.id);
  const todos = provaveis.length > 0 && provaveis.every((id) => selecionados.has(id));
  return (
    <>
    <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white shadow-sm">
      <table className="w-full text-left text-sm">
        <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
          <tr>
            <th className="w-10 px-3 py-3">
              <input type="checkbox" aria-label="Selecionar todos os prováveis" disabled={!provaveis.length} checked={todos}
                onChange={(e) => onSelecionar(provaveis, e.target.checked)} />
            </th>
            <th className="px-3 py-3">Item</th>
            <th className="px-3 py-3">Situação</th>
            <th className="px-3 py-3">Por quê</th>
            <th className="px-3 py-3 text-right">Valores {ano}</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-100 align-top">
          {itens.map((i) => {
            const s = SITUACOES[i.situacao];
            return (
              <tr key={i.id}>
                <td className="px-3 py-3">
                  {i.situacao === "provavel" && (
                    <input type="checkbox" aria-label={`Selecionar ${i.descricao}`} checked={selecionados.has(i.id)}
                      onChange={(e) => onSelecionar([i.id], e.target.checked)} />
                  )}
                </td>
                <td className="min-w-40 px-3 py-3">
                  <p className="font-medium text-slate-900">{i.descricao || "(sem descrição)"}</p>
                  <p className="text-xs tabular-nums text-slate-500">
                    {[i.codigo && i.codigo !== i.descricao && `Cód. ${i.codigo}`, i.ncm && `NCM ${i.ncm}`, i.nbs && `NBS ${i.nbs}`, !i.ncm && !i.nbs && i.cTribNac && `Serviço ${i.cTribNac}`, reais(i.ultimoValor)].filter(Boolean).join(" · ")}
                  </p>
                </td>
                <td className="whitespace-nowrap px-3 py-3">
                  <span className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ring-1 ${s.cor}`}>{s.icone} {s.rotulo}</span>
                </td>
                <td className="min-w-72 px-3 py-3"><PorQue item={i} {...acoes} /></td>
                <td className="whitespace-nowrap px-3 py-3"><Valores c={i.calculos?.[String(ano)]} ano={ano} pronto={i.situacao === "ok"} /></td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
    {regime === "simples" && (
      <p className="mt-2 text-xs text-slate-500">
        No Simples Nacional o valor vem da alíquota efetiva do DAS informada no cadastro; a redução exibida é a da classificação do item.
      </p>
    )}
    </>
  );
}
