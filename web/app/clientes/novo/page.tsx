"use client";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import Shell from "@/components/Shell";
import { Botao, Campo, Cartao, Erro, Titulo } from "@/components/ui";
import { api } from "@/lib/api";

export default function NovoCliente() {
  const router = useRouter();
  const [regime, setRegime] = useState("regular");
  const [erro, setErro] = useState("");
  const [ocupado, setOcupado] = useState(false);

  async function salvar(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const txt = (k: string) => String(f.get(k) ?? "").trim();
    const pct = (k: string) => Number(txt(k).replace(",", "."));
    const corpo = {
      razaoSocial: txt("razao"), cnpj: txt("cnpj").replace(/\D/g, ""), regime, uf: txt("uf").toUpperCase(), municipioIbge: Number(txt("ibge")),
      ...(regime === "simples" && { cAtivSn: txt("cativ"), pIbsSn: pct("pibs"), pCbsSn: pct("pcbs") }),
    };
    const num = (v: number) => Number.isFinite(v);
    if (corpo.cnpj.length !== 14) return setErro("CNPJ deve ter 14 dígitos.");
    if (!/^\d{7}$/.test(txt("ibge"))) return setErro("Código IBGE do município deve ter 7 dígitos.");
    if (regime === "simples") {
      if (!/^\d{2}$/.test(txt("cativ"))) return setErro("Atividade do Simples deve ter 2 dígitos (01 a 15).");
      if (!num(pct("pibs")) || !num(pct("pcbs")) || !txt("pibs") || !txt("pcbs")) return setErro("Informe as alíquotas efetivas de IBS e CBS como número.");
    }
    setErro("");
    setOcupado(true);
    try {
      await api("/clientes", { method: "POST", body: JSON.stringify(corpo) });
      router.push("/clientes");
    } catch (err) {
      setErro((err as Error).message);
      setOcupado(false);
    }
  }

  return (
    <Shell>
      <Titulo>Novo cliente</Titulo>
      <div className="max-w-xl"><Cartao>
        <form onSubmit={salvar} className="space-y-4">
          <Campo rotulo="Razão social" name="razao" required />
          <Campo rotulo="CNPJ" name="cnpj" required inputMode="numeric" placeholder="00.000.000/0000-00" />
          <label className="block">
            <span className="mb-1 block text-sm font-medium text-slate-700">Regime</span>
            <select value={regime} onChange={(e) => setRegime(e.target.value)} className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm">
              <option value="regular">Regime regular</option>
              <option value="simples">Simples</option>
            </select>
          </label>
          <div className="grid grid-cols-2 gap-4">
            <Campo rotulo="UF" name="uf" required maxLength={2} placeholder="SP" />
            <Campo rotulo="Código IBGE do município" name="ibge" required inputMode="numeric" maxLength={7} />
          </div>
          {regime === "simples" && (
            <div className="space-y-4 rounded-md bg-slate-50 p-4">
              <Campo rotulo="Atividade do Simples (cAtivSN)" name="cativ" required maxLength={2} placeholder="01" ajuda="Código de 2 dígitos, de 01 a 15." />
              <div className="grid grid-cols-2 gap-4">
                <Campo rotulo="Alíquota efetiva IBS (%)" name="pibs" required inputMode="decimal" ajuda="As mesmas do cálculo do DAS." />
                <Campo rotulo="Alíquota efetiva CBS (%)" name="pcbs" required inputMode="decimal" ajuda="As mesmas do cálculo do DAS." />
              </div>
            </div>
          )}
          <Erro>{erro}</Erro>
          <Botao type="submit" disabled={ocupado}>Salvar cliente</Botao>
        </form>
      </Cartao></div>
    </Shell>
  );
}
