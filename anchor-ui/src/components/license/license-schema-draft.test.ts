import {
	LicenseFieldType,
	type LicenseSchemaResponse,
	UsageShape,
} from "@/client";
import { describe, expect, it } from "vitest";
import {
	fieldRowsFromSchema,
	fieldRowsToDeclarations,
	newFieldRow,
	validateFieldRows,
} from "./license-schema-draft";

const FIELD_DEFAULTS = {
	rules: {},
	created_at: "2026-09-21T10:00:00Z",
	updated_at: "2026-09-21T10:00:00Z",
};

describe("license schema usage shapes", () => {
	it("preserves every limit's usage shape when renaming a field", () => {
		const schema: LicenseSchemaResponse = {
			id: "lsc_test",
			product_id: "prd_test",
			created_at: "2026-09-21T10:00:00Z",
			updated_at: "2026-09-21T10:00:00Z",
			fields: [
				{
					...FIELD_DEFAULTS,
					id: "lfd_flows",
					name: "max_flows",
					type: LicenseFieldType.LIMIT,
					usage_shape: UsageShape.GAUGE,
				},
				{
					...FIELD_DEFAULTS,
					id: "lfd_runs",
					name: "monthly_runs",
					type: LicenseFieldType.LIMIT,
					usage_shape: UsageShape.WINDOWED_COUNTER,
				},
			],
		};
		const rows = fieldRowsFromSchema(schema);
		rows[0].name = "max_flows2";
		expect(fieldRowsToDeclarations(rows)).toEqual([
			expect.objectContaining({
				name: "max_flows2",
				usage_shape: UsageShape.GAUGE,
			}),
			expect.objectContaining({
				name: "monthly_runs",
				usage_shape: UsageShape.WINDOWED_COUNTER,
			}),
		]);
	});

	it("requires an explicit usage shape for a limit", () => {
		const row = newFieldRow({
			name: "max_flows2",
			type: LicenseFieldType.LIMIT,
		});
		expect(validateFieldRows([row])[row.uiKey]).toBe(
			"Choose a usage shape for this limit.",
		);
	});

	it.each(Object.values(UsageShape))(
		"accepts the declared shape %s",
		(usageShape) => {
			const row = newFieldRow({
				name: "limit",
				type: LicenseFieldType.LIMIT,
				usageShape,
			});
			expect(validateFieldRows([row])).toEqual({});
		},
	);

	it("omits a stale usage shape when the field is no longer a limit", () => {
		const row = newFieldRow({
			name: "seats",
			type: LicenseFieldType.NUMBER,
			usageShape: UsageShape.GAUGE,
		});
		expect(fieldRowsToDeclarations([row])[0].usage_shape).toBeUndefined();
		expect(validateFieldRows([row])).toEqual({});
	});
});
