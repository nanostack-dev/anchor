export function e2eFrontend(value = process.env.E2E_FRONTEND_PORT) {
	const configured = value ?? "3015";
	const port = Number(configured);
	if (
		!/^\d+$/.test(configured) ||
		!Number.isInteger(port) ||
		port < 1 ||
		port > 65535
	)
		throw new Error(
			"E2E_FRONTEND_PORT must be a whole port between 1 and 65535",
		);
	return { port, url: `http://127.0.0.1:${port}` };
}
