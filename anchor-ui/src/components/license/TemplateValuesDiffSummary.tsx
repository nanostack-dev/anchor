import type { LicenseTemplateResponse } from "@/client";
import { Info } from "lucide-react";
import type { ReactNode } from "react";

interface TemplateValuesDiffSummaryProps {
	target: LicenseTemplateResponse;
	singleSourceName?: string;
	sourceCount: number;
	children: ReactNode;
}

/**
 * Frames the tier comparison, and says plainly when there is nothing to compare
 * against — a selection spread over several tiers has no single "before", and
 * showing one anyway would be a comfortable lie.
 */
export function TemplateValuesDiffSummary({
	target,
	singleSourceName,
	sourceCount,
	children,
}: TemplateValuesDiffSummaryProps) {
	return (
		<section className="flex flex-col gap-3">
			<div className="flex items-baseline justify-between gap-3">
				<h3 className="text-sm font-medium">
					{singleSourceName
						? `${singleSourceName} → ${target.name}`
						: `Moving to ${target.name}`}
				</h3>
				{sourceCount > 1 && (
					<span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
						<Info aria-hidden className="size-3.5" />
						{sourceCount} tiers in this selection
					</span>
				)}
			</div>
			{children}
		</section>
	);
}
