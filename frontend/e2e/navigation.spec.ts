import { expect, test } from "@playwright/test";

test("site overview → devices → device detail", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Sites" })).toBeVisible();

  await page.getByRole("link", { name: /Bangkok Data Center/ }).click();
  await expect(page.getByRole("heading", { name: "Bangkok Data Center" })).toBeVisible();
  await expect(page.getByRole("link", { name: "temp-sensor-01" })).toBeVisible();

  await page.getByRole("link", { name: "temp-sensor-01" }).click();
  await expect(page.getByRole("heading", { name: /temp-sensor-01/ })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Telemetry" })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
});
