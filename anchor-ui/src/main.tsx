import { StrictMode } from "react";
import ReactDOM from "react-dom/client";

// Import OpenAPI config
import "./styles.css";
import type { router } from "@/routeTree";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "sonner";
import App from "./App";
import { client } from "./client/client.gen";
import reportWebVitals from "./reportWebVitals";

function extractBaseUrl(): string | null {
	const params = new URLSearchParams(window.location.search);
	// Check top-level query param first
	const direct = params.get("baseUrl");
	if (direct) return direct;
	// Check nested inside redirect param (e.g. /login?redirect=%2F%3FbaseUrl%3D...)
	const redirect = params.get("redirect");
	if (redirect) {
		const decoded = decodeURIComponent(redirect);
		try {
			const redirectUrl = new URL(decoded, window.location.origin);
			const nested = redirectUrl.searchParams.get("baseUrl");
			if (nested) return nested;
		} catch {
			// decoded might be a path, not a full URL
			const redirectParams = new URLSearchParams(decoded.split("?")[1] || "");
			const nested = redirectParams.get("baseUrl");
			if (nested) return nested;
		}
	}
	return null;
}

const baseURL = extractBaseUrl() || import.meta.env.VITE_API_BASE_URL || "";
if (!baseURL && import.meta.env.DEV) {
	console.warn(
		"VITE_API_BASE_URL environment variable is not set. API calls might fail.",
	);
}

client.setConfig({
	baseUrl: baseURL,
});

declare module "@tanstack/react-router" {
	interface Register {
		router: typeof router;
	}
}

const queryClient = new QueryClient();

const rootElement = document.getElementById("app");
if (rootElement && !rootElement.innerHTML) {
	const root = ReactDOM.createRoot(rootElement);
	root.render(
		<StrictMode>
			<QueryClientProvider client={queryClient}>
				<App />
				<Toaster />
			</QueryClientProvider>
		</StrictMode>,
	);
}

reportWebVitals();
