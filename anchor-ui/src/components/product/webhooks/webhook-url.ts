/**
 * Hostnames that are obviously a developer's own machine. `http://` is
 * tolerated for these because a local receiver has no certificate, but Anchor
 * only accepts them when the deployment explicitly allows insecure targets
 * (development and tests) — hence a warning rather than silent approval.
 */
const DEV_HOSTNAMES = new Set([
	"localhost",
	"127.0.0.1",
	"0.0.0.0",
	"[::1]",
	"::1",
]);

const isDevHostname = (hostname: string): boolean => {
	const normalized = hostname.toLowerCase();
	return (
		DEV_HOSTNAMES.has(normalized) ||
		normalized.endsWith(".localhost") ||
		normalized.endsWith(".local")
	);
};

export interface WebhookUrlVerdict {
	/** Blocking problem: the form must not submit. */
	error?: string;
	/** Non-blocking caveat worth surfacing next to the field. */
	warning?: string;
}

/**
 * Mirrors `webhook.ValidateTargetURL` on the server: an absolute URL with a
 * host, HTTPS unless the target is obviously a development host. This is UX,
 * not security — the real control is the post-resolution IP check at send
 * time, which no client-side check can stand in for.
 */
export function classifyWebhookUrl(raw: string): WebhookUrlVerdict {
	const trimmed = raw.trim();
	if (!trimmed) {
		return { error: "URL is required" };
	}

	let parsed: URL;
	try {
		parsed = new URL(trimmed);
	} catch {
		return {
			error:
				"Enter a full URL including the scheme, e.g. https://example.com/hooks",
		};
	}

	if (!parsed.hostname) {
		return { error: "URL must include a host" };
	}

	if (parsed.protocol === "https:") {
		return {};
	}

	if (parsed.protocol === "http:") {
		if (isDevHostname(parsed.hostname)) {
			return {
				warning:
					"Plain http is only accepted by deployments that allow insecure targets (development). Production Anchor refuses http and never dials loopback or private addresses.",
			};
		}
		return { error: "Webhook URL must use https" };
	}

	return {
		error: `URL scheme "${parsed.protocol.replace(":", "")}" is not supported`,
	};
}
