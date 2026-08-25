import { expect, test } from "@playwright/test";
import { publishTempNormal, publishTempSpike, waitForAlertStatus } from "./helpers/backend";

// Runs against the real stack (Vite dev server + real api + real
// Postgres/Mongo/Mosquitto from infra/docker-compose.yml), not a mocked
// API — see docs/frontend-design.md. Each test drives a real MQTT message
// through ingestion's actual alert engine rather than seeding the
// database directly, so it also proves the pipeline these tests are
// meant to guard actually works end to end.
//
// Selectors go through data-testid / data-* attributes (see
// components/indicators.tsx, AlertTable.tsx), not CSS classes — a visual
// redesign (this project has already had one) changes class names but
// has no reason to touch these.

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
  await expect(page.getByTestId("alert-row").first()).toBeVisible();
  await expect(page.getByTestId("alert-severity").first()).toBeVisible();
});

test("acknowledging an alert updates its status", async ({ page }) => {
  await page.goto("/alerts"); // default filter: status=open

  // The spike trips both of temp-sensor-01's rules (warning + critical).
  // The "open" filter naturally excludes every resolved alert from past
  // runs, so exactly one warning-severity row should be open here.
  const openWarningRow = page.getByTestId("alert-row").filter({ has: page.locator('[data-testid="alert-severity"][data-severity="warning"]') });
  await expect(openWarningRow).toHaveCount(1);
  await openWarningRow.getByRole("button", { name: "Acknowledge" }).click();

  // Acknowledging removes the row from an "open"-filtered view (it's no
  // longer open) — that's real, correct behavior, not a bug. Switch to
  // the "acknowledged" filter to see it land there with updated status.
  await page.getByLabel("Status").selectOption("acknowledged");
  const ackedRow = page.getByTestId("alert-row").filter({ has: page.locator('[data-testid="alert-severity"][data-severity="warning"]') });
  await expect(ackedRow.getByTestId("alert-status")).toHaveAttribute("data-status", "acknowledged");
  await expect(ackedRow.getByRole("button", { name: "Acknowledge" })).toHaveCount(0);
  await expect(ackedRow.getByRole("button", { name: "Resolve" })).toBeVisible();
});

test("status filter narrows the list to the selected status", async ({ page }) => {
  await page.goto("/alerts");
  await page.getByLabel("Status").selectOption("acknowledged");
  await expect(page.getByTestId("alert-row")).toHaveCount(0);

  await page.getByLabel("Status").selectOption("open");
  await expect(page.getByTestId("alert-row").first()).toBeVisible();
});
