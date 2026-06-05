import { cn } from "@/lib/utils";
import { type VariantProps, cva } from "class-variance-authority";
import { AlertCircle, AlertTriangle, CheckCircle, Info } from "lucide-react";
import type React from "react";
import { Alert, AlertDescription, AlertTitle } from "../ui/alert";

const formAlertVariants = cva("", {
	variants: {
		variant: {
			default: "bg-destructive/10 text-destructive border-destructive/20",
			warning: "bg-warning/10 text-warning border-warning/20",
			info: "bg-accent-soft text-accent-foreground border-border",
			success: "bg-success/10 text-success border-success/20",
		},
	},
	defaultVariants: {
		variant: "default",
	},
});

const alertVariantMap = {
	default: "destructive",
	warning: "warning",
	info: "default",
	success: "success",
} as const;

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
				return <AlertTriangle className="size-4" />;
			case "info":
				return <Info className="size-4" />;
			case "success":
				return <CheckCircle className="size-4" />;
			default:
				return <AlertCircle className="size-4" />;
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
		<Alert
			variant={alertVariantMap[variant ?? "default"]}
			className={cn(formAlertVariants({ variant }), className)}
			{...props}
		>
			{getIcon()}
			<AlertTitle>{getTitle()}</AlertTitle>
			<AlertDescription>{message}</AlertDescription>
		</Alert>
	);
}
