"use client";
import Link from "next/link";
import { useEffect, useState } from "react";
import Shell from "@/components/Shell";
import { Cartao, Erro, Titulo } from "@/components/ui";
import { api } from "@/lib/api";

type Cliente = { id: string; razaoSocial: string; cnpj: string; regime: string; uf: string; contagem: Record<string, number> };

const cnpj = (d: string) => d.replace(/^(\d{2})(\d{3})(\d{3})(\d{4})(\d{2})$/, "$1.$2.$3/$4-$5");

export default function Clientes() {
  const [lista, setLista] = useState<Cliente[] | null>(null);
  const [erro, setErro] = useState("");
  useEffect(() => {
    api<Cliente[]>("/clientes").then(setLista, (e) => setErro(e.message));
  }, []);
  return (
    <Shell>
      <Titulo acao={<Link href="/clientes/novo" className="rounded-md bg-teal-800 px-4 py-2 text-sm font-medium text-white hover:bg-teal-900">Novo cliente</Link>}>Clientes</Titulo>
      <Erro>{erro}</Erro>
      {lista && lista.length === 0 && <Cartao><p className="text-sm text-slate-600">Nenhum cliente cadastrado ainda. Cadastre o primeiro para importar os itens.</p></Cartao>}
      {lista && lista.length > 0 && (
        <div className="overflow-hidden rounded-lg border border-slate-200 bg-white shadow-sm">
          <table className="w-full text-left text-sm">
            <thead className="bg-slate-50 text-xs uppercase tracking-wide text-slate-500">
              <tr>
                <th className="px-4 py-3">Razão social</th><th className="px-4 py-3">CNPJ</th><th className="px-4 py-3">Regime</th>
                <th className="px-4 py-3 text-right">Prontos</th><th className="px-4 py-3 text-right">Prováveis (confirmar)</th><th className="px-4 py-3 text-right">Revisar</th><th className="px-4 py-3 text-right">Inválidos</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-100">
              {lista.map((c) => (
                <tr key={c.id} className="hover:bg-slate-50">
                  <td className="px-4 py-3"><Link href={`/clientes/${c.id}`} className="font-medium text-teal-800 hover:underline">{c.razaoSocial}</Link></td>
                  <td className="px-4 py-3 tabular-nums text-slate-600">{cnpj(c.cnpj)}</td>
                  <td className="px-4 py-3 text-slate-600">{c.regime === "simples" ? "Simples Nacional" : "Regime regular"}</td>
                  <td className="px-4 py-3 text-right tabular-nums text-green-700">✓ {c.contagem.ok}</td>
                  <td className="px-4 py-3 text-right tabular-nums text-sky-700">? {c.contagem.provavel}</td>
                  <td className="px-4 py-3 text-right tabular-nums text-amber-700">⚠ {c.contagem.revisar}</td>
                  <td className="px-4 py-3 text-right tabular-nums text-red-700">✗ {c.contagem.invalido}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Shell>
  );
}
