import { expect, test } from "@playwright/test";
import { publishTempNormal, publishTempSpike, waitForAlertStatus } from "./helpers/backend";

// Runs against the real stack (Vite dev server + real api + real
// Postgres/Mongo/Mosquitto from infra/docker-compose.yml), not a mocked
// API — see docs/frontend-design.md. Each test drives a real MQTT message
// through ingestion's actual alert engine rather than seeding the
// database directly, so it also proves the pipeline these tests are
// meant to guard actually works end to end.

test.beforeEach(async () => {
  await publishTempSpike();
  await waitForAlertStatus("open");
});

test.afterEach(async () => {
  // Leave temp-sensor-01 clear for the next test/run.
  await publishTempNormal();
  await waitForAlertStatus("resolved");
});

test("alerts list shows the triggered alert", async ({ page }) => {
  await page.goto("/alerts");
  await expect(page.getByRole("heading", { name: "Alerts" })).toBeVisible();
  await expect(page.locator(".alert-table tbody tr").first()).toBeVisible();
  await expect(page.locator(".badge--warning, .badge--critical").first()).toBeVisible();
});

test("acknowledging an alert updates its status", async ({ page }) => {
  await page.goto("/alerts"); // default filter: status=open

  // The spike trips both of temp-sensor-01's rules (warning + critical).
  // The "open" filter naturally excludes every resolved alert from past
  // runs, so exactly one warning-severity row should be open here.
  const openWarningRow = page.locator(".alert-table tbody tr").filter({ has: page.locator(".badge--warning") });
  await expect(openWarningRow).toHaveCount(1);
  await openWarningRow.getByRole("button", { name: "Acknowledge" }).click();

  // Acknowledging removes the row from an "open"-filtered view (it's no
  // longer open) — that's real, correct behavior, not a bug. Switch to
  // the "acknowledged" filter to see it land there with updated status.
  await page.getByLabel("Status").selectOption("acknowledged");
  const ackedRow = page.locator(".alert-table tbody tr").filter({ has: page.locator(".badge--warning") });
  await expect(ackedRow).toContainText("acknowledged");
  await expect(ackedRow.getByRole("button", { name: "Acknowledge" })).toHaveCount(0);
  await expect(ackedRow.getByRole("button", { name: "Resolve" })).toBeVisible();
});

test("status filter narrows the list to the selected status", async ({ page }) => {
  await page.goto("/alerts");
  await page.getByLabel("Status").selectOption("acknowledged");
  await expect(page.locator(".alert-table tbody tr")).toHaveCount(0);

  await page.getByLabel("Status").selectOption("open");
  await expect(page.locator(".alert-table tbody tr").first()).toBeVisible();
});
