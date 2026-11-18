import { supabase } from "./supabase";

export const API_URL = process.env.NEXT_PUBLIC_API_URL!;
if (!API_URL) throw new Error("Defina NEXT_PUBLIC_API_URL (veja .env.example).");

async function enviar(path: string, init: RequestInit = {}): Promise<Response> {
  const { data } = await supabase.auth.getSession();
  const headers = new Headers(init.headers);
  if (data.session) headers.set("Authorization", `Bearer ${data.session.access_token}`);
  if (init.body && !(init.body instanceof FormData) && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  let r: Response;
  try {
    r = await fetch(API_URL + path, { ...init, headers });
  } catch {
    throw new Error("Não foi possível conectar ao servidor. Tente de novo em instantes.");
  }
  if (r.status === 401) {
    await supabase.auth.signOut();
    window.location.href = "/login";
    return new Promise<Response>(() => {}); // a página está saindo; não propaga erro
  }
  if (!r.ok) {
    const corpo = await r.json().catch(() => ({}));
    throw new Error(corpo.erro || "Erro inesperado. Tente de novo.");
  }
  return r;
}

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const r = await enviar(path, init);
  return r.status === 204 ? (undefined as T) : r.json();
}

// baixar: um link comum não manda o token, então busca o arquivo e salva via blob.
export async function baixar(path: string, nome: string) {
  const url = URL.createObjectURL(await (await enviar(path)).blob());
  const a = Object.assign(document.createElement("a"), { href: url, download: nome });
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
