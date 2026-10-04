import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { AlertCircle, AlertTriangle, CheckCircle, Info } from "lucide-react";
import type { ReactNode } from "react";

export interface FormAlertProps {
	variant?: "default" | "warning" | "info" | "success";
	title?: string;
	message?: string | null;
	icon?: ReactNode;
	id?: string;
}

const alerts = {
	default: { tone: "critical", title: "Error", icon: AlertCircle },
	warning: { tone: "warning", title: "Warning", icon: AlertTriangle },
	info: { tone: "info", title: "Information", icon: Info },
	success: { tone: "success", title: "Success", icon: CheckCircle },
} as const;

export function FormAlert({
	variant = "default",
	title,
	message,
	icon,
	id,
}: FormAlertProps) {
	if (!message) return null;
	const alert = alerts[variant];
	const Icon = alert.icon;
	return (
		<Alert tone={alert.tone} id={id}>
			{icon ?? <Icon aria-hidden />}
			<AlertTitle>{title ?? alert.title}</AlertTitle>
			<AlertDescription>{message}</AlertDescription>
		</Alert>
	);
}
