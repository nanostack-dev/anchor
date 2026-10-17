import { defineConfig } from "playwright/test";

const apiURL = process.env.ANCHOR_E2E_API_URL;
if (
	!apiURL ||
	!["localhost", "127.0.0.1", "[::1]"].includes(new URL(apiURL).hostname)
) {
	throw new Error(
		"Set ANCHOR_E2E_API_URL to a local Anchor backend with a disposable database.",
	);
}

export default defineConfig({
	testDir: "./e2e",
	testMatch: "**/*.e2e.ts",
	testIgnore: ["**/smoke/**", "**/features/**"],
	workers: 1,
	use: {
		baseURL: "http://127.0.0.1:3013",
		trace: "retain-on-failure",
	},
	webServer: {
		command:
			"pnpm build && pnpm serve --port 3013 --host 127.0.0.1 --strictPort",
		url: "http://127.0.0.1:3013",
		env: { VITE_API_BASE_URL: apiURL },
	},
});
