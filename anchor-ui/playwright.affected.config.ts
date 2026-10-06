import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { defineConfig } from "playwright/test";
import { z } from "zod";
import base from "./playwright.app.config";

const planPath = process.env.E2E_SELECTION_FILE;
if (!planPath)
	throw new Error(
		"Use test:e2e:affected or test:e2e:verify to create a current selection.",
	);
const plan = z
	.object({
		mode: z.enum(["full", "selected", "none"]),
		testPaths: z.array(z.string()),
		allTestPaths: z.array(z.string()),
	})
	.parse(JSON.parse(readFileSync(planPath, "utf8")));
const repositoryRoot = path.resolve(import.meta.dirname, "..");
function discover(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const file = path.join(directory, entry.name);
		return entry.isDirectory()
			? discover(file)
			: entry.isFile() && entry.name.endsWith(".e2e.ts")
				? [path.relative(repositoryRoot, file).split(path.sep).join("/")]
				: [];
	});
}
const inventory = discover(
	path.resolve(import.meta.dirname, "e2e/features"),
).sort();
if (
	JSON.stringify(inventory) !== JSON.stringify(plan.allTestPaths) ||
	new Set(plan.testPaths).size !== plan.testPaths.length ||
	plan.testPaths.some((file) => !inventory.includes(file))
)
	throw new Error(
		"Selection does not match the current feature inventory; regenerate the plan.",
	);
if (plan.mode !== "selected")
	throw new Error(
		"Only selected plans use this configuration; full runs use playwright.app.config.ts.",
	);
const bootstrap = base.projects?.find(
	(project) => project.name === "bootstrap",
);
const chromium = base.projects?.find((project) => project.name === "chromium");
if (!bootstrap || !chromium)
	throw new Error(
		"The app configuration must define bootstrap and chromium projects.",
	);
const bootstrapPath = "anchor-ui/e2e/features/auth/bootstrap.e2e.ts";
if (!plan.testPaths.includes(bootstrapPath))
	throw new Error("A selection must include bootstrap.");
const selected = plan.testPaths.filter((file) => file !== bootstrapPath);
const remaining = inventory.filter((file) => !plan.testPaths.includes(file));
if (!selected.length)
	throw new Error("A selection must contain feature scenarios.");
const exactFiles = (files: string[]) =>
	files.map(
		(file) =>
			new RegExp(
				`^${path.resolve(repositoryRoot, file).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}$`,
			),
	);
const affected = {
	...chromium,
	name: "chromium-affected",
	testMatch: exactFiles(selected),
	dependencies: ["bootstrap"],
};
const projects = [bootstrap, affected];
if (process.env.E2E_COMPLETE === "1" && remaining.length)
	projects.push({
		...chromium,
		name: "chromium-remaining",
		testMatch: exactFiles(remaining),
		dependencies: [affected.name],
	});

export default defineConfig({ ...base, projects });
