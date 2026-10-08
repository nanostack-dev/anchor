import tailwindcss from "@tailwindcss/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig } from "vite";

import { resolve } from "node:path";
import { TanStackRouterVite } from "@tanstack/router-plugin/vite";

const apiProxyTarget = process.env.ANCHOR_API_PROXY_TARGET;
const apiProxyOrigin = process.env.ANCHOR_API_PROXY_ORIGIN;

function loopbackURL(value) {
	const url = new URL(value);
	if (
		url.protocol !== "http:" ||
		url.hostname !== "127.0.0.1" ||
		url.username ||
		url.password ||
		url.pathname !== "/" ||
		url.search ||
		url.hash
	)
		throw new Error("The local Anchor API proxy requires a loopback origin.");
	return url.origin;
}

const proxy = apiProxyTarget
	? Object.fromEntries(
			["/health", "/v1"].map((path) => [
				path,
				{
					target: loopbackURL(apiProxyTarget),
					changeOrigin: true,
					cookieDomainRewrite: "",
					...(apiProxyOrigin
						? { headers: { Origin: loopbackURL(apiProxyOrigin) } }
						: {}),
				},
			]),
		)
	: undefined;

// https://vitejs.dev/config/
export default defineConfig({
	server: {
		proxy,
		watch: { ignored: ["**/.ui-craft/**", "**/.local*/**"] },
	},
	plugins: [
		TanStackRouterVite({
			autoCodeSplitting: true,
			routeFileIgnorePattern: "routePaths.ts",
		}),
		viteReact(),
		tailwindcss(),
	],
	test: {
		globals: true,
		environment: "jsdom",
	},
	resolve: {
		dedupe: ["react", "react-dom"],
		alias: {
			"@": resolve(__dirname, "./src"),
		},
	},
});
