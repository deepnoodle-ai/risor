import { expect, test } from "@playwright/test";

const field = (page: import("@playwright/test").Page) => page.locator(".risor-mantine-input .cm-content");

test("colours, a check error at its place, and the message under the field", async ({ page }) => {
  await page.goto("/");
  await expect(field(page)).toBeVisible();
  await expect(page.locator(".risor-mantine-input .risor-t-keyword, .risor-mantine-input .risor-t-operator").first()).toBeVisible();
  await field(page).click();
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.type("order.totl > 1");
  await expect(page.locator(".risor-mantine-input .cm-lintRange-error")).toHaveText("totl");
  await expect(page.getByText(/has no field "totl"/).first()).toBeVisible();
});

test("completion of a record's fields after a dot", async ({ page }) => {
  await page.goto("/");
  await field(page).click();
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.type("order.");
  const list = page.locator(".cm-tooltip-autocomplete");
  await expect(list).toBeVisible();
  await expect(list.getByText("total", { exact: true })).toBeVisible();
  await list.getByText("total", { exact: true }).click();
  await expect(field(page)).toHaveText("order.total");
});

test("a submit right after an error is typed is blocked by validate()", async ({ page }) => {
  await page.goto("/");
  await field(page).click();
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.type("order.total");
  await page.getByRole("button", { name: "Save" }).click(); // before the pause ends
  await expect(page.getByTestId("submitted")).toContainText("Blocked");
  await field(page).click();
  await page.keyboard.type(" > 1");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("submitted")).toHaveText("Saved: order.total > 1");
});

test("the preview runs in the worker; with a host function it says to use run", async ({ page }) => {
  await page.goto("/");
  await expect(page.locator(".risor-mantine-preview .risor-v-bool")).toHaveText("true");
  await page.getByText("With a host function").click();
  await expect(page.locator(".risor-mantine-preview")).toContainText("host-functions");
});

test("the playground: a result tree, and a long run ends with a limit error while the page stays responsive", async ({ page }) => {
  // The deadline and the worker restart are tested with a fake worker in
  // @risor/service: a program cannot hang past its own deadline here.
  await page.goto("/");
  const result = page.locator(".risor-result").last();
  await expect(result.locator(".risor-v-key").first()).toBeVisible();
  await expect(result).toContainText("Ada (gold)");
  const editor = page.locator('[aria-label="Program"]');
  await editor.click();
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.type("len(range(100000000).map(i => i))");
  await expect(result).toContainText(/steps-exceeded|allocation-exceeded|deadline-exceeded/, { timeout: 10_000 });
  await page.keyboard.type(" ");
});

test("an empty field shows no error, and a required one blocks the submit", async ({ page }) => {
  await page.goto("/");
  await field(page).click();
  await page.keyboard.press("ControlOrMeta+a");
  // First an error, so the test sees a check run, and the invalid state on the editable element.
  await page.keyboard.type("order.totl");
  await expect(page.locator(".risor-mantine-input .cm-lintRange-error")).toHaveText("totl");
  await expect(page.locator(".risor-mantine-input .cm-content")).toHaveAttribute("aria-invalid", "true");
  // Then empty: the error goes, and no new one comes.
  await page.keyboard.press("ControlOrMeta+a");
  await page.keyboard.press("Backspace");
  await expect(page.locator(".risor-mantine-input .cm-lintRange-error")).toHaveCount(0);
  await expect(page.locator(".risor-mantine-input .cm-content")).not.toHaveAttribute("aria-invalid", "true");
  await expect(page.locator(".risor-mantine-input.risor-editor-invalid")).toHaveCount(0);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByTestId("submitted")).toHaveText("Blocked: Enter an expression.");
});
