"use client";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState, type ReactNode } from "react";
import { supabase } from "@/lib/supabase";

const abas = [["/clientes", "Clientes"], ["/aliquotas", "Alíquotas"]];

// Moldura das telas autenticadas: cabeçalho, navegação e redirecionamento ao login.
export default function Shell({ children }: { children: ReactNode }) {
  const router = useRouter();
  const path = usePathname();
  const [pronto, setPronto] = useState(false);
  useEffect(() => {
    supabase.auth.getSession().then(({ data }) => (data.session ? setPronto(true) : router.replace("/login")));
  }, [router]);
  if (!pronto) return null;
  return (
    <>
      <header className="border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-5xl items-center gap-8 px-6 py-3">
          <span className="font-semibold text-teal-900">RTC Hub</span>
          <nav className="flex flex-1 gap-5 text-sm">
            {abas.map(([href, nome]) => (
              <Link key={href} href={href} className={path.startsWith(href) ? "font-medium text-teal-800" : "text-slate-600 hover:text-slate-900"}>
                {nome}
              </Link>
            ))}
          </nav>
          <button onClick={() => supabase.auth.signOut().then(() => router.replace("/login"))} className="text-sm text-slate-600 hover:text-slate-900">
            Sair
          </button>
        </div>
      </header>
      <main className="mx-auto max-w-5xl px-6 py-8">{children}</main>
    </>
  );
}
