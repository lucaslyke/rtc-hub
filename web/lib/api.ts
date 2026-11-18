import { supabase } from "./supabase";

export const API_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8090";

export async function api<T>(path: string, init: RequestInit = {}): Promise<T> {
  const { data } = await supabase.auth.getSession();
  const headers = new Headers(init.headers);
  if (data.session) headers.set("Authorization", `Bearer ${data.session.access_token}`);
  if (init.body && !headers.has("Content-Type")) headers.set("Content-Type", "application/json");
  let r: Response;
  try {
    r = await fetch(API_URL + path, { ...init, headers });
  } catch {
    throw new Error("Não foi possível conectar ao servidor. Tente de novo em instantes.");
  }
  if (r.status === 401) {
    await supabase.auth.signOut();
    window.location.href = "/login";
  }
  if (!r.ok) {
    const corpo = await r.json().catch(() => ({}));
    throw new Error(corpo.erro || "Erro inesperado. Tente de novo.");
  }
  return r.status === 204 ? (undefined as T) : r.json();
}
