"use client";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";
import Shell from "@/components/Shell";
import TabelaItens, { nbsDaOpcao, SITUACOES, type Item, type Opcao } from "@/components/TabelaItens";
import { Botao, Cartao, Erro, Titulo } from "@/components/ui";
import { api, baixar } from "@/lib/api";

type Cliente = { id: string; razaoSocial: string; cnpj: string; regime: string };
type Situacao = keyof typeof SITUACOES;
const ANOS = [2026, 2027, 2028, 2029, 2030, 2031, 2032, 2033];

function Tela() {
  const { id } = useParams<{ id: string }>();
  const [cliente, setCliente] = useState<Cliente | null>(null);
  const [itens, setItens] = useState<Item[] | null>(null);
  const [contagem, setContagem] = useState<Record<Situacao, number>>({ ok: 0, provavel: 0, revisar: 0, invalido: 0 });
  const [filtro, setFiltro] = useState<Situacao | "">("");
  const [sel, setSel] = useState<Set<string>>(new Set());
  const [ano, setAno] = useState(2027);
  const [ocupado, setOcupado] = useState("");
  const [aviso, setAviso] = useState("");
  const [erro, setErro] = useState("");

  const carregar = useCallback(
    () => api<{ itens: Item[]; contagem: Record<Situacao, number> }>(`/clientes/${id}/itens${filtro && `?situacao=${filtro}`}`)
      .then((r) => { setItens(r.itens); setContagem(r.contagem); }),
    [id, filtro],
  );
  useEffect(() => { api<Cliente>(`/clientes/${id}`).then(setCliente, (e) => setErro(e.message)); }, [id]);
  useEffect(() => { carregar().catch((e) => setErro(e.message)); }, [carregar]);

  // executa uma ação, mostra o aviso e recarrega a lista; o rótulo trava os botões enquanto roda
  async function acao(rotulo: string, fn: () => Promise<string | void>) {
    setOcupado(rotulo); setErro(""); setAviso("");
    try {
      const msg = await fn();
      if (msg) setAviso(msg);
      await carregar();
    } catch (e) {
      setErro((e as Error).message);
    } finally {
      setOcupado("");
    }
  }

  const confirmar = () => acao("confirmar", async () => {
    const r = await api<{ confirmados: number }>(`/clientes/${id}/itens/confirmar`, { method: "POST", body: JSON.stringify({ ids: [...sel] }) });
    setSel(new Set());
    return `${r.confirmados} ${r.confirmados === 1 ? "item confirmado" : "itens confirmados"}.`;
  });

  const escolherIS = (i: Item, o: Opcao | null) => acao("escolher", async () => {
    await api(`/itens/${i.id}`, { method: "PATCH", body: JSON.stringify({ cst: i.cst, cClassTrib: i.cClassTrib, isCst: o?.cst ?? "", isCClassTrib: o?.cClassTrib ?? "" }) });
  });

  const escolher = (i: Item, o: Opcao) => acao("escolher", async () => {
    const nbs = nbsDaOpcao(o);
    await api(`/itens/${i.id}`, { method: "PATCH", body: JSON.stringify({ cst: o.cst, cClassTrib: o.cClassTrib, isCst: i.isCst, isCClassTrib: i.isCClassTrib, ...(nbs && { nbs }) }) });
  });

  const calcular = () => acao("calcular", async () => {
    const r = await api<{ resultados: { estado: string }[]; pendentes: Item[] }>(`/clientes/${id}/calcular`, { method: "POST", body: JSON.stringify({ ano }) });
    const ok = r.resultados.filter((x) => x.estado === "CALCULADO" || x.estado === "CALCULO_SIMULADO").length;
    const falhas = r.resultados.length - ok, n = r.pendentes.length;
    return `${ok} ${ok === 1 ? "item calculado" : "itens calculados"} para ${ano}.` +
      (falhas ? ` ${falhas} com erro (veja a coluna Valores).` : "") +
      (n ? ` ${n} ${n === 1 ? "item ficou" : "itens ficaram"} de fora: confirme ou revise a classificação antes de calcular.` : "");
  });

  const exportar = () => acao("exportar", () => baixar(`/clientes/${id}/exportar.xlsx?ano=${ano}`, `cadastro-${cliente?.cnpj ?? id}-${ano}.xlsx`));

  const total = contagem.ok + contagem.provavel + contagem.revisar + contagem.invalido;
  const filtros: [Situacao | "", string, number][] = [
    ["", "Todos", total],
    ...(Object.keys(SITUACOES) as Situacao[]).map((s): [Situacao, string, number] => [s, `${SITUACOES[s].icone} ${SITUACOES[s].rotulo}`, contagem[s]]),
  ];

  return (
    <>
      <Titulo acao={
        <div className="flex items-center gap-4">
          <Link href={`/clientes/${id}/editar`} className="text-sm text-teal-800 hover:underline">Editar cliente</Link>
          <Link href={`/clientes/${id}/importar`} className="rounded-md bg-teal-800 px-4 py-2 text-sm font-medium text-white hover:bg-teal-900">Importar arquivos</Link>
        </div>
      }>
        {cliente?.razaoSocial ?? "Cliente"}
        {cliente && <span className="ml-3 text-sm font-normal text-slate-500">{cliente.regime === "simples" ? "Simples Nacional" : "Regime regular"}</span>}
      </Titulo>

      <div className="mb-4 flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-wrap gap-2" role="group" aria-label="Filtrar por situação">
          {filtros.map(([s, nome, n]) => (
            <button key={s} onClick={() => { setFiltro(s); setSel(new Set()); }} aria-pressed={filtro === s}
              className={`rounded-full px-3 py-1 text-sm ring-1 ${filtro === s ? "bg-teal-800 text-white ring-teal-800" : "bg-white text-slate-700 ring-slate-300 hover:bg-slate-50"}`}>
              {nome} <span className="tabular-nums opacity-75">({n})</span>
            </button>
          ))}
        </div>
        <div className="flex items-end gap-2">
          <label className="text-sm">
            <span className="mb-1 block font-medium text-slate-700">Ano</span>
            <select value={ano} onChange={(e) => setAno(Number(e.target.value))} className="rounded-md border border-slate-300 bg-white px-3 py-2 text-sm">
              {ANOS.map((a) => <option key={a} value={a}>{a}</option>)}
            </select>
          </label>
          <Botao onClick={calcular} disabled={!!ocupado}>{ocupado === "calcular" ? "Calculando…" : "Calcular"}</Botao>
          <Botao secundario onClick={exportar} disabled={!!ocupado}>Exportar XLSX</Botao>
        </div>
      </div>
      <p className="mb-4 text-xs text-slate-500">
        Em 2026 o cálculo usa as alíquotas de teste da Receita. Clientes do Simples Nacional só são calculados a partir de 2027.
        De 2027 em diante os valores são uma simulação com as alíquotas estimadas.
      </p>

      <div className="mb-4 space-y-2">
        <Erro>{erro}</Erro>
        {aviso && <p role="status" className="rounded-md border border-teal-200 bg-teal-50 px-3 py-2 text-sm text-teal-900">{aviso}</p>}
      </div>

      {(filtro === "" || filtro === "provavel") && contagem.provavel > 0 && (
        <div className="mb-3 flex items-center gap-3 text-sm text-slate-600">
          <Botao onClick={confirmar} disabled={!sel.size || !!ocupado}>Confirmar selecionados ({sel.size})</Botao>
          <span>Marque os itens prováveis cuja classificação está correta.</span>
        </div>
      )}

      {itens && itens.length === 0 && (
        <Cartao><p className="text-sm text-slate-600">{total === 0 ? "Nenhum item ainda. Importe as notas ou a planilha de produtos do cliente." : "Nenhum item nesta situação."}</p></Cartao>
      )}
      {itens && itens.length > 0 && (
        <TabelaItens itens={itens} ano={ano} regime={cliente?.regime ?? ""} selecionados={sel} ocupado={!!ocupado} onEscolher={escolher} onEscolherIS={escolherIS}
          onSelecionar={(ids, marcar) => setSel((s) => { const n = new Set(s); ids.forEach((i) => (marcar ? n.add(i) : n.delete(i))); return n; })} />
      )}
    </>
  );
}

export default function ClienteDetalhe() {
  return (
    <Suspense>
      <Shell><Tela /></Shell>
    </Suspense>
  );
}
