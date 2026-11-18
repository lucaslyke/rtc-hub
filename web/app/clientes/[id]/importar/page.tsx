"use client";
import Link from "next/link";
import { useParams } from "next/navigation";
import { Suspense, useState, type DragEvent } from "react";
import Shell from "@/components/Shell";
import MapearColunas, { faltando, sugerirMapeamento, type Mapeamento, type Previa } from "@/components/MapearColunas";
import { Botao, Cartao, Erro, Titulo } from "@/components/ui";
import { api } from "@/lib/api";

type Resumo = { itensNovos: number; itensAtualizados: number; notasRepetidas: number; ignorados: { arquivo: string; motivo: string }[] };

const aceito = (f: File) => /\.(xml|zip|csv|xlsx)$/i.test(f.name);
const planilha = (f: File) => /\.(csv|xlsx)$/i.test(f.name);
const plural = (n: number, um: string, varios: string) => `${n} ${n === 1 ? um : varios}`;

function Tela() {
  const { id } = useParams<{ id: string }>();
  const [arquivos, setArquivos] = useState<File[]>([]);
  const [previa, setPrevia] = useState<{ nome: string; previa: Previa } | null>(null);
  const [mapa, setMapa] = useState<Mapeamento | null>(null);
  const [resumo, setResumo] = useState<Resumo | null>(null);
  const [arrastando, setArrastando] = useState(false);
  const [ocupado, setOcupado] = useState(false);
  const [erro, setErro] = useState("");

  async function escolher(novos: File[]) {
    const lista = [...arquivos, ...novos.filter(aceito)];
    setArquivos(lista);
    setResumo(null);
    setErro(novos.some((f) => !aceito(f)) ? "Só são aceitos arquivos XML, ZIP, CSV ou XLSX." : "");
    // o mesmo mapeamento vale para todas as planilhas do envio; a prévia vem da primeira
    const p = lista.find(planilha);
    if (!p || previa?.nome === p.name) return;
    const corpo = new FormData();
    corpo.append("arquivo", p);
    try {
      const r = await api<Previa>(`/clientes/${id}/planilha/previa`, { method: "POST", body: corpo });
      setPrevia({ nome: p.name, previa: r });
      setMapa(sugerirMapeamento(r.cabecalho));
    } catch (e) {
      setErro((e as Error).message);
    }
  }

  function remover(k: number) {
    const lista = arquivos.filter((_, i) => i !== k);
    setArquivos(lista);
    if (!lista.some(planilha)) { setPrevia(null); setMapa(null); }
  }

  const temPlanilha = arquivos.some(planilha);
  const pendencia = temPlanilha ? (!mapa ? "Aguarde a prévia da planilha." : faltando(mapa)) : "";

  async function importar() {
    setOcupado(true); setErro("");
    const corpo = new FormData();
    arquivos.forEach((f) => corpo.append("arquivos[]", f));
    if (temPlanilha && mapa) corpo.append("mapeamento", JSON.stringify(mapa));
    try {
      setResumo(await api<Resumo>(`/clientes/${id}/importar`, { method: "POST", body: corpo }));
      setArquivos([]); setPrevia(null); setMapa(null);
    } catch (e) {
      setErro((e as Error).message);
    } finally {
      setOcupado(false);
    }
  }

  const soltar = (e: DragEvent) => { e.preventDefault(); setArrastando(false); escolher([...e.dataTransfer.files]); };

  return (
    <>
      <Titulo acao={<Link href={`/clientes/${id}`} className="text-sm text-teal-800 hover:underline">← Voltar aos itens</Link>}>Importar arquivos</Titulo>
      <div className="space-y-6">
        <label
          onDragOver={(e) => { e.preventDefault(); setArrastando(true); }} onDragLeave={() => setArrastando(false)} onDrop={soltar}
          className={`block cursor-pointer rounded-lg border-2 border-dashed p-10 text-center ${arrastando ? "border-teal-700 bg-teal-50" : "border-slate-300 bg-white hover:border-slate-400"}`}
        >
          <input type="file" multiple accept=".xml,.zip,.csv,.xlsx" className="sr-only"
            onChange={(e) => { escolher([...(e.target.files ?? [])]); e.target.value = ""; }} />
          <p className="font-medium text-slate-800">Arraste os arquivos aqui ou clique para escolher</p>
          <p className="mt-1 text-sm text-slate-500">XML de NF-e, NFC-e e NFS-e (soltos ou em ZIP), ou a planilha de produtos em CSV ou XLSX.</p>
        </label>

        {arquivos.length > 0 && (
          <Cartao>
            <ul className="divide-y divide-slate-100 text-sm">
              {arquivos.map((f, k) => (
                <li key={k} className="flex items-center justify-between py-2">
                  <span className="text-slate-800">{f.name} <span className="text-slate-400">· {Math.max(1, Math.round(f.size / 1024))} KB</span></span>
                  <button onClick={() => remover(k)} className="text-sm text-slate-500 hover:text-red-700">Remover</button>
                </li>
              ))}
            </ul>
          </Cartao>
        )}

        {temPlanilha && previa && mapa && <Cartao><MapearColunas nome={previa.nome} previa={previa.previa} valor={mapa} onChange={setMapa} /></Cartao>}

        <Erro>{erro}</Erro>
        {arquivos.length > 0 && (
          <div className="flex items-center gap-3">
            <Botao onClick={importar} disabled={ocupado || !!pendencia}>{ocupado ? "Importando…" : "Importar"}</Botao>
            {pendencia && <span className="text-sm text-amber-800">{pendencia}</span>}
          </div>
        )}

        {resumo && (
          <Cartao>
            <h2 className="mb-3 font-medium text-slate-900">Importação concluída</h2>
            <ul className="space-y-1 text-sm text-slate-700">
              <li>{plural(resumo.itensNovos, "item novo", "itens novos")}</li>
              <li>{plural(resumo.itensAtualizados, "item atualizado", "itens atualizados")}</li>
              <li>{plural(resumo.notasRepetidas, "nota já importada antes (ignorada)", "notas já importadas antes (ignoradas)")}</li>
            </ul>
            {resumo.ignorados.length > 0 && (
              <div className="mt-4">
                <p className="text-sm font-medium text-amber-900">{plural(resumo.ignorados.length, "arquivo ou linha ignorado", "arquivos ou linhas ignorados")}:</p>
                <ul className="mt-1 list-disc space-y-0.5 pl-5 text-sm text-slate-700">
                  {resumo.ignorados.map((g, k) => <li key={k}><span className="font-medium">{g.arquivo}</span>: {g.motivo}</li>)}
                </ul>
              </div>
            )}
            <Link href={`/clientes/${id}`} className="mt-4 inline-block rounded-md bg-teal-800 px-4 py-2 text-sm font-medium text-white hover:bg-teal-900">Ver itens</Link>
          </Cartao>
        )}
      </div>
    </>
  );
}

export default function Importar() {
  return (
    <Suspense>
      <Shell><Tela /></Shell>
    </Suspense>
  );
}
