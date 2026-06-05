import { cn } from "@/lib/utils";
import type { StandardSchemaV1 } from "@standard-schema/spec";
import type { AnyFieldApi } from "@tanstack/react-form";
import { AlertCircle, AlertTriangle, CheckCircle, Info } from "lucide-react";
import type React from "react";

export interface FormErrorProps extends React.HTMLAttributes<HTMLDivElement> {
	field: AnyFieldApi;
	variant?: "default" | "warning" | "info" | "success";
	icon?: React.ReactNode;
}

const variantStyles: Record<string, string> = {
	default: "bg-destructive/10 text-destructive border-destructive/20",
	warning: "bg-warning/10 text-warning border-warning/20",
	info: "bg-accent-soft text-accent-foreground border-border",
	success: "bg-success/10 text-success border-success/20",
};

const variantIcons: Record<string, React.ReactNode> = {
	default: <AlertCircle className="size-4" />,
	warning: <AlertTriangle className="size-4" />,
	info: <Info className="size-4" />,
	success: <CheckCircle className="size-4" />,
};

function handleErrorMessage(issue: StandardSchemaV1.Issue): string {
	return issue.message || "Unknown error";
}

export function FormValidationError({
	className,
	variant = "default",
	icon,
	field,
	...props
}: FormErrorProps) {
	if (
		!field.state.meta.isTouched ||
		!field.state.meta.errors ||
		field.state.meta.errors.length === 0
	) {
		return null;
	}
	const errors = field.state.meta.errors as StandardSchemaV1.Issue[];

	return (
		<div className="flex flex-col gap-1">
			{errors.map((error, index) => {
				const formattedMessage = handleErrorMessage(error);
				if (!formattedMessage) return null;
				const errorKey = `${formattedMessage}-${index}`;

				return (
					<div
						key={errorKey}
						className={cn(
							"flex items-start gap-2 text-sm p-3 rounded-lg mt-1 border",
							variantStyles[variant],
							className,
						)}
						{...props}
					>
						<div className="shrink-0 mt-0.5">
							{icon ?? variantIcons[variant]}
						</div>
						<span className="flex-1">{formattedMessage}</span>
					</div>
				);
			})}
		</div>
	);
}
