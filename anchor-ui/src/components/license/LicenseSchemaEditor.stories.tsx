import { LicenseFieldType } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import {
	LicenseSchemaEditor,
	type SchemaEditorMode,
} from "./LicenseSchemaEditor";
import {
	type FieldRow,
	newFieldRow,
	validateFieldRows,
} from "./license-schema-draft";

/** A schema with enough fields that the density difference is the point. */
function seedFields(): FieldRow[] {
	return [
		newFieldRow({
			name: "max_flows",
			type: LicenseFieldType.LIMIT,
			description: "Concurrent flows an organization may run",
			rules: { min: 0, max: 500 },
		}),
		newFieldRow({
			name: "max_collections",
			type: LicenseFieldType.LIMIT,
			rules: { min: 0 },
		}),
		newFieldRow({
			name: "seats",
			type: LicenseFieldType.NUMBER,
			description: "Billable members",
			rules: { min: 1, max: 100 },
		}),
		newFieldRow({ name: "sso", type: LicenseFieldType.BOOLEAN }),
		newFieldRow({
			name: "tier",
			type: LicenseFieldType.ENUM,
			rules: { values: ["free", "pro", "enterprise"] },
		}),
		newFieldRow({
			name: "webhook_url",
			type: LicenseFieldType.STRING,
			rules: { pattern: "^https://", min_length: 1, max_length: 2048 },
		}),
	];
}

/**
 * The editor is controlled, so the story owns the draft. `Validate` mirrors what
 * the submitting dialog does: a client-side pass that keys errors by row.
 */
function EditorHarness({
	initialFields,
	defaultMode,
}: {
	initialFields: FieldRow[];
	defaultMode?: SchemaEditorMode;
}) {
	const [description, setDescription] = useState(
		"What an Echopoint organization license may carry.",
	);
	const [fields, setFields] = useState(initialFields);
	const [errors, setErrors] = useState<Record<string, string>>({});
	const [sourceInvalid, setSourceInvalid] = useState(false);

	return (
		<div className="mx-auto flex max-w-[680px] flex-col gap-4 p-6">
			<LicenseSchemaEditor
				description={description}
				onDescriptionChange={setDescription}
				fields={fields}
				onFieldsChange={(next) => {
					setFields(next);
					setErrors({});
				}}
				errors={errors}
				onSourceInvalidChange={setSourceInvalid}
				defaultMode={defaultMode}
			/>
			<div className="flex justify-end gap-2 border-t border-border pt-4">
				<button
					type="button"
					disabled={sourceInvalid}
					onClick={() => setErrors(validateFieldRows(fields))}
					className="h-8 rounded-lg bg-primary px-3 text-sm font-medium text-primary-foreground disabled:opacity-50"
				>
					Create Schema
				</button>
			</div>
		</div>
	);
}

const meta = {
	title: "License/LicenseSchemaEditor",
	component: EditorHarness,
	parameters: { layout: "fullscreen" },
	args: { initialFields: [] },
} satisfies Meta<typeof EditorHarness>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * Six declared fields, all readable at once. The equivalent stack of expanded
 * per-field forms runs past 1200px, so the schema being written is never
 * visible as a whole.
 */
export const Visual: Story = {
	args: { initialFields: seedFields() },
};

/**
 * The same schema as source. Five to six lines replace six expanded forms, and
 * the operator who already knows the grammar never leaves the keyboard.
 */
export const Text: Story = {
	args: { initialFields: seedFields(), defaultMode: "text" },
};

/** A first-time schema opens on one blank field, ready to type into. */
export const Empty: Story = {
	args: { initialFields: [newFieldRow()] },
};

/**
 * Enter in a name commits the row and opens the next one. Declaring a schema is
 * type-name, Enter, type-name, Enter — the mouse is only needed to change a
 * type away from the default.
 */
export const EnterAddsTheNextField: Story = {
	args: { initialFields: [newFieldRow()] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const first = canvas.getByPlaceholderText("max_flows");
		await userEvent.type(first, "max_flows{Enter}");
		await userEvent.keyboard("seats{Enter}");
		await userEvent.keyboard("sso");

		// Three declared rows, and only the one still being typed into is open.
		await expect(canvas.getByText("3 fields")).toBeInTheDocument();
		await waitFor(() => {
			const open = canvas
				.getAllByRole("button", { expanded: true })
				.map((row) => row.textContent);
			expect(open).toHaveLength(1);
			expect(open[0]).toContain("sso");
		});
		await expect(
			canvas.getByRole("button", { name: /^max_flows/ }),
		).toHaveAttribute("aria-expanded", "false");
	},
};

/**
 * Opening one field folds the previous one away. The list cannot grow into the
 * scrolling stack of forms this replaces.
 */
export const OnlyOneFieldIsOpenAtATime: Story = {
	args: { initialFields: seedFields() },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const maxFlows = canvas.getByRole("button", { name: /^max_flows/ });
		const seats = canvas.getByRole("button", { name: /^seats/ });

		await userEvent.click(maxFlows);
		await expect(maxFlows).toHaveAttribute("aria-expanded", "true");

		await userEvent.click(seats);
		await expect(seats).toHaveAttribute("aria-expanded", "true");
		await expect(maxFlows).toHaveAttribute("aria-expanded", "false");
	},
};

/**
 * A failed submit reopens the offending field. An error against a row that is
 * folded shut is an error the operator cannot act on.
 */
export const ValidationReopensTheOffendingField: Story = {
	args: {
		initialFields: [
			newFieldRow({ name: "max_flows", type: LicenseFieldType.LIMIT }),
			newFieldRow({ name: "", type: LicenseFieldType.STRING }),
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Create Schema" }),
		);

		await expect(canvas.getByText("Name is required.")).toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: /^New field/ }),
		).toHaveAttribute("aria-expanded", "true");
	},
};

/**
 * Text mode reports each parse failure against its line, and clicking the
 * failure selects that line. A bad type keyword never reaches the API.
 */
export const TextModeReportsErrorsByLine: Story = {
	args: { initialFields: [], defaultMode: "text" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const editor = canvas.getByLabelText("Fields");
		await userEvent.click(editor);
		await userEvent.paste("max_flows: limit 0..100\nseats: integer 1..10");

		await expect(
			canvas.getByText(/`integer` is not a field type/),
		).toBeInTheDocument();
		// Only line 2 failed — errors are per line, not per document.
		await expect(canvas.getAllByRole("listitem")).toHaveLength(1);
	},
};

/**
 * A draft written in one mode is readable in the other. The two views are
 * representations of the same `FieldRow[]`, not two editors that have to be
 * kept in sync by hand.
 */
export const DraftSurvivesAModeSwitch: Story = {
	args: { initialFields: seedFields() },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByRole("button", { name: "Text" }));

		const editor = canvas.getByLabelText<HTMLTextAreaElement>("Fields");
		await expect(editor.value).toMatch(
			/^max_flows: +limit 0\.\.500 +# Concurrent/m,
		);
		await expect(editor.value).toMatch(
			/^tier: +enum free \| pro \| enterprise$/m,
		);
		await expect(editor.value).toMatch(/^sso: +boolean$/m);
		await expect(editor.value).toMatch(
			/^webhook_url: +string \/\^https:\\\/\\\/\/ len 1\.\.2048$/m,
		);

		await userEvent.click(canvas.getByRole("button", { name: "Visual" }));
		await expect(canvas.getByText("6 fields")).toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: /^webhook_url/ }),
		).toBeInTheDocument();
	},
};
