import { Button } from "@nanostackorg/design-system/components/button";
import { Label } from "@nanostackorg/design-system/components/label";
import { Text } from "@nanostackorg/design-system/components/text";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	CheckIcon as Check,
	WarningIcon as TriangleAlert,
} from "@phosphor-icons/react";
import { type ReactNode, useId, useMemo, useRef } from "react";

import type { FieldRow } from "./license-schema-draft";
import { parseSchemaDsl } from "./license-schema-dsl";

const MIN_ROWS = 8;

export const SCHEMA_DSL_PLACEHOLDER = `max_flows:   limit gauge 0..100              # Concurrent flows allowed
monthly_runs: limit windowed_counter 0..5000
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

	return (
		<Stack space="sm">
			<Spread space="md">
				<Label htmlFor={editorId}>Fields</Label>
				{headerAction}
			</Spread>

			<Textarea
				id={editorId}
				ref={textareaRef}
				value={value}
				onChange={(e) => emit(e.target.value)}
				font="mono"
				wrap="off"
				spellCheck={false}
				autoCapitalize="off"
				autoCorrect="off"
				rows={Math.max(lines.length, MIN_ROWS)}
				placeholder={SCHEMA_DSL_PLACEHOLDER}
				disabled={disabled}
				aria-invalid={errors.length > 0}
			/>

			{errors.length > 0 ? (
				<Stack space="xs" as="ul">
					{errors.map((error) => (
						<li key={`${error.line}-${error.message}`}>
							<Button
								width="fill"
								variant="ghost"
								tone="critical"
								size="sm"
								icon={TriangleAlert}
								type="button"
								onClick={() => focusLine(error.line)}
							>
								<Text as="span" size="xs" font="mono" tabular>
									{error.line}
								</Text>
								<Box as="span" className="min-w-0 flex-1">
									{error.message}
								</Box>
							</Button>
						</li>
					))}
				</Stack>
			) : (
				<Box className="flex items-center justify-between gap-3 px-1">
					<Box
						as="p"
						className="flex items-center gap-1.5 text-sm text-muted-foreground"
					>
						<Check className="size-3.5 text-success" />
						{rows.length === 0
							? "Nothing declared yet."
							: `${rows.length} ${rows.length === 1 ? "field" : "fields"} parsed.`}
					</Box>
					<code className="text-xs text-muted-foreground">
						name: type [rules] # description
					</code>
				</Box>
			)}
		</Stack>
	);
}
