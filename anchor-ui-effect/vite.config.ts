import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";

const apiTarget = process.env.ANCHOR_API_URL ?? "http://127.0.0.1:8080";

export default defineConfig({
	plugins: [tailwindcss()],
	server: {
		proxy: {
			"/health": { target: apiTarget, changeOrigin: true },
			"/v1": { target: apiTarget, changeOrigin: true, cookieDomainRewrite: "" },
		},
	},
	preview: {
		proxy: {
			"/health": { target: apiTarget, changeOrigin: true },
			"/v1": { target: apiTarget, changeOrigin: true, cookieDomainRewrite: "" },
		},
	},
});
