import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { test, expect } from "@playwright/test";

const filename = "山景.jpg";
const fixture = fileURLToPath(new URL("../fixtures/山景.jpg", import.meta.url));
const sha256 = (bytes) => createHash("sha256").update(bytes).digest("hex");

async function chooseFile(page) {
  const chooser = page.waitForEvent("filechooser");
  await page.getByRole("button", { name: "选择文件" }).click();
  await (await chooser).setFiles(fixture);
  await expect(page.locator("#selected")).toHaveText(filename);
}

async function uploadFile(page) {
  await page.goto("/");
  await page.getByRole("button", { name: "以测试用户登录" }).click();
  await expect(page.getByText("已登录 · 测试用户")).toBeVisible();
  await chooseFile(page);
  const put = page.waitForResponse((response) => response.request().method() === "PUT");
  await page.getByRole("button", { name: "上传文件" }).click();
  const stored = await put;
  expect(stored.ok()).toBeTruthy();
  expect(new URL(stored.url()).origin).not.toBe(new URL(page.url()).origin);
  await expect(page.getByText("上传成功：山景.jpg")).toBeVisible();
  await expect(page.getByRole("list", { name: "我的文件列表" }).getByText(filename).first()).toBeVisible();
}

async function downloadAndCompare(page) {
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "下载原文件 " + filename }).first().click();
  const saved = await download;
  expect(saved.suggestedFilename()).toBe(filename);
  const actual = sha256(await readFile(await saved.path()));
  const expected = sha256(await readFile(fixture));
  expect(actual).toBe(expected);
  console.log("下载文件的 SHA-256 与原图一致", actual);
  await expect(page.getByText("已下载 山景.jpg")).toBeVisible();
}

test("上传原图", async ({ page }) => {
  await uploadFile(page);
  await downloadAndCompare(page);
});

test("再次找到文件", async ({ page }) => {
  await uploadFile(page);
  const reload = page.waitForEvent("load");
  await page.getByRole("button", { name: "刷新页面" }).click();
  await reload;
  await expect(page.locator("#selected")).toHaveText("未选择文件");
  await expect(page.getByRole("list", { name: "我的文件列表" }).getByText(filename).first()).toBeVisible();
  await downloadAndCompare(page);
});

test("未登录时上传", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByText("访客", { exact: true })).toBeVisible();
  await chooseFile(page);
  const rejected = page.waitForResponse((response) => new URL(response.url()).pathname === "/v1/upload/presign");
  await page.getByRole("button", { name: "上传文件" }).click();
  expect((await rejected).status()).toBe(401);
  await expect(page.getByText("请先登录再继续。")).toBeVisible();
  await expect(page.getByText("上传成功", { exact: false })).toHaveCount(0);
});
