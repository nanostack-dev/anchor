import { LicenseFieldType } from "@/client";
import { ArrowRight } from "lucide-react";
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
		return <p className="text-sm text-muted-foreground">{emptyMessage}</p>;
	}

	return (
		<div className="flex flex-col gap-2">
			<div className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-x-3 px-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
				<span className="truncate">{fromLabel}</span>
				<span aria-hidden className="w-4" />
				<span className="truncate text-right">{toLabel}</span>
			</div>

			<ul className="divide-y divide-border rounded-lg border border-border">
				{changes.map((change) => (
					<li
						key={change.field}
						className="grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-baseline gap-x-3 px-3 py-2.5"
					>
						<div className="min-w-0">
							<span className="block truncate font-mono text-xs text-muted-foreground">
								{change.field}
							</span>
							{/* A selection spread over several tiers has no single value
								to strike out; a struck-through dash would read as a
								rendering fault rather than as an absence. */}
							{change.from !== undefined && (
								<span className="block truncate text-sm tabular-nums line-through decoration-muted-foreground/50">
									{formatFieldValue(
										change.type ?? LicenseFieldType.STRING,
										change.from,
									)}
								</span>
							)}
						</div>

						<ArrowRight
							aria-hidden
							className="size-4 shrink-0 self-center text-muted-foreground"
						/>

						<div className="min-w-0 text-right">
							{change.carried ? (
								<span className="block truncate text-xs text-warning-strong">
									kept for this customer
								</span>
							) : (
								<span aria-hidden className="block h-4" />
							)}
							<span className="block truncate text-sm font-medium tabular-nums">
								{formatFieldValue(
									change.type ?? LicenseFieldType.STRING,
									change.to,
								)}
							</span>
						</div>
					</li>
				))}
			</ul>

			{unchangedCount !== undefined && unchangedCount > 0 && (
				<p className="px-3 text-xs text-muted-foreground">
					{unchangedCount} other license field
					{unchangedCount === 1 ? "" : "s"} unchanged.
				</p>
			)}
		</div>
	);
}
