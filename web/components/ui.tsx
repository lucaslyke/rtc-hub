import type { ReactNode, InputHTMLAttributes, ButtonHTMLAttributes } from "react";

export const Titulo = ({ children, acao }: { children: ReactNode; acao?: ReactNode }) => (
  <div className="mb-6 flex items-center justify-between">
    <h1 className="text-xl font-semibold text-slate-900">{children}</h1>
    {acao}
  </div>
);

export const Cartao = ({ children }: { children: ReactNode }) => (
  <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">{children}</div>
);

export function Campo({ rotulo, ajuda, ...p }: { rotulo: string; ajuda?: string } & InputHTMLAttributes<HTMLInputElement>) {
  return (
    <label className="block">
      <span className="mb-1 block text-sm font-medium text-slate-700">{rotulo}</span>
      <input {...p} className="w-full rounded-md border border-slate-300 px-3 py-2 text-sm focus:border-teal-700 focus:outline-none focus:ring-1 focus:ring-teal-700" />
      {ajuda && <span className="mt-1 block text-xs text-slate-500">{ajuda}</span>}
    </label>
  );
}

export function Botao({ secundario, ...p }: { secundario?: boolean } & ButtonHTMLAttributes<HTMLButtonElement>) {
  const cor = secundario
    ? "border border-slate-300 bg-white text-slate-700 hover:bg-slate-50"
    : "bg-teal-800 text-white hover:bg-teal-900";
  return <button {...p} className={`rounded-md px-4 py-2 text-sm font-medium disabled:opacity-50 ${cor}`} />;
}

export const Erro = ({ children }: { children?: ReactNode }) =>
  children ? <p role="alert" className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-800">{children}</p> : null;

export const Aviso = ({ children }: { children: ReactNode }) => (
  <p className="rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-900">{children}</p>
);
