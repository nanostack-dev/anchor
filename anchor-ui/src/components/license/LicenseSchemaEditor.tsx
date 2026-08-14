import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";
import { Code2, Rows3 } from "lucide-react";
import { useState } from "react";

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

	// The draft is the source of truth. Entering text mode renders it; leaving
	// keeps whatever the last successful parse produced, which the text editor
	// has already pushed up.
	const switchMode = (next: SchemaEditorMode) => {
		if (next === mode) return;
		if (next === "text") setSource(serializeSchemaDsl(fields));
		setMode(next);
	};

	const modeSwitch = (
		<fieldset
			aria-label="Editor mode"
			className="flex items-center gap-0.5 rounded-lg bg-muted p-0.5"
		>
			{MODES.map(({ id, label, icon: Icon }) => (
				<Button
					key={id}
					type="button"
					variant="ghost"
					size="xs"
					aria-pressed={mode === id}
					onClick={() => switchMode(id)}
					className={cn(
						"gap-1.5 text-muted-foreground hover:bg-transparent",
						mode === id &&
							"bg-background text-foreground shadow-sm hover:bg-background",
					)}
				>
					<Icon />
					{label}
				</Button>
			))}
		</fieldset>
	);

	return (
		<div className="flex flex-col gap-4">
			<div className="space-y-1.5">
				<Label htmlFor="schema-description">Schema description</Label>
				<Textarea
					id="schema-description"
					value={description}
					onChange={(e) => onDescriptionChange(e.target.value)}
					placeholder="What this product's license schema is for (optional)"
					rows={2}
					disabled={disabled}
				/>
			</div>

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
		</div>
	);
}
