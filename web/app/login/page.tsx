"use client";
import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { supabase } from "@/lib/supabase";
import { Botao, Campo, Cartao, Erro } from "@/components/ui";

export default function Login() {
  const router = useRouter();
  const [email, setEmail] = useState("");
  const [senha, setSenha] = useState("");
  const [criar, setCriar] = useState(false);
  const [erro, setErro] = useState("");
  const [ocupado, setOcupado] = useState(false);

  async function executar(fn: () => Promise<string | undefined>) {
    setErro("");
    setOcupado(true);
    const msg = await fn().catch(() => "Não foi possível conectar. Tente de novo.");
    setOcupado(false);
    if (msg) setErro(msg);
    else router.replace("/clientes");
  }

  const enviar = (e: FormEvent) => {
    e.preventDefault();
    return executar(async () => {
      const { error } = criar ? await supabase.auth.signUp({ email, password: senha }) : await supabase.auth.signInWithPassword({ email, password: senha });
      if (error) return criar ? "Não foi possível criar a conta. Confira o e-mail e use uma senha de ao menos 6 caracteres." : "E-mail ou senha incorretos.";
    });
  };

  const demo = () =>
    executar(async () => {
      const a = await supabase.auth.signInAnonymously();
      if (a.error) return "Não foi possível iniciar a demonstração agora.";
      const r = await supabase.rpc("clonar_demo");
      if (r.error) return "A demonstração ainda não está disponível. Tente novamente mais tarde.";
    });

  return (
    <main className="mx-auto mt-24 max-w-sm px-6">
      <h1 className="mb-1 text-2xl font-semibold text-teal-900">RTC Hub</h1>
      <p className="mb-6 text-sm text-slate-600">Sugestões de CST e cClassTrib para a reforma tributária, com base nas tabelas oficiais da Receita Federal.</p>
      <Cartao>
        <form onSubmit={enviar} className="space-y-4">
          <Campo rotulo="E-mail" type="email" required value={email} onChange={(e) => setEmail(e.target.value)} />
          <Campo rotulo="Senha" type="password" required minLength={6} value={senha} onChange={(e) => setSenha(e.target.value)} />
          <Erro>{erro}</Erro>
          <Botao type="submit" disabled={ocupado}>{criar ? "Criar conta" : "Entrar"}</Botao>
          <button type="button" onClick={() => setCriar(!criar)} className="ml-4 text-sm text-teal-800 hover:underline">
            {criar ? "Já tenho conta" : "Criar conta"}
          </button>
        </form>
        <hr className="my-5 border-slate-200" />
        <Botao secundario onClick={demo} disabled={ocupado}>Entrar como demo</Botao>
        <p className="mt-2 text-xs text-slate-500">Explore com dados de exemplo, sem precisar de cadastro.</p>
      </Cartao>
    </main>
  );
}
