import { createHash } from "node:crypto";
import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(
	path.dirname(fileURLToPath(import.meta.url)),
	"../..",
);
export function frontendFingerprint() {
	const digest = createHash("sha256");
	function read(directory: string) {
		for (const entry of readdirSync(directory, { withFileTypes: true }).sort(
			(a, b) => a.name.localeCompare(b.name),
		)) {
			const file = path.join(directory, entry.name);
			if (entry.isDirectory()) read(file);
			else if (!/\.(stories|test)\.[jt]sx?$/.test(file))
				digest.update(file.slice(root.length)).update(readFileSync(file));
		}
	}
	read(path.join(root, "src"));
	read(path.join(root, "public"));
	for (const file of ["vite.config.js", "index.html", "../pnpm-lock.yaml"])
		digest.update(readFileSync(path.join(root, file)));
	return digest.digest("hex");
}
