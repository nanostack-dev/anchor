import { LicenseFieldType } from "@/client";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { ArrowRightIcon as ArrowRight } from "@phosphor-icons/react";
import { formatFieldValue } from "./license-field-format";
import type { ValueChange } from "./license-migration-format";

interface TemplateValuesDiffProps {
	changes: ValueChange[];
	fromLabel: string;
	toLabel: string;
	unchangedCount?: number;
	emptyMessage?: string;
}

/**
 * What moves between two sets of license field values, field by field.
 *
 * There is no server-side preview of a migration, so this is what an operator
 * reads before running one. A row marked as carried reads the other way round:
 * the target's value is what would apply, and the organization's own value is
 * what keeps applying instead.
 */
export function TemplateValuesDiff({
	changes,
	fromLabel,
	toLabel,
	unchangedCount,
	emptyMessage = "The two tiers grant exactly the same values.",
}: TemplateValuesDiffProps) {
	if (changes.length === 0) {
		return <Text tone="muted">{emptyMessage}</Text>;
	}

	return (
		<Stack space="sm">
			<Box className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-x-3 px-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
				<Text as="span" truncate>
					{fromLabel}
				</Text>
				<Box as="span" aria-hidden className="w-4" />
				<Text as="span" truncate align="end">
					{toLabel}
				</Text>
			</Box>

			<Box
				as="ul"
				className="divide-y divide-border rounded-lg border border-border"
			>
				{changes.map((change) => (
					<Box
						as="li"
						key={change.field}
						className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-baseline gap-x-3 px-3 py-2.5"
					>
						<Box className="min-w-0">
							<Text as="span" size="xs" tone="muted" font="mono" truncate>
								{change.field}
							</Text>
							{/* A selection spread over several tiers has no single value
								to strike out; a struck-through dash would read as a
								rendering fault rather than as an absence. */}
							{change.from !== undefined && (
								<Box
									as="span"
									className="block truncate text-sm tabular-nums line-through decoration-muted-foreground/50"
								>
									{formatFieldValue(
										change.type ?? LicenseFieldType.STRING,
										change.from,
									)}
								</Box>
							)}
						</Box>

						<ArrowRight
							aria-hidden
							className="size-4 shrink-0 self-center text-muted-foreground"
						/>

						<Box className="min-w-0 text-right">
							{change.carried ? (
								<Text as="span" size="xs" tone="warning" truncate>
									kept for this customer
								</Text>
							) : (
								<Box as="span" aria-hidden className="block h-4" />
							)}
							<Text as="span" weight="medium" tabular truncate>
								{formatFieldValue(
									change.type ?? LicenseFieldType.STRING,
									change.to,
								)}
							</Text>
						</Box>
					</Box>
				))}
			</Box>

			{unchangedCount !== undefined && unchangedCount > 0 && (
				<Box as="p" className="px-3 text-xs text-muted-foreground">
					{unchangedCount} other license field
					{unchangedCount === 1 ? "" : "s"} unchanged.
				</Box>
			)}
		</Stack>
	);
}
