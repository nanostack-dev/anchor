import {
	Field,
	FieldDescription,
	FieldError,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
import { useId } from "react";

export function BillingSelect({
	label,
	value,
	options,
	onChange,
	disabled,
	description,
	error,
}: {
	label: string;
	value: string;
	options: { value: string; label: string }[];
	onChange: (value: string) => void;
	disabled?: boolean;
	description?: string;
	error?: string;
}) {
	const id = useId();
	return (
		<Field invalid={!!error}>
			<FieldLabel htmlFor={id}>{label}</FieldLabel>
			<Select
				value={value || null}
				items={options}
				onValueChange={(next) => onChange(next ?? "")}
				disabled={disabled}
			>
				<SelectTrigger id={id} width="fill" aria-invalid={!!error}>
					<SelectValue placeholder="Choose an option" />
				</SelectTrigger>
				<SelectContent aria-label={`${label} options`}>
					{options.map((option) => (
						<SelectItem key={option.value} value={option.value}>
							{option.label}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
			{description && <FieldDescription>{description}</FieldDescription>}
			{error && <FieldError>{error}</FieldError>}
		</Field>
	);
}
