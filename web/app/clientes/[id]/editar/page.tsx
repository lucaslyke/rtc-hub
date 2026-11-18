"use client";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import FormCliente, { type DadosCliente } from "@/components/FormCliente";
import Shell from "@/components/Shell";
import { Botao, Cartao, Erro, Titulo } from "@/components/ui";
import { api } from "@/lib/api";

function Tela() {
  const { id } = useParams<{ id: string }>();
  const router = useRouter();
  const [cliente, setCliente] = useState<DadosCliente | null>(null);
  const [erro, setErro] = useState("");
  useEffect(() => { api<DadosCliente>(`/clientes/${id}`).then(setCliente, (e) => setErro(e.message)); }, [id]);

  async function excluir() {
    if (!window.confirm(`Excluir ${cliente?.razaoSocial}? Os itens, cálculos e o histórico de importações deste cliente serão apagados.`)) return;
    try {
      await api(`/clientes/${id}`, { method: "DELETE" });
      router.push("/clientes");
    } catch (e) {
      setErro((e as Error).message);
    }
  }

  return (
    <>
      <Titulo acao={<Link href={`/clientes/${id}`} className="text-sm text-teal-800 hover:underline">← Voltar aos itens</Link>}>Editar cliente</Titulo>
      <div className="max-w-xl space-y-4">
        <Erro>{erro}</Erro>
        {cliente && (
          <>
            <Cartao>
              <FormCliente inicial={cliente} onSalvar={async (c) => { await api(`/clientes/${id}`, { method: "PUT", body: JSON.stringify(c) }); router.push(`/clientes/${id}`); }} />
            </Cartao>
            <p className="text-xs text-slate-500">Mudar regime, UF, município ou as alíquotas do Simples descarta os cálculos já feitos deste cliente.</p>
            <Botao secundario onClick={excluir}>Excluir cliente</Botao>
          </>
        )}
      </div>
    </>
  );
}

export default function EditarCliente() {
  return (
    <Suspense>
      <Shell><Tela /></Shell>
    </Suspense>
  );
}
