import { LicenseFieldType, UsageShape } from "@/client";
import { describe, expect, it } from "vitest";
import { fieldRowsToDeclarations, newFieldRow } from "./license-schema-draft";
import { parseSchemaDsl, serializeSchemaDsl } from "./license-schema-dsl";

describe("usage shapes in schema text mode", () => {
	it("preserves shapes and constraints through visual to text to visual", () => {
		const rows = [
			newFieldRow({
				name: "max_flows",
				type: LicenseFieldType.LIMIT,
				usageShape: UsageShape.GAUGE,
				rules: { min: 1, max: 100000 },
			}),
			newFieldRow({
				name: "monthly_runs",
				type: LicenseFieldType.LIMIT,
				usageShape: UsageShape.WINDOWED_COUNTER,
				rules: { min: 0, max: 50000000 },
			}),
			newFieldRow({
				name: "timeout",
				type: LicenseFieldType.NUMBER,
				rules: { min: 60 },
			}),
		];
		const source = serializeSchemaDsl(rows);
		expect(source).toContain("limit gauge 1..100000");
		expect(source).toContain("limit windowed_counter 0..50000000");
		const parsed = parseSchemaDsl(source.replace("max_flows:", "max_flows2:"));
		expect(parsed.errors).toEqual([]);
		rows[0].name = "max_flows2";
		expect(fieldRowsToDeclarations(parsed.rows)).toEqual(
			fieldRowsToDeclarations(rows),
		);
	});

	it.each([
		"max_flows: limit 0..100",
		"monthly_runs: limit counter 0..100",
		"max_flows: limit",
	])("rejects missing or unknown shapes: %s", (source) => {
		expect(parseSchemaDsl(source).errors).toEqual([
			{
				line: 1,
				message: "A limit needs `gauge` or `windowed_counter` after its type.",
			},
		]);
	});
});
