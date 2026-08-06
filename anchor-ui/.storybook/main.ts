import { resolve } from "node:path";

import type { StorybookConfig } from "@storybook/react-vite";

import { OPTIMIZE_DEPS_INCLUDE } from "./optimize-deps";

const config: StorybookConfig = {
	stories: ["../src/**/*.stories.@(ts|tsx)"],
	staticDirs: ["../public"],
	addons: [
		"@storybook/addon-docs",
		"@storybook/addon-a11y",
		"@storybook/addon-vitest",
	],
	framework: {
		name: "@storybook/react-vite",
		options: {},
	},
	viteFinal: async (config) => {
		// The app's vite.config.js runs TanStackRouterVite, which regenerates
		// routeTree.gen.ts on every start. Stories never mount the real route
		// tree — they use StoryRouter — so the plugin is dropped here to keep
		// Storybook from rewriting a generated file while it watches.
		config.plugins = (config.plugins ?? []).filter(
			(plugin) =>
				!(
					plugin &&
					typeof plugin === "object" &&
					"name" in plugin &&
					String(plugin.name).includes("tanstack-router")
				),
		);

		config.resolve ??= {};
		config.resolve.alias = {
			...(Array.isArray(config.resolve.alias) ? {} : config.resolve.alias),
			"@": resolve(import.meta.dirname, "../src"),
		};

		// This is the authoritative list. `storybookTest` merges what viteFinal
		// returns into the vitest project config, and because viteFinal only sees
		// the config Storybook built, the result replaces any optimizeDeps.include
		// set in vitest.workspace.ts. Both read the same shared constant, so it
		// does not matter which one wins. See .storybook/optimize-deps.ts.
		config.optimizeDeps ??= {};
		config.optimizeDeps.include = [
			...(config.optimizeDeps.include ?? []),
			...OPTIMIZE_DEPS_INCLUDE,
		];
		config.optimizeDeps.entries = [
			"../src/**/*.stories.@(ts|tsx)",
			"./preview.ts",
		];

		return config;
	},
};

export default config;
