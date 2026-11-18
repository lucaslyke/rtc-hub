import { createClient } from "@supabase/supabase-js";

// Fallbacks só deixam o build passar sem env; em execução as variáveis são obrigatórias.
export const supabase = createClient(
  process.env.NEXT_PUBLIC_SUPABASE_URL || "http://localhost:54321",
  process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY || "sem-chave",
);
