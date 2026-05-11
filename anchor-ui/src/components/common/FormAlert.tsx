import { cn } from "@/lib/utils";
import { type VariantProps, cva } from "class-variance-authority";
import { AlertCircle, AlertTriangle, CheckCircle, Info } from "lucide-react";
import type React from "react";
import { Alert, AlertDescription, AlertTitle } from "../ui/alert";

const formAlertVariants = cva("", {
	variants: {
		variant: {
			default: "bg-destructive/10 text-destructive border-destructive/20",
			warning:
				"bg-yellow-500/10 text-yellow-700 dark:text-yellow-400 border-yellow-500/20",
			info: "bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/20",
			success:
				"bg-green-500/10 text-green-700 dark:text-green-400 border-green-500/20",
		},
	},
	defaultVariants: {
		variant: "default",
	},
});

export interface FormAlertProps
	extends React.HTMLAttributes<HTMLDivElement>,
		VariantProps<typeof formAlertVariants> {
	title?: string;
	message?: string | null;
	icon?: React.ReactNode;
}

export function FormAlert({
	className,
	variant,
	title,
	message,
	icon,
	...props
}: FormAlertProps) {
	if (!message) return null;

	const getIcon = () => {
		if (icon) return icon;
		switch (variant) {
			case "warning":
				return <AlertTriangle className="h-4 w-4" />;
			case "info":
				return <Info className="h-4 w-4" />;
			case "success":
				return <CheckCircle className="h-4 w-4" />;
			default:
				return <AlertCircle className="h-4 w-4" />;
		}
	};

	const getTitle = () => {
		if (title) return title;
		switch (variant) {
			case "warning":
				return "Warning";
			case "info":
				return "Information";
			case "success":
				return "Success";
			default:
				return "Error";
		}
	};

	return (
		<Alert className={cn(formAlertVariants({ variant }), className)} {...props}>
			{getIcon()}
			<AlertTitle>{getTitle()}</AlertTitle>
			<AlertDescription>{message}</AlertDescription>
		</Alert>
	);
}
