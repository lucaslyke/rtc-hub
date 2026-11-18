import { test, expect } from "@playwright/test";

test("entrar como demo mostra o cliente de exemplo", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("button", { name: "Entrar como demo" }).click();
  await expect(page.getByText("Padaria")).toBeVisible();
});

test("na demo, medicamento fica em revisão com as classes da Anvisa", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("button", { name: "Entrar como demo" }).click();
  await page.getByRole("link", { name: "Farmácia Saúde Ltda" }).click();
  const paracetamol = page.getByRole("row", { name: /Paracetamol/ });
  await expect(paracetamol).toContainText("Revisar");
  await expect(paracetamol).toContainText("Medicamentos: a redução depende de registro na Anvisa.");
  await expect(paracetamol.getByLabel(/Classificação de Paracetamol/)).toContainText("200009");
  await expect(paracetamol.getByLabel(/Classificação de Paracetamol/)).toContainText("200032");
});
