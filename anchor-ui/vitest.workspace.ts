import { availableParallelism } from "node:os";
import { resolve } from "node:path";

import { storybookTest } from "@storybook/addon-vitest/vitest-plugin";
import { playwright } from "@vitest/browser-playwright";
import { defineConfig, defineProject } from "vitest/config";

import { OPTIMIZE_DEPS_INCLUDE } from "./.storybook/optimize-deps";

/**
 * How many browser tabs run story files at once.
 *
 * Vitest's browser pool derives its own default as `Math.min(12, numCpus - 1)`,
 * which is 1 on a 2-vCPU hosted runner — every story file then pays a fresh page
 * load serially. echopoint measured 180s -> ~100s going from 1 to 2 workers on
 * two cores, and 3 workers being *worse* than 2 (the tabs fight the Vite server
 * for the same cores). The cap of 4 is for bigger hosts and stays well under the
 * pool's own 12: vitest#7871 has the main thread choking above ~12 tabs, which
 * surfaces as intermittent `userEvent` timeouts.
 *
 * Not settable from the CLI — `maxWorkers` is absent from Vitest's cliOverrides
 * list, so `vitest --maxWorkers=N` is silently ignored. It has to live here.
 */
const storyTestWorkers = Math.min(4, Math.max(2, availableParallelism() - 1));

export default defineConfig({
	resolve: {
		alias: {
			"@": resolve(import.meta.dirname, "./src"),
		},
	},
	optimizeDeps: {
		// Shared with .storybook/main.ts — see .storybook/optimize-deps.ts for why
		// this must be identical in both places.
		include: OPTIMIZE_DEPS_INCLUDE,
		entries: ["src/**/*.stories.@(ts|tsx)", ".storybook/preview.ts"],
	},
	test: {
		projects: [
			defineProject({
				plugins: [
					storybookTest({
						configDir: resolve(import.meta.dirname, ".storybook"),
						storybookScript: "pnpm storybook --ci",
					}),
				],
				test: {
					name: "storybook",
					maxWorkers: storyTestWorkers,
					browser: {
						enabled: true,
						provider: playwright({}),
						// Load-bearing for the above: the browser pool forces a single
						// worker when `headless` is false, whatever `maxWorkers` says.
						headless: true,
						instances: [{ browser: "chromium" }],
					},
				},
			}),
		],
	},
});
