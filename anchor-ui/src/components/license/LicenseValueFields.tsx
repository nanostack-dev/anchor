import {
	type LicenseFieldResponse,
	LicenseFieldType,
	type LicenseTemplateValues,
} from "@/client";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMemo } from "react";
import { formatFieldValue } from "./license-field-format";

interface LicenseValueFieldsProps {
	fields: LicenseFieldResponse[];
	values: LicenseTemplateValues;
	/** Omit for a read-only render — every field then renders as plain text. */
	onChange?: (name: string, value: unknown) => void;
	errors?: Record<string, string>;
	disabled?: boolean;
	/**
	 * A short note per field name, shown under its label. The organization
	 * license form uses it to mark the fields that have come apart from the
	 * tier, which is the thing an operator adjusting one customer needs to see
	 * without leaving the form.
	 */
	notes?: Record<string, string>;
}

/**
 * One input (or, without `onChange`, one plain value) per field the product's
 * license schema declares. Shared by the template form, the template detail
 * view, and the read-only organization license view, so the three surfaces
 * that show "what a license sets" never grow their own copy of the same
 * per-type rendering decision.
 */
export function LicenseValueFields({
	fields,
	values,
	onChange,
	errors,
	disabled,
	notes,
}: LicenseValueFieldsProps) {
	const readOnly = !onChange;
	const enumItemsByField = useMemo(
		() =>
			new Map(
				fields.map((field) => [
					field.id,
					(field.rules.values ?? []).map((value) => ({ value, label: value })),
				]),
			),
		[fields],
	);

	if (fields.length === 0) {
		return (
			<Text tone="muted">
				This product&rsquo;s license schema declares no fields yet.
			</Text>
		);
	}

	return (
		<Box className="divide-y divide-border rounded-lg border border-border">
			{fields.map((field) => {
				const value = values[field.name];
				const enumItems = enumItemsByField.get(field.id) ?? [];
				const inputId = `license-value-${field.name}`;
				const error = errors?.[field.name];
				const errorId = `${inputId}-error`;
				// Red text under an otherwise normal-looking control is the only
				// signal a sighted mouse user gets, and none at all for a screen
				// reader. Both are wired to the control that caused it.
				const invalid = error
					? { "aria-invalid": true, "aria-describedby": errorId }
					: {};

				return (
					<Box key={field.id} className="flex flex-col gap-1.5 p-3">
						<Stack space="xs">
							<Label htmlFor={inputId}>{field.name}</Label>
							{field.description && (
								<Text as="span" size="xs" tone="muted">
									{field.description}
								</Text>
							)}
						</Stack>

						{/* A sentence, so it reads as one rather than as a status chip
							that cannot wrap on a narrow screen. */}
						{notes?.[field.name] && (
							<Text size="xs" tone="muted">
								{notes[field.name]}
							</Text>
						)}

						{readOnly ? (
							<Text id={inputId}>{formatFieldValue(field.type, value)}</Text>
						) : field.type === LicenseFieldType.BOOLEAN ? (
							<Switch
								id={inputId}
								{...invalid}
								checked={Boolean(value)}
								onCheckedChange={(checked) => onChange(field.name, checked)}
								disabled={disabled}
							/>
						) : field.type === LicenseFieldType.ENUM ? (
							<Select
								items={enumItems}
								value={typeof value === "string" ? value : null}
								onValueChange={(v) => onChange(field.name, v)}
								disabled={disabled}
							>
								<SelectTrigger width="fill" id={inputId} {...invalid}>
									<SelectValue placeholder="Select a value" />
								</SelectTrigger>
								<SelectContent aria-label={`${field.name} options`}>
									{enumItems.map((option) => (
										<SelectItem key={option.value} value={option.value}>
											{option.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						) : field.type === LicenseFieldType.LIMIT ||
							field.type === LicenseFieldType.NUMBER ? (
							<Input
								id={inputId}
								{...invalid}
								type="number"
								value={typeof value === "number" ? value : ""}
								onWheel={(event) => event.currentTarget.blur()}
								onChange={(e) =>
									onChange(
										field.name,
										e.target.value === "" ? undefined : Number(e.target.value),
									)
								}
								min={field.type === LicenseFieldType.LIMIT ? 0 : undefined}
								disabled={disabled}
							/>
						) : (
							<Input
								id={inputId}
								{...invalid}
								value={typeof value === "string" ? value : ""}
								onChange={(e) => onChange(field.name, e.target.value)}
								disabled={disabled}
							/>
						)}

						{error && (
							<Text id={errorId} tone="critical">
								{error}
							</Text>
						)}
					</Box>
				);
			})}
		</Box>
	);
}
