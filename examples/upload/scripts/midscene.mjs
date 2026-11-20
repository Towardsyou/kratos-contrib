import { spawnSync } from "node:child_process";
import { appendFileSync, existsSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { basename, join } from "node:path";
import { loadEnvFile } from "node:process";
import { parseYamlScript } from "@midscene/core/yaml";
import { chromium } from "@playwright/test";

if (existsSync(".env")) loadEnvFile(".env");
process.env.UPLOAD_BASE_URL ||= "http://127.0.0.1:8080";
process.env.MIDSCENE_RUN_DIR ||= "midscene_run";

const choices = process.argv.slice(2);
const files = readdirSync("midscene").filter((name) => name.endsWith(".yaml"));
for (const choice of choices) {
  if (!files.includes(choice + ".yaml")) {
    throw new Error("场景不存在：" + choice + "。可选场景：" + files.map((name) => basename(name, ".yaml")).join("、"));
  }
}
const scenes = files
  .filter((name) => choices.length === 0 || choices.includes(basename(name, ".yaml")))
  .map((name) => {
    const file = join("midscene", name);
    const { tasks } = parseYamlScript(readFileSync(file, "utf8"), file);
    return { file, name: tasks[0].name };
  });

const dir = "midscene_run";
const summaryPath = join(dir, "results.json");
mkdirSync(dir, { recursive: true });
const missing = [
  "MIDSCENE_MODEL_BASE_URL", "MIDSCENE_MODEL_API_KEY", "MIDSCENE_MODEL_NAME", "MIDSCENE_MODEL_FAMILY",
].filter((name) => !process.env[name]?.trim());
const skipReason = missing.length ? "缺少模型配置（" + missing.join("、") + "）" : "";
let results = scenes.map(({ file }) => ({ script: file, resultType: "notExecuted" }));
writeFileSync(summaryPath, JSON.stringify({ results, ...(skipReason && { skipReason }) }, null, 2) + "\n");

if (!skipReason) {
  if (!process.env.PUPPETEER_EXECUTABLE_PATH && existsSync(chromium.executablePath())) {
    process.env.PUPPETEER_EXECUTABLE_PATH = chromium.executablePath();
  }
  const run = spawnSync("midscene", [
    "--continue-on-error", "--summary", summaryPath, "--files", ...scenes.map(({ file }) => file),
  ], { stdio: "inherit" });
  if (run.error) console.error(run.error.message);
  process.exitCode = run.status ?? 1;
  results = JSON.parse(readFileSync(summaryPath, "utf8")).results;
}

const labels = { success: "通过", failed: "失败", partialFailed: "部分失败", notExecuted: "未运行" };
const summary = [
  "## 上传示例 Midscene 结果",
  "",
  ...(skipReason ? [skipReason, ""] : []),
  ...scenes.map(({ file, name }) => {
    const result = results.find(({ script }) => basename(script) === basename(file));
    const label = labels[result?.resultType] ?? "未产生结果";
    return "- " + name + " — " + label + (result?.report ? "（`" + result.report + "`）" : "");
  }),
  "",
].join("\n");
console.log(summary);
writeFileSync(join(dir, "summary.md"), summary);
if (process.env.GITHUB_STEP_SUMMARY) appendFileSync(process.env.GITHUB_STEP_SUMMARY, summary);
