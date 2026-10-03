import type { LicenseTemplateResponse } from "@/client";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { InfoIcon as Info } from "@phosphor-icons/react";
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
		<Stack space="md" as="section">
			<Spread space="md" alignY="baseline">
				<Heading level={3}>
					{singleSourceName
						? `${singleSourceName} → ${target.name}`
						: `Moving to ${target.name}`}
				</Heading>
				{sourceCount > 1 && (
					<Box
						as="span"
						className="inline-flex items-center gap-1.5 text-xs text-muted-foreground"
					>
						<Info aria-hidden className="size-3.5" />
						{sourceCount} tiers in this selection
					</Box>
				)}
			</Spread>
			{children}
		</Stack>
	);
}
