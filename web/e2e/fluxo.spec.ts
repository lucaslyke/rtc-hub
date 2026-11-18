import { test, expect } from "@playwright/test";
import path from "node:path";

// Fluxo completo contra Supabase local, api e motores de pé (veja web/README.md).
test("importar, confirmar, calcular 2027 e exportar", async ({ page }) => {
  test.setTimeout(120_000);
  await page.goto("/login");
  await page.getByRole("button", { name: "Criar conta", exact: true }).first().click();
  await page.getByLabel("E-mail").fill(`fluxo-${Date.now()}@example.com`);
  await page.getByLabel("Senha").fill("senha-e2e-123");
  await page.getByRole("button", { name: "Criar conta" }).first().click();
  await page.waitForURL("**/clientes");

  // o emitente da nota de teste é 11222333000181
  await page.goto("/clientes/novo");
  await page.getByLabel("Razão social").fill("Mercado Teste");
  await page.getByLabel("CNPJ").fill("11.222.333/0001-81");
  await page.getByLabel("UF").fill("SP");
  await page.getByLabel("Código IBGE do município").fill("3550308");
  await page.getByRole("button", { name: "Salvar cliente" }).click();
  await page.waitForURL("**/clientes");
  await page.getByRole("link", { name: "Mercado Teste" }).click();

  await page.getByRole("link", { name: "Importar arquivos" }).click();
  await page.locator('input[type="file"]').setInputFiles(path.join(__dirname, "fixtures/nfe_saida.xml"));
  await page.getByRole("button", { name: "Importar", exact: true }).click();
  await expect(page.getByText("3 itens novos")).toBeVisible();
  await page.getByRole("link", { name: "Ver itens" }).click();

  const arroz = page.getByRole("row", { name: /Arroz tipo 1/ });
  const carne = page.getByRole("row", { name: /Carne bovina/ });
  await expect(arroz).toContainText("Revisar");
  await expect(carne).toContainText("Provável (confirmar)");

  await carne.getByRole("checkbox").check();
  await page.getByRole("button", { name: /Confirmar selecionados/ }).click();
  await expect(carne).toContainText("Pronto");

  await expect(page.getByLabel("Ano")).toHaveValue("2027");
  await page.getByRole("button", { name: "Calcular" }).click();
  await expect(carne.getByText("simulação")).toBeVisible({ timeout: 60_000 });

  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "Exportar XLSX" }).click();
  expect((await download).suggestedFilename()).toMatch(/\.xlsx$/);
});
