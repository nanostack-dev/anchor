export const uiHelp = `Usage: pnpm test:e2e:ui [options]

  --headed          Show the Chromium browser while selected tests run.
  --workers N       Use N parallel workers in the interactive run.
  --slow-mo N       Pace browser actions by N milliseconds in the interactive run.
  --print-config    Print the launch plan without opening ports or starting services.
  --help            Print this help without starting services.

N must be a positive safe integer. Select tests in Playwright UI.
The bootstrap prerequisite runs headless before the interactive UI opens.
`;

export function parseUIOptions(args) {
	const options = { action: "launch", headed: false };
	const seen = new Set();
	for (let index = 0; index < args.length; index++) {
		const argument = args[index];
		const separator = argument.indexOf("=");
		const name = separator === -1 ? argument : argument.slice(0, separator);
		const inline = separator === -1 ? undefined : argument.slice(separator + 1);
		if (seen.has(name)) throw new Error(`Duplicate option: ${name}`);
		seen.add(name);
		if (["--workers", "--slow-mo"].includes(name)) {
			const value = inline ?? args[++index];
			if (
				!value ||
				!/^[1-9]\d*$/.test(value) ||
				!Number.isSafeInteger(Number(value))
			)
				throw new Error(`${name} must be a positive safe integer.`);
			options[name === "--workers" ? "workers" : "slowMo"] = Number(value);
		} else if (["--help", "--print-config", "--headed"].includes(name)) {
			if (inline !== undefined)
				throw new Error(`${name} does not accept a value.`);
			if (name === "--headed") options.headed = true;
			else {
				if (options.action !== "launch")
					throw new Error("Choose either --help or --print-config.");
				options.action = name === "--help" ? "help" : "print-config";
			}
		} else throw new Error(`Unknown option: ${argument}`);
	}
	return options;
}

export function uiLaunchPlan(options) {
	const base = ["test", "--config", "playwright.app.config.ts"];
	const environment = {
		E2E_REUSE_SERVER: "1",
		E2E_EMAIL: "",
		E2E_PASSWORD: "",
	};
	return {
		frontendURL: "http://127.0.0.1:3015",
		uiURL: "http://127.0.0.1:9351",
		bootstrap: {
			args: [...base, "--project=bootstrap"],
			environment: { ...environment, E2E_SLOW_MO: "" },
		},
		ui: {
			args: [
				...base,
				"--ui",
				"--ui-host=127.0.0.1",
				"--ui-port=9351",
				...(options.headed ? ["--headed"] : []),
				...(options.workers ? [`--workers=${options.workers}`] : []),
			],
			environment: {
				...environment,
				...(options.slowMo ? { E2E_SLOW_MO: String(options.slowMo) } : {}),
			},
		},
	};
}
