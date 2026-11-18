import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "RTC Hub",
  description: "Classificação tributária para a reforma (CBS, IBS e IS)",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="pt-BR">
      <body>{children}</body>
    </html>
  );
}
