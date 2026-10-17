import { Button } from "@nanostackorg/design-system/components/button";
import { Label } from "@nanostackorg/design-system/components/label";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { CodeIcon as Code2, RowsIcon as Rows3 } from "@phosphor-icons/react";
import { useLayoutEffect, useRef, useState } from "react";

import { LicenseSchemaFieldsEditor } from "./LicenseSchemaFieldsEditor";
import { LicenseSchemaTextEditor } from "./LicenseSchemaTextEditor";
import type { FieldRow } from "./license-schema-draft";
import { serializeSchemaDsl } from "./license-schema-dsl";

export type SchemaEditorMode = "visual" | "text";

const MODES: { id: SchemaEditorMode; label: string; icon: typeof Rows3 }[] = [
	{ id: "visual", label: "Visual", icon: Rows3 },
	{ id: "text", label: "Text", icon: Code2 },
];

interface LicenseSchemaEditorProps {
	description: string;
	onDescriptionChange: (description: string) => void;
	fields: FieldRow[];
	onFieldsChange: (fields: FieldRow[]) => void;
	errors?: Record<string, string>;
	/** Raised when the text mode holds source the parser cannot read. */
	onSourceInvalidChange?: (invalid: boolean) => void;
	disabled?: boolean;
	defaultMode?: SchemaEditorMode;
}

/**
 * The license schema editor body: the same draft, in whichever representation
 * suits the operator.
 *
 * Visual mode teaches the grammar — the type list and each type's rules are
 * discoverable without documentation. Text mode is faster once the grammar is
 * known, which for this surface is after the first schema. Both write the same
 * `FieldRow[]`, so switching mid-draft loses nothing.
 */
export function LicenseSchemaEditor({
	description,
	onDescriptionChange,
	fields,
	onFieldsChange,
	errors,
	onSourceInvalidChange,
	disabled,
	defaultMode = "visual",
}: LicenseSchemaEditorProps) {
	const [mode, setMode] = useState<SchemaEditorMode>(defaultMode);
	const [source, setSource] = useState(() => serializeSchemaDsl(fields));
	const activeModeButtonRef = useRef<HTMLButtonElement>(null);
	const renderedMode = useRef(mode);

	// Each mode renders its own copy of the switch, so the button that was
	// pressed unmounts. Handing focus to its replacement keeps a keyboard
	// operator on the switch instead of dropping them onto the dialog.
	useLayoutEffect(() => {
		if (renderedMode.current === mode) return;
		renderedMode.current = mode;
		activeModeButtonRef.current?.focus();
	}, [mode]);

	// The draft is the source of truth. Entering text mode renders it; leaving
	// keeps whatever the last successful parse produced, which the text editor
	// has already pushed up.
	const switchMode = (next: SchemaEditorMode) => {
		if (next === mode) return;
		if (next === "text") setSource(serializeSchemaDsl(fields));
		setMode(next);
	};

	const modeSwitch = (
		<Box
			as="fieldset"
			aria-label="Editor mode"
			className="flex items-center gap-0.5 rounded-lg bg-muted p-0.5"
		>
			{MODES.map(({ id, label, icon: Icon }) => (
				<Button
					icon={Icon}
					key={id}
					ref={mode === id ? activeModeButtonRef : undefined}
					type="button"
					variant={mode === id ? "soft" : "ghost"}
					size="xs"
					aria-pressed={mode === id}
					onClick={() => switchMode(id)}
				>
					{label}
				</Button>
			))}
		</Box>
	);

	return (
		<Stack space="lg">
			<Stack space="xs">
				<Label htmlFor="schema-description">Schema description</Label>
				<Textarea
					id="schema-description"
					value={description}
					onChange={(e) => onDescriptionChange(e.target.value)}
					placeholder="What this product's license schema is for (optional)"
					rows={2}
					disabled={disabled}
				/>
			</Stack>

			{mode === "visual" ? (
				<LicenseSchemaFieldsEditor
					fields={fields}
					onChange={onFieldsChange}
					errors={errors}
					disabled={disabled}
					headerAction={modeSwitch}
				/>
			) : (
				<LicenseSchemaTextEditor
					value={source}
					onValueChange={setSource}
					headerAction={modeSwitch}
					onParsed={(rows, hasErrors) => {
						onFieldsChange(rows);
						onSourceInvalidChange?.(hasErrors);
					}}
					disabled={disabled}
				/>
			)}
		</Stack>
	);
}
