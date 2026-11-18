import { test, expect } from "@playwright/test";

// Depende do RPC clonar_demo (Task 11).
test.fixme("entrar como demo mostra o cliente de exemplo", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("button", { name: "Entrar como demo" }).click();
  await expect(page.getByText("Padaria")).toBeVisible();
});
