import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

const inputPath = process.env.SUB2API_SKILL_INPUT;
const outputPath = process.env.SUB2API_SKILL_OUTPUT;

const payload = JSON.parse(await readFile(inputPath, "utf8"));
const response = {
  ok: true,
  runtime: "node20",
  echo: payload.message ?? "",
  lower: String(payload.message ?? "").toLowerCase(),
};

await mkdir(dirname(outputPath), { recursive: true });
await writeFile(outputPath, JSON.stringify(response), "utf8");
console.log("node skill completed");
