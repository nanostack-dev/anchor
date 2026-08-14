import { Label } from "@/components/ui/label";
import { cn } from "@/lib/utils";
import { Check, TriangleAlert } from "lucide-react";
import { type ReactNode, useId, useMemo, useRef } from "react";

import type { FieldRow } from "./license-schema-draft";
import { parseSchemaDsl } from "./license-schema-dsl";

const MIN_ROWS = 8;

export const SCHEMA_DSL_PLACEHOLDER = `max_flows:   limit 0..100                    # Concurrent flows allowed
seats:       number 1..
sso:         boolean
tier:        enum free | pro | enterprise
webhook_url: string /^https:\\/\\// len 1..2048`;

interface LicenseSchemaTextEditorProps {
	value: string;
	onValueChange: (source: string) => void;
	/** Fires on every parse, so the parent can hold the draft the form submits. */
	onParsed?: (rows: FieldRow[], hasErrors: boolean) => void;
	disabled?: boolean;
	/** Rendered at the right of the header row. */
	headerAction?: ReactNode;
}

/**
 * Authors a license schema as text, one field per line.
 *
 * A schema is a declaration, and the operator declaring it is the same person
 * who reads the product's API contract. Typing five lines is faster than
 * filling five stacked forms, and the whole schema stays on screen while it is
 * written. Every parse error names its line, and clicking it puts the caret
 * there.
 */
export function LicenseSchemaTextEditor({
	value,
	onValueChange,
	onParsed,
	disabled,
	headerAction,
}: LicenseSchemaTextEditorProps) {
	const editorId = useId();
	const textareaRef = useRef<HTMLTextAreaElement>(null);

	const { rows, errors } = useMemo(() => parseSchemaDsl(value), [value]);
	const lines = value.split("\n");

	// The parent needs the draft, not just the text. Reporting it during render
	// would set state in another component mid-render, so it is deferred to the
	// change that produced it.
	const emit = (next: string) => {
		onValueChange(next);
		if (onParsed) {
			const parsed = parseSchemaDsl(next);
			onParsed(parsed.rows, parsed.errors.length > 0);
		}
	};

	const focusLine = (line: number) => {
		const textarea = textareaRef.current;
		if (!textarea) return;
		const offset = lines
			.slice(0, line - 1)
			.reduce((total, text) => total + text.length + 1, 0);
		textarea.focus();
		textarea.setSelectionRange(offset, offset + (lines[line - 1]?.length ?? 0));
	};

	const errorLines = new Set(errors.map((error) => error.line));

	return (
		<div className="flex flex-col gap-2">
			<div className="flex items-center justify-between gap-3">
				<Label htmlFor={editorId}>Fields</Label>
				{headerAction}
			</div>

			<div
				className={cn(
					"flex overflow-hidden rounded-lg border border-border bg-card font-mono text-sm transition-[border-color,box-shadow] focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50",
					errors.length > 0 && "border-destructive/50",
				)}
			>
				<div
					aria-hidden="true"
					className="shrink-0 select-none border-r border-border bg-muted/40 py-2.5 text-right font-mono leading-6"
				>
					{Array.from(
						{ length: Math.max(lines.length, MIN_ROWS) },
						(_, index) => (
							<div
								key={`line-${index + 1}`}
								className={cn(
									"px-2.5 tabular-nums",
									errorLines.has(index + 1)
										? "font-medium text-destructive"
										: "text-muted-foreground/60",
								)}
							>
								{index + 1}
							</div>
						),
					)}
				</div>

				<textarea
					id={editorId}
					ref={textareaRef}
					value={value}
					onChange={(e) => emit(e.target.value)}
					// Tab belongs to the form's focus order here; there is no nesting in
					// the grammar, so it never needs to insert one.
					wrap="off"
					spellCheck={false}
					autoCapitalize="off"
					autoCorrect="off"
					rows={Math.max(lines.length, MIN_ROWS)}
					placeholder={SCHEMA_DSL_PLACEHOLDER}
					disabled={disabled}
					aria-invalid={errors.length > 0}
					className="w-full resize-none overflow-x-auto bg-transparent px-3 py-2.5 font-mono text-sm leading-6 outline-none placeholder:text-muted-foreground/50 disabled:opacity-50"
				/>
			</div>

			{errors.length > 0 ? (
				<ul className="flex flex-col gap-1">
					{errors.map((error) => (
						<li key={`${error.line}-${error.message}`}>
							<button
								type="button"
								onClick={() => focusLine(error.line)}
								className="flex w-full items-start gap-2 rounded-md px-1 py-0.5 text-left text-sm text-destructive outline-none transition-colors hover:bg-destructive/10 focus-visible:ring-3 focus-visible:ring-destructive/20"
							>
								<TriangleAlert className="mt-0.5 size-3.5 shrink-0" />
								<span className="font-mono text-xs tabular-nums">
									{error.line}
								</span>
								<span className="min-w-0 flex-1">{error.message}</span>
							</button>
						</li>
					))}
				</ul>
			) : (
				<div className="flex items-center justify-between gap-3 px-1">
					<p className="flex items-center gap-1.5 text-sm text-muted-foreground">
						<Check className="size-3.5 text-success" />
						{rows.length === 0
							? "Nothing declared yet."
							: `${rows.length} ${rows.length === 1 ? "field" : "fields"} parsed.`}
					</p>
					<code className="text-xs text-muted-foreground">
						name: type [rules] # description
					</code>
				</div>
			)}
		</div>
	);
}
