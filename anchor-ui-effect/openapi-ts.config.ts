import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
	input: "../apps/anchor/cmd/http/openapi.yaml",
	output: "src/client",
	plugins: [{ name: "@hey-api/typescript", enums: "javascript" }],
});
