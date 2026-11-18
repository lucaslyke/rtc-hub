"use client";
import { useRouter } from "next/navigation";
import FormCliente from "@/components/FormCliente";
import Shell from "@/components/Shell";
import { Cartao, Titulo } from "@/components/ui";
import { api } from "@/lib/api";

export default function NovoCliente() {
  const router = useRouter();
  return (
    <Shell>
      <Titulo>Novo cliente</Titulo>
      <div className="max-w-xl"><Cartao>
        <FormCliente onSalvar={async (c) => { await api("/clientes", { method: "POST", body: JSON.stringify(c) }); router.push("/clientes"); }} />
      </Cartao></div>
    </Shell>
  );
}
