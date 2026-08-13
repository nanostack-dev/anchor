import { LicenseFieldType } from "@/client";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { cn } from "@/lib/utils";
import { ChevronRight, CornerDownLeft, Plus, Trash2 } from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import { type ReactNode, useEffect, useState } from "react";

import { LicenseFieldRulesEditor } from "./LicenseFieldRulesEditor";
import {
	FIELD_TYPES,
	FIELD_TYPE_LABELS,
	summarizeRules,
} from "./license-field-format";
import { type FieldRow, newFieldRow } from "./license-schema-draft";

// Entry is slower than exit: opening is the operator's request and should be
// legible, closing is the system getting out of the way.
const EASE_OUT: [number, number, number, number] = [0.23, 1, 0.32, 1];
const OPEN_DURATION = 0.2;
const CLOSE_DURATION = 0.15;

interface LicenseSchemaFieldsEditorProps {
	fields: FieldRow[];
	onChange: (fields: FieldRow[]) => void;
	/** Keyed by `uiKey`, as `validateFieldRows` returns them. */
	errors?: Record<string, string>;
	disabled?: boolean;
	/** Rendered at the right of the header row, beside the field count. */
	headerAction?: ReactNode;
}

/**
 * Authors a license schema as a list of one-line fields, expanding only the
 * field being edited.
 *
 * The whole point is that the schema stays readable while it is written: a
 * collapsed row states the field's name, type, and effective rules, so an
 * operator declaring the tenth field can still see the first nine. Enter in a
 * name input commits the row and opens the next one, so a schema can be
 * declared without reaching for the mouse.
 */
export function LicenseSchemaFieldsEditor({
	fields,
	onChange,
	errors = {},
	disabled,
	headerAction,
}: LicenseSchemaFieldsEditorProps) {
	const reduceMotion = useReducedMotion();
	// A draft that is already declared opens fully collapsed — the reason to
	// show the schema as a list is to read all of it at once. Only a first
	// field, still blank, opens ready to type into.
	const [expandedKey, setExpandedKey] = useState<string | null>(() =>
		fields.length === 1 && !fields[0].name.trim() ? fields[0].uiKey : null,
	);
	const [autoFocusKey, setAutoFocusKey] = useState<string | null>(null);

	// A submit that fails validation must not leave the offending field folded
	// away behind a summary line.
	const firstErrorKey = fields.find((field) => errors[field.uiKey])?.uiKey;
	useEffect(() => {
		if (firstErrorKey) setExpandedKey(firstErrorKey);
	}, [firstErrorKey]);

	const patchRow = (uiKey: string, patch: Partial<FieldRow>) =>
		onChange(
			fields.map((field) =>
				field.uiKey === uiKey ? { ...field, ...patch } : field,
			),
		);

	// Changing type drops the previous type's rules: the backend rejects a rule
	// that does not apply to the field's type, so carrying them over would only
	// produce a server error later.
	const changeType = (uiKey: string, type: LicenseFieldType) =>
		onChange(
			fields.map((field) =>
				field.uiKey === uiKey ? { ...field, type, rules: {} } : field,
			),
		);

	const insertAfter = (uiKey: string | null) => {
		const row = newFieldRow();
		const at = uiKey ? fields.findIndex((field) => field.uiKey === uiKey) : -1;
		const next = [...fields];
		next.splice(at === -1 ? fields.length : at + 1, 0, row);
		onChange(next);
		setExpandedKey(row.uiKey);
		setAutoFocusKey(row.uiKey);
	};

	const removeRow = (uiKey: string) => {
		onChange(fields.filter((field) => field.uiKey !== uiKey));
		if (expandedKey === uiKey) setExpandedKey(null);
	};

	return (
		<div className="flex flex-col gap-2">
			<div className="flex items-center justify-between gap-3">
				<Label>Fields</Label>
				<div className="flex items-center gap-3">
					<span className="text-xs tabular-nums text-muted-foreground">
						{`${fields.length} ${fields.length === 1 ? "field" : "fields"}`}
					</span>
					{headerAction}
				</div>
			</div>

			<div className="divide-y divide-border overflow-hidden rounded-lg border border-border">
				<AnimatePresence initial={false}>
					{fields.map((field) => {
						const expanded = expandedKey === field.uiKey;
						const error = errors[field.uiKey];
						const name = field.name.trim();

						return (
							<motion.div
								key={field.uiKey}
								initial={reduceMotion ? false : { opacity: 0 }}
								animate={{ opacity: 1 }}
								exit={{
									opacity: 0,
									transition: { duration: CLOSE_DURATION, ease: EASE_OUT },
								}}
								transition={{ duration: OPEN_DURATION, ease: EASE_OUT }}
							>
								<div
									className={cn(
										"flex items-center gap-1 pr-1.5",
										error && "bg-destructive/5",
									)}
								>
									<button
										type="button"
										onClick={() =>
											setExpandedKey(expanded ? null : field.uiKey)
										}
										aria-expanded={expanded}
										aria-controls={`${field.uiKey}-detail`}
										className="flex min-w-0 flex-1 items-center gap-2.5 rounded-md px-2.5 py-2 text-left outline-none transition-colors hover:bg-muted/60 focus-visible:ring-3 focus-visible:ring-ring/50"
									>
										<ChevronRight
											className={cn(
												"size-3.5 shrink-0 text-muted-foreground transition-transform duration-150",
												expanded && "rotate-90",
											)}
										/>
										<span
											className={cn(
												"w-44 shrink-0 truncate font-mono text-sm",
												!name && "font-sans italic text-muted-foreground",
											)}
										>
											{name || "New field"}
										</span>
										<span className="w-20 shrink-0">
											<Badge variant="outline">
												{FIELD_TYPE_LABELS[field.type]}
											</Badge>
										</span>
										<span className="min-w-0 flex-1 truncate text-xs text-muted-foreground">
											{summarizeRules(field.type, field.rules)}
										</span>
									</button>
									<Button
										type="button"
										variant="ghost"
										size="icon-sm"
										className="shrink-0 text-muted-foreground hover:text-destructive"
										onClick={() => removeRow(field.uiKey)}
										disabled={disabled}
									>
										<span className="sr-only">
											Remove {name || "new field"}
										</span>
										<Trash2 />
									</Button>
								</div>

								{/*
								 * `grid-template-rows: 0fr -> 1fr` rather than an animated height:
								 * the height never has to be measured, so a panel whose content
								 * lays out after the animation starts still opens to all of it.
								 * Collapsed content stays mounted but `inert`, which keeps it out
								 * of the tab order and out of the accessibility tree.
								 */}
								<div
									id={`${field.uiKey}-detail`}
									inert={!expanded}
									style={{ gridTemplateRows: expanded ? "1fr" : "0fr" }}
									className="grid transition-[grid-template-rows] duration-200 ease-[cubic-bezier(0.23,1,0.32,1)] motion-reduce:transition-none"
								>
									<div className="overflow-hidden">
										<div className="flex flex-col gap-3 border-t border-border/60 px-2.5 py-3">
											<div className="flex items-start gap-2">
												<div className="flex-1 space-y-1.5">
													<Label
														htmlFor={`${field.uiKey}-name`}
														className="text-xs"
													>
														Name
													</Label>
													<Input
														id={`${field.uiKey}-name`}
														// The row was just opened by the operator; focus is why it opened.
														autoFocus={autoFocusKey === field.uiKey}
														value={field.name}
														onChange={(e) =>
															patchRow(field.uiKey, { name: e.target.value })
														}
														onKeyDown={(e) => {
															if (e.key !== "Enter") return;
															e.preventDefault();
															insertAfter(field.uiKey);
														}}
														placeholder="max_flows"
														maxLength={120}
														className="font-mono"
														aria-invalid={!!error}
														disabled={disabled}
													/>
												</div>
												<div className="w-36 space-y-1.5">
													<Label className="text-xs">Type</Label>
													<Select
														value={field.type}
														onValueChange={(value) =>
															changeType(field.uiKey, value as LicenseFieldType)
														}
														disabled={disabled}
													>
														<SelectTrigger className="w-full">
															<SelectValue>
																{(value: LicenseFieldType) =>
																	FIELD_TYPE_LABELS[value]
																}
															</SelectValue>
														</SelectTrigger>
														<SelectContent>
															{FIELD_TYPES.map((type) => (
																<SelectItem key={type} value={type}>
																	{FIELD_TYPE_LABELS[type]}
																</SelectItem>
															))}
														</SelectContent>
													</Select>
												</div>
											</div>

											<div className="space-y-1.5">
												<Label
													htmlFor={`${field.uiKey}-description`}
													className="text-xs"
												>
													Description
												</Label>
												<Input
													id={`${field.uiKey}-description`}
													value={field.description ?? ""}
													onChange={(e) =>
														patchRow(field.uiKey, {
															description: e.target.value,
														})
													}
													placeholder="Optional"
													disabled={disabled}
												/>
											</div>

											{field.type !== LicenseFieldType.BOOLEAN && (
												<LicenseFieldRulesEditor
													idPrefix={field.uiKey}
													type={field.type}
													rules={field.rules}
													onChange={(rules) => patchRow(field.uiKey, { rules })}
													disabled={disabled}
												/>
											)}

											{error && (
												<p className="text-sm text-destructive">{error}</p>
											)}
										</div>
									</div>
								</div>
							</motion.div>
						);
					})}
				</AnimatePresence>
			</div>

			<div className="flex items-center justify-between">
				<Button
					type="button"
					variant="outline"
					size="sm"
					onClick={() => insertAfter(fields.at(-1)?.uiKey ?? null)}
					disabled={disabled}
				>
					<Plus />
					Add field
				</Button>
				<span className="flex items-center gap-1.5 text-xs text-muted-foreground">
					<CornerDownLeft className="size-3" />
					Enter in a name adds the next field
				</span>
			</div>
		</div>
	);
}
