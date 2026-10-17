import { preview } from "vite";

const target = process.env.E2E_API_URL || "https://apidev.tryanchor.dev";

await preview({
	preview: {
		host: "127.0.0.1",
		port: 3014,
		strictPort: true,
		proxy: {
			"/health": { target, changeOrigin: true },
			"/v1": {
				target,
				changeOrigin: true,
				cookieDomainRewrite: "",
			},
		},
	},
});
