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

				return (
					<div key={field.id} className="flex flex-col gap-1.5 p-3">
						<div className="flex items-baseline justify-between gap-2">
							<Label htmlFor={inputId} className="font-mono text-sm">
								{field.name}
							</Label>
							{field.description && (
								<span className="truncate text-xs text-muted-foreground">
									{field.description}
								</span>
							)}
						</div>

						{readOnly ? (
							<p id={inputId} className="text-sm">
								{formatFieldValue(field.type, value)}
							</p>
						) : field.type === LicenseFieldType.BOOLEAN ? (
							<Switch
								id={inputId}
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
								<SelectTrigger id={inputId} className="w-full">
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
								type="number"
								value={typeof value === "number" ? value : ""}
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
								value={typeof value === "string" ? value : ""}
								onChange={(e) => onChange(field.name, e.target.value)}
								disabled={disabled}
							/>
						)}

						{error && <p className="text-sm text-destructive">{error}</p>}
					</div>
				);
			})}
		</div>
	);
}
