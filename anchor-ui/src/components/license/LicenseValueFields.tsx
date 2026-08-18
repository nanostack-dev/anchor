import {
	type LicenseFieldResponse,
	LicenseFieldType,
	type LicenseTemplateValues,
} from "@/client";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
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

	if (fields.length === 0) {
		return (
			<p className="text-sm text-muted-foreground">
				This product&rsquo;s license schema declares no fields yet.
			</p>
		);
	}

	return (
		<div className="divide-y divide-border rounded-lg border border-border">
			{fields.map((field) => {
				const value = values[field.name];
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
					<div key={field.id} className="flex flex-col gap-1.5 p-3">
						<div className="flex items-baseline justify-between gap-2">
							<Label htmlFor={inputId} className="min-w-0 font-mono text-sm">
								{field.name}
							</Label>
							{field.description && (
								<span className="truncate text-xs text-muted-foreground">
									{field.description}
								</span>
							)}
						</div>

						{/* A sentence, so it reads as one rather than as a status chip
							that cannot wrap on a narrow screen. */}
						{notes?.[field.name] && (
							<p className="text-xs text-muted-foreground">
								{notes[field.name]}
							</p>
						)}

						{readOnly ? (
							<p id={inputId} className="text-sm">
								{formatFieldValue(field.type, value)}
							</p>
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
								value={typeof value === "string" ? value : undefined}
								onValueChange={(v) => onChange(field.name, v)}
								disabled={disabled}
							>
								<SelectTrigger id={inputId} {...invalid} className="w-full">
									<SelectValue placeholder="Select a value" />
								</SelectTrigger>
								<SelectContent>
									{(field.rules.values ?? []).map((option) => (
										<SelectItem key={option} value={option}>
											{option}
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
							<p id={errorId} className="text-sm text-destructive">
								{error}
							</p>
						)}
					</div>
				);
			})}
		</div>
	);
}
