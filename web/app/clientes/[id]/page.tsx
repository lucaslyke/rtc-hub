import { Suspense } from "react";
import Shell from "@/components/Shell";
import { Cartao, Titulo } from "@/components/ui";

// Placeholder: a tela do cliente (importação e revisão) vem na próxima etapa.
export default function ClienteDetalhe() {
  return (
    <Suspense>
    <Shell>
      <Titulo>Cliente</Titulo>
      <Cartao><p className="text-sm text-slate-600">Importação e revisão dos itens em breve.</p></Cartao>
    </Shell>
    </Suspense>
  );
}
