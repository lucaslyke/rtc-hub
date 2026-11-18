"use client";
import { useState, type FormEvent } from "react";
import { Botao, Campo, Erro } from "@/components/ui";

export type DadosCliente = {
  razaoSocial: string; cnpj: string; regime: string; uf: string; municipioIbge: number;
  cAtivSn?: string; pIbsSn?: number | null; pCbsSn?: number | null;
};

// FormCliente valida no navegador e chama onSalvar; o erro da api volta para a tela.
export default function FormCliente({ inicial, onSalvar }: { inicial?: DadosCliente; onSalvar: (c: DadosCliente) => Promise<void> }) {
  const [regime, setRegime] = useState(inicial?.regime ?? "regular");
  const [erro, setErro] = useState("");
  const [ocupado, setOcupado] = useState(false);

  async function salvar(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const txt = (k: string) => String(f.get(k) ?? "").trim();
    const pct = (k: string) => Number(txt(k).replace(",", "."));
    const corpo: DadosCliente = {
      razaoSocial: txt("razao"), cnpj: txt("cnpj").replace(/\D/g, ""), regime, uf: txt("uf").toUpperCase(), municipioIbge: Number(txt("ibge")),
      ...(regime === "simples" && { cAtivSn: txt("cativ"), pIbsSn: pct("pibs"), pCbsSn: pct("pcbs") }),
    };
    const faixa = (v: number) => Number.isFinite(v) && v >= 0 && v <= 100;
    if (corpo.cnpj.length !== 14) return setErro("CNPJ deve ter 14 dígitos.");
    if (!/^\d{7}$/.test(txt("ibge"))) return setErro("Código IBGE do município deve ter 7 dígitos.");
    if (regime === "simples") {
      const c = Number(txt("cativ"));
      if (!/^\d{2}$/.test(txt("cativ")) || c < 1 || c > 15) return setErro("Atividade do Simples deve ter 2 dígitos (01 a 15).");
      if (!faixa(pct("pibs")) || !faixa(pct("pcbs")) || !txt("pibs") || !txt("pcbs")) return setErro("Informe as alíquotas efetivas de IBS e CBS como número entre 0 e 100.");
    }
    setErro("");
    setOcupado(true);
    try {
      await onSalvar(corpo);
    } catch (err) {
      setErro((err as Error).message);
      setOcupado(false);
    }
  }

  const num = (v?: number | null) => (v == null ? "" : String(v).replace(".", ","));
  return (
    <form onSubmit={salvar} className="space-y-4">
      <Campo rotulo="Razão social" name="razao" required defaultValue={inicial?.razaoSocial} />
      <Campo rotulo="CNPJ" name="cnpj" required inputMode="numeric" placeholder="00.000.000/0000-00" defaultValue={inicial?.cnpj} />
      <label className="block">
        <span className="mb-1 block text-sm font-medium text-slate-700">Regime</span>
        <select value={regime} onChange={(e) => setRegime(e.target.value)} className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm">
          <option value="regular">Regime regular</option>
          <option value="simples">Simples</option>
        </select>
      </label>
      <div className="grid grid-cols-2 gap-4">
        <Campo rotulo="UF" name="uf" required maxLength={2} placeholder="SP" defaultValue={inicial?.uf} />
        <Campo rotulo="Código IBGE do município" name="ibge" required inputMode="numeric" maxLength={7} defaultValue={inicial?.municipioIbge} />
      </div>
      {regime === "simples" && (
        <div className="space-y-4 rounded-md bg-slate-50 p-4">
          <Campo rotulo="Atividade do Simples (cAtivSN)" name="cativ" required maxLength={2} placeholder="01" ajuda="Código de 2 dígitos, de 01 a 15." defaultValue={inicial?.cAtivSn} />
          <div className="grid grid-cols-2 gap-4">
            <Campo rotulo="Alíquota efetiva IBS (%)" name="pibs" required inputMode="decimal" ajuda="As mesmas do cálculo do DAS." defaultValue={num(inicial?.pIbsSn)} />
            <Campo rotulo="Alíquota efetiva CBS (%)" name="pcbs" required inputMode="decimal" ajuda="As mesmas do cálculo do DAS." defaultValue={num(inicial?.pCbsSn)} />
          </div>
        </div>
      )}
      <Erro>{erro}</Erro>
      <Botao type="submit" disabled={ocupado}>Salvar cliente</Botao>
    </form>
  );
}
