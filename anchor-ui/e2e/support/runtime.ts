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
	const namespace = process.env.ANCHOR_E2E_RUNTIME_NAMESPACE ?? "";
	if (namespace && !/^[a-z][a-z0-9-]{0,31}$/.test(namespace))
		throw new Error(
			"ANCHOR_E2E_RUNTIME_NAMESPACE must be a short lowercase identifier.",
		);
	const metadataPath = namespace
		? path.join("e2e/runtime/.local/namespaces", namespace, "runtime.json")
		: "e2e/runtime/.local/runtime.json";
	const value = JSON.parse(readFileSync(metadataPath, "utf8")) as LocalRuntime;
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
