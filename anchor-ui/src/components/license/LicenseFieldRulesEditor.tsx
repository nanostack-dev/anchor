import { type LicenseFieldRules, LicenseFieldType } from "@/client";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";

interface LicenseFieldRulesEditorProps {
	idPrefix: string;
	type: LicenseFieldType;
	rules: LicenseFieldRules;
	onChange: (rules: LicenseFieldRules) => void;
	disabled?: boolean;
}

function numberOrUndefined(raw: string): number | undefined {
	if (raw.trim() === "") return undefined;
	const n = Number(raw);
	return Number.isNaN(n) ? undefined : n;
}

/**
 * Rule inputs conditioned on a license field's type — only the rules the
 * backend evaluator applies for that type (`internal/license/rules`) are
 * shown, so a form can never submit a rule the server would reject as
 * inapplicable.
 */
export function LicenseFieldRulesEditor({
	idPrefix,
	type,
	rules,
	onChange,
	disabled,
}: LicenseFieldRulesEditorProps) {
	if (type === LicenseFieldType.LIMIT || type === LicenseFieldType.NUMBER) {
		return (
			<Box className="grid grid-cols-2 gap-3">
				<Stack space="xs">
					<Label htmlFor={`${idPrefix}-min`}>Min</Label>
					<Input
						id={`${idPrefix}-min`}
						type="number"
						value={rules.min ?? ""}
						onChange={(e) =>
							onChange({ ...rules, min: numberOrUndefined(e.target.value) })
						}
						placeholder="No minimum"
						disabled={disabled}
					/>
				</Stack>
				<Stack space="xs">
					<Label htmlFor={`${idPrefix}-max`}>Max</Label>
					<Input
						id={`${idPrefix}-max`}
						type="number"
						value={rules.max ?? ""}
						onChange={(e) =>
							onChange({ ...rules, max: numberOrUndefined(e.target.value) })
						}
						placeholder="No maximum"
						disabled={disabled}
					/>
				</Stack>
			</Box>
		);
	}

	if (type === LicenseFieldType.STRING) {
		return (
			<Box className="grid grid-cols-3 gap-3">
				<Box className="col-span-3 space-y-1.5">
					<Label htmlFor={`${idPrefix}-pattern`}>
						Pattern (regular expression)
					</Label>
					<Input
						font="mono"
						id={`${idPrefix}-pattern`}
						value={rules.pattern ?? ""}
						onChange={(e) =>
							onChange({ ...rules, pattern: e.target.value || undefined })
						}
						placeholder="No pattern"
						disabled={disabled}
					/>
				</Box>
				<Stack space="xs">
					<Label htmlFor={`${idPrefix}-min-length`}>Min length</Label>
					<Input
						id={`${idPrefix}-min-length`}
						type="number"
						min={0}
						value={rules.min_length ?? ""}
						onChange={(e) =>
							onChange({
								...rules,
								min_length: numberOrUndefined(e.target.value),
							})
						}
						placeholder="None"
						disabled={disabled}
					/>
				</Stack>
				<Stack space="xs">
					<Label htmlFor={`${idPrefix}-max-length`}>Max length</Label>
					<Input
						id={`${idPrefix}-max-length`}
						type="number"
						min={0}
						value={rules.max_length ?? ""}
						onChange={(e) =>
							onChange({
								...rules,
								max_length: numberOrUndefined(e.target.value),
							})
						}
						placeholder="None"
						disabled={disabled}
					/>
				</Stack>
			</Box>
		);
	}

	if (type === LicenseFieldType.ENUM) {
		return (
			<Stack space="xs">
				<Label htmlFor={`${idPrefix}-values`}>
					Allowed values (comma-separated)
				</Label>
				<Input
					id={`${idPrefix}-values`}
					value={rules.values?.join(", ") ?? ""}
					onChange={(e) =>
						onChange({
							...rules,
							values: e.target.value
								.split(",")
								.map((v) => v.trim())
								.filter(Boolean),
						})
					}
					placeholder="free, pro, enterprise"
					disabled={disabled}
				/>
			</Stack>
		);
	}

	return (
		<Text size="xs" tone="muted">
			Boolean fields take no validation rules.
		</Text>
	);
}
