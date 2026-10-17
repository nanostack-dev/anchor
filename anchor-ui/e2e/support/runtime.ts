import { readFileSync } from "node:fs";
import path from "node:path";

export interface LocalRuntime {
	apiURL: string;
	mailpitURL: string;
	smtpPort: number;
	runId: string;
	directory: string;
}
export function runtime(): LocalRuntime {
	const value = JSON.parse(
		readFileSync("e2e/runtime/.local/runtime.json", "utf8"),
	) as LocalRuntime;
	if (!["127.0.0.1", "localhost"].includes(new URL(value.apiURL).hostname))
		throw new Error(
			"App coverage requires its owned disposable loopback backend.",
		);
	return value;
}
export function owner() {
	return JSON.parse(
		readFileSync(path.join(runtime().directory, "owner.json"), "utf8"),
	) as { email: string; password: string };
}
