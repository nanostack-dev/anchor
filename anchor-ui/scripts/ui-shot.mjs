#!/usr/bin/env node
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";
import { parseArgs } from "node:util";

import { chromium } from "playwright";

const USAGE = `Usage: pnpm ui-shot <story-id | /url/path> [options]

Screenshot a Storybook story, or a path on a running dev server, once it has rendered.

Arguments
  <story-id>      Storybook story id, e.g. flows-canvas-editing--playground
  </url/path>     A path (leading slash) on the server at --base, e.g. /iframe.html?id=... or /flows

Options
  --theme <light|dark>   Theme to render (default: light)
  --width <px>           Viewport width (default: 1200)
  --height <px>          Viewport height (default: 800)
  --out <path>           Output file (default: .ui-craft/<name>-<theme>-<width>.png)
  --base <url>           Server origin (default: http://localhost:6007; the Vite dev server is http://localhost:3000)
  --storage-state <path> Local Playwright auth state for authenticated app screenshots (keep ignored)
  --actions <path>       Local module exporting async default(page) to prepare the visible state
  --full-page            Capture the whole scrollable page instead of the viewport
  --timeout <ms>         Render wait budget (default: 60000)
  -h, --help             Show this help

Start Storybook first: pnpm storybook (port 6007). Screenshots go in .ui-craft/, which is gitignored and never committed.`;

const THEMES = ["light", "dark"];
// The app uses nanostack-theme; stories that mount their own ThemeProvider use storybook-theme.
const THEME_STORAGE_KEYS = ["nanostack-theme", "storybook-theme"];
const IDLE_BUDGET_MS = 10_000;
const SMALL_PNG_BYTES = 3 * 1024;

function fail(message) {
	console.error(`ui-shot: ${message}`);
	process.exit(1);
}

function parsePositiveInt(name, value) {
	const parsed = Number(value);
	if (!Number.isInteger(parsed) || parsed <= 0) {
		fail(`--${name} must be a positive integer, got "${value}"`);
	}
	return parsed;
}

function slug(value) {
	return value
		.replace(/^\/+/, "")
		.replace(/[^a-zA-Z0-9]+/g, "-")
		.replace(/^-+|-+$/g, "")
		.toLowerCase();
}

function parseCli() {
	let parsed;
	try {
		parsed = parseArgs({
			allowPositionals: true,
			options: {
				theme: { type: "string", default: "light" },
				width: { type: "string", default: "1200" },
				height: { type: "string", default: "800" },
				out: { type: "string" },
				base: { type: "string", default: "http://localhost:6007" },
				"storage-state": { type: "string" },
				actions: { type: "string" },
				"full-page": { type: "boolean", default: false },
				timeout: { type: "string", default: "60000" },
				help: { type: "boolean", short: "h", default: false },
			},
		});
	} catch (error) {
		fail(`${error.message}\n\n${USAGE}`);
	}

	const { values, positionals } = parsed;
	if (values.help) {
		console.log(USAGE);
		process.exit(0);
	}
	if (positionals.length !== 1) {
		fail(`expected one story id or URL path\n\n${USAGE}`);
	}
	if (!THEMES.includes(values.theme)) {
		fail(`--theme must be one of ${THEMES.join(", ")}, got "${values.theme}"`);
	}

	const target = positionals[0];
	const isPath = target.startsWith("/");
	const base = values.base.replace(/\/+$/, "");
	const width = parsePositiveInt("width", values.width);
	const name = isPath ? slug(target) || "root" : target;
	const outPath = path.resolve(
		values.out ??
			path.join(".ui-craft", `${name}-${values.theme}-${width}.png`),
	);

	return {
		theme: values.theme,
		width,
		height: parsePositiveInt("height", values.height),
		timeout: parsePositiveInt("timeout", values.timeout),
		fullPage: values["full-page"],
		storageState: values["storage-state"],
		actions: values.actions,
		outPath,
		url: isPath
			? `${base}${target}`
			: `${base}/iframe.html?${new URLSearchParams({
					id: target,
					viewMode: "story",
					globals: `theme:${values.theme}`,
				})}`,
		isStorybook: !isPath || target.includes("iframe.html"),
	};
}

async function waitForRender(page, { isStorybook, timeout }) {
	const rootSelector = isStorybook ? "#storybook-root" : "#app";

	await page.waitForFunction(
		({ selector, storybook }) => {
			const root = document.querySelector(selector);
			const preparing = document.body.classList.contains("sb-preparing-story");
			const failed = document.body.classList.contains("sb-show-errordisplay");
			return (
				failed ||
				(root !== null &&
					root.childElementCount > 0 &&
					!(storybook && preparing))
			);
		},
		{ selector: rootSelector, storybook: isStorybook },
		{ timeout },
	);

	if (isStorybook) {
		const errorText = await page.evaluate(() => {
			if (!document.body.classList.contains("sb-show-errordisplay"))
				return null;
			return (
				document.querySelector("#error-message")?.textContent?.trim() ??
				"story failed to render"
			);
		});
		if (errorText !== null) {
			throw new Error(`Storybook could not render the story: ${errorText}`);
		}
	}

	try {
		await page.waitForLoadState("networkidle", { timeout: IDLE_BUDGET_MS });
	} catch {
		console.warn(
			`ui-shot: network not idle after ${IDLE_BUDGET_MS / 1000}s, shooting anyway`,
		);
	}

	await page.evaluate(async () => {
		await document.fonts.ready;
		await new Promise((resolve) =>
			requestAnimationFrame(() => requestAnimationFrame(resolve)),
		);
	});
}

async function assertTheme(page, theme) {
	// The Storybook theme decorator can apply its global after the first paint.
	await page.waitForFunction(
		(expectedDark) =>
			document.documentElement.classList.contains("dark") === expectedDark,
		theme === "dark",
		{ timeout: 5000 },
	);
	const isDark = await page.evaluate(() =>
		document.documentElement.classList.contains("dark"),
	);
	if (isDark !== (theme === "dark")) {
		throw new Error(
			`asked for the ${theme} theme but <html> is ${isDark ? "dark" : "light"}; the page did not honour the theme`,
		);
	}
}

async function main() {
	const options = parseCli();
	const browser = await chromium.launch();

	try {
		const context = await browser.newContext({
			viewport: { width: options.width, height: options.height },
			colorScheme: options.theme,
			storageState: options.storageState,
		});
		// The app's ThemeProvider reads its storage key; Storybook gets the theme through the `globals` URL param.
		await context.addInitScript(
			([keys, theme]) => {
				try {
					for (const key of keys) window.localStorage.setItem(key, theme);
				} catch {
					// storage blocked: the colorScheme emulation still covers a "system" theme
				}
			},
			[THEME_STORAGE_KEYS, options.theme],
		);

		const page = await context.newPage();
		const response = await page.goto(options.url, {
			waitUntil: "domcontentloaded",
			timeout: options.timeout,
		});
		if (response !== null && !response.ok()) {
			throw new Error(`${options.url} answered ${response.status()}`);
		}

		await waitForRender(page, options);
		if (options.actions) {
			const actions = await import(
				pathToFileURL(path.resolve(options.actions)).href
			);
			await actions.default(page);
			await waitForRender(page, options);
		}
		await assertTheme(page, options.theme);

		const png = await page.screenshot({
			fullPage: options.fullPage,
			animations: "disabled",
		});
		await mkdir(path.dirname(options.outPath), { recursive: true });
		await writeFile(options.outPath, png);

		if (png.length < SMALL_PNG_BYTES) {
			console.warn(
				`ui-shot: ${png.length} bytes is suspiciously small; the shot may be blank`,
			);
		}
		console.log(options.outPath);
	} finally {
		await browser.close();
	}
}

main().catch((error) => {
	const hint = error.message.includes("ERR_CONNECTION_REFUSED")
		? "\nIs the server running? Start Storybook with `pnpm storybook` (port 6006) or the dev server with `pnpm dev`."
		: "";
	fail(`${error.message}${hint}`);
});
