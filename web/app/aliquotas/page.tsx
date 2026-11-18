"use client";
import { useEffect, useState, type FormEvent } from "react";
import Shell from "@/components/Shell";
import { Aviso, Botao, Campo, Cartao, Erro, Titulo } from "@/components/ui";
import { api } from "@/lib/api";

const campos = [["cbs", "CBS (%)"], ["ibsUf", "IBS estadual (%)"], ["ibsMun", "IBS municipal (%)"], ["isAdValorem", "Imposto Seletivo, ad valorem (%)"]] as const;
type Valores = Record<(typeof campos)[number][0], string>;

export default function Aliquotas() {
  const [v, setV] = useState<Valores | null>(null);
  const [erro, setErro] = useState("");
  const [salvo, setSalvo] = useState(false);
  useEffect(() => {
    api<Record<string, number>>("/aliquotas").then(
      (a) => setV(Object.fromEntries(campos.map(([k]) => [k, String(a[k] ?? 0)])) as Valores),
      (e) => setErro(e.message),
    );
  }, []);

  async function salvar(e: FormEvent) {
    e.preventDefault();
    if (!v) return;
    setErro("");
    setSalvo(false);
    try {
      await api("/aliquotas", { method: "PUT", body: JSON.stringify(Object.fromEntries(campos.map(([k]) => [k, Number(v[k].replace(",", "."))]))) });
      setSalvo(true);
    } catch (err) {
      setErro((err as Error).message);
    }
  }

  return (
    <Shell>
      <Titulo>Alíquotas</Titulo>
      <div className="max-w-xl space-y-4">
        <Aviso>Estimativas para simulação, não são alíquotas oficiais.</Aviso>
        <Erro>{erro}</Erro>
        {v && (
          <Cartao>
            <form onSubmit={salvar} className="space-y-4">
              {campos.map(([k, rotulo]) => (
                <Campo key={k} rotulo={rotulo} inputMode="decimal" value={v[k]} onChange={(e) => setV({ ...v, [k]: e.target.value })} />
              ))}
              {salvo && <p role="status" className="text-sm text-green-700">Alíquotas salvas.</p>}
              <Botao type="submit">Salvar alíquotas</Botao>
            </form>
          </Cartao>
        )}
      </div>
    </Shell>
  );
}
