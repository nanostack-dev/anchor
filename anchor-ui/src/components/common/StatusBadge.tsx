import type * as React from "react";

import { Badge } from "@/components/ui/badge";

/**
 * Semantic status tones. Use these instead of hard-coded palette classes
 * (`bg-green-100 text-green-800`, etc.) so statuses follow the Nanostack
 * theme tokens and stay consistent across every table and detail surface.
 */
export type StatusTone =
	| "success"
	| "warning"
	| "destructive"
	| "info"
	| "neutral";

const toneToVariant: Record<
	StatusTone,
	React.ComponentProps<typeof Badge>["variant"]
> = {
	success: "success",
	warning: "warning",
	destructive: "destructive",
	info: "default",
	neutral: "secondary",
};

/**
 * Best-effort mapping from a raw status string to a semantic tone. Callers with
 * a known enum should pass an explicit `tone` instead of relying on this.
 */
export function inferStatusTone(status: string): StatusTone {
	const normalized = status.trim().toUpperCase();
	switch (normalized) {
		case "ACTIVE":
		case "ACTIVE_PLAN":
		case "ENABLED":
		case "HEALTHY":
		case "SENT":
		case "DELIVERED":
		case "PUBLISHED":
		case "CONNECTED":
		case "SUCCESS":
		case "SUCCEEDED":
		case "COMPLETED":
		case "TRUE":
			return "success";
		case "INVITED":
		case "PENDING":
		case "ROTATED":
		case "QUEUED":
		case "SENDING":
		case "SUPPRESSED":
		case "DRAFT":
		case "WARNING":
			return "warning";
		case "SUSPENDED":
		case "FAILED":
		case "REVOKED":
		case "DISABLED":
		case "ERROR":
		case "BOUNCED":
			return "destructive";
		case "INACTIVE":
		case "NONE":
		case "UNKNOWN":
		case "FALSE":
			return "neutral";
		default:
			return "info";
	}
}

export type StatusBadgeProps = Omit<
	React.ComponentProps<typeof Badge>,
	"variant"
> & {
	/** Explicit semantic tone. Falls back to inference from the label text. */
	tone?: StatusTone;
};

/**
 * Renders a status as a semantic `Badge`. Pass an explicit `tone` for known
 * enums; otherwise the tone is inferred from the child text.
 */
export function StatusBadge({ tone, children, ...props }: StatusBadgeProps) {
	const resolvedTone =
		tone ?? (typeof children === "string" ? inferStatusTone(children) : "info");
	return (
		<Badge variant={toneToVariant[resolvedTone]} {...props}>
			{children}
		</Badge>
	);
}
