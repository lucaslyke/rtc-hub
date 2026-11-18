import { test, expect } from "@playwright/test";

test("escolher Simples mostra os campos do Simples", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("button", { name: "Criar conta", exact: true }).first().click();
  await page.getByLabel("E-mail").fill(`e2e-${Date.now()}@example.com`);
  await page.getByLabel("Senha").fill("senha-e2e-123");
  await page.getByRole("button", { name: "Criar conta" }).first().click();
  await page.waitForURL("**/clientes");
  await page.goto("/clientes/novo");
  await expect(page.getByText("Atividade do Simples")).toHaveCount(0);
  await page.getByLabel("Regime").selectOption("simples");
  await expect(page.getByText("Atividade do Simples (cAtivSN)")).toBeVisible();
});
