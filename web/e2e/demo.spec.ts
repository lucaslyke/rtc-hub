import { test, expect } from "@playwright/test";

test("entrar como demo mostra o cliente de exemplo", async ({ page }) => {
  await page.goto("/login");
  await page.getByRole("button", { name: "Entrar como demo" }).click();
  await expect(page.getByText("Padaria")).toBeVisible();
});
