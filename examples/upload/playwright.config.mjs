import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./browser",
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: process.env.UPLOAD_BASE_URL ?? "http://127.0.0.1:8080",
    trace: "on",
  },
});
