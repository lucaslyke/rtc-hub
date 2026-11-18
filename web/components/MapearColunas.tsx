"use client";

export type Previa = { cabecalho: string[]; linhas: string[][] };
export type Mapeamento = { descricao: number; codigo: number; ncm: number; nbs: number; valor: number };

const CAMPOS: [keyof Mapeamento, string, RegExp][] = [
  ["descricao", "Descrição do produto ou serviço", /descri|produto|nome/],
  ["valor", "Valor (opcional)", /valor|pre[cç]o/],
  ["ncm", "NCM", /ncm/],
  ["nbs", "NBS", /nbs/],
  ["codigo", "Código do item", /c[oó]d/],
];

// sugestao: primeira coluna cujo título lembra o campo; -1 = não tem.
export function sugerirMapeamento(cabecalho: string[]): Mapeamento {
  const m = { descricao: -1, codigo: -1, ncm: -1, nbs: -1, valor: -1 };
  for (const [campo, , re] of CAMPOS) m[campo] = cabecalho.findIndex((c) => re.test(c.toLowerCase()));
  return m;
}

export function faltando(m: Mapeamento) {
  if (m.descricao < 0) return "Escolha a coluna da descrição.";
  if (m.ncm < 0 && m.nbs < 0) return "Escolha a coluna de NCM ou a de NBS (ao menos uma).";
  return "";
}

export default function MapearColunas({ nome, previa, valor, onChange }: {
  nome: string; previa: Previa; valor: Mapeamento; onChange: (m: Mapeamento) => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <h2 className="font-medium text-slate-900">Colunas da planilha</h2>
        <p className="text-sm text-slate-600">Prévia de <strong>{nome}</strong>. Indique em qual coluna está cada informação.</p>
      </div>
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {CAMPOS.map(([campo, rotulo]) => (
          <label key={campo} className="block text-sm">
            <span className="mb-1 block font-medium text-slate-700">{rotulo}</span>
            <select value={valor[campo]} onChange={(e) => onChange({ ...valor, [campo]: Number(e.target.value) })}
              className="w-full rounded-md border border-slate-300 bg-white px-3 py-2 text-sm">
              <option value={-1}>Não tem</option>
              {previa.cabecalho.map((c, k) => <option key={k} value={k}>{c || `Coluna ${k + 1}`}</option>)}
            </select>
          </label>
        ))}
      </div>
      <div className="overflow-x-auto rounded-md border border-slate-200">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-50 text-slate-600">
            <tr>{previa.cabecalho.map((c, k) => <th key={k} className="px-3 py-2 font-medium">{c || `Coluna ${k + 1}`}</th>)}</tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {previa.linhas.map((l, r) => (
              <tr key={r}>{previa.cabecalho.map((_, k) => <td key={k} className="px-3 py-2 tabular-nums text-slate-700">{l[k]}</td>)}</tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
