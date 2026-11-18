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

test("editar e excluir um cliente", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("button", { name: "Criar conta", exact: true }).first().click();
  await page.getByLabel("E-mail").fill(`e2e-ed-${Date.now()}@example.com`);
  await page.getByLabel("Senha").fill("senha-e2e-123");
  await page.getByRole("button", { name: "Criar conta" }).first().click();
  await page.waitForURL("**/clientes");
  await page.goto("/clientes/novo");
  await page.getByLabel("Razão social").fill("Loja Antiga");
  await page.getByLabel("CNPJ").fill("11222333000181");
  await page.getByLabel("UF").fill("SP");
  await page.getByLabel("Código IBGE do município").fill("3550308");
  await page.getByRole("button", { name: "Salvar cliente" }).click();
  await page.getByRole("link", { name: "Loja Antiga" }).click();

  await page.getByRole("link", { name: "Editar cliente" }).click();
  await expect(page.getByLabel("Razão social")).toHaveValue("Loja Antiga");
  await page.getByLabel("Razão social").fill("Loja Nova");
  await page.getByRole("button", { name: "Salvar cliente" }).click();
  await expect(page.getByRole("heading", { name: /Loja Nova/ })).toBeVisible();

  await page.getByRole("link", { name: "Editar cliente" }).click();
  page.once("dialog", (d) => d.accept());
  await page.getByRole("button", { name: "Excluir cliente" }).click();
  await page.waitForURL("**/clientes");
  await expect(page.getByText("Nenhum cliente cadastrado ainda")).toBeVisible();
});
