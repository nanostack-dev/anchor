import { LicenseFieldType } from "@/client";
import { ArrowRight } from "lucide-react";
import { formatFieldValue } from "./license-field-format";
import type { ValueChange } from "./license-migration-format";

export interface OrganizationAdjustments {
	organization: string;
	changes: ValueChange[];
}

interface CarriedAdjustmentsProps {
	groups: OrganizationAdjustments[];
	tierName: string;
	maxOrganizations?: number;
}

/**
 * Which customers keep which adjustment through a migration.
 *
 * Grouped by organization rather than listed flat: a cohort of any size
 * produces several rows for the same license field, and `max_flows → 1200`
 * next to `max_flows → 260` says nothing without the customer's name on it.
 *
 * Only the first few organizations are listed. A migration can carry five
 * hundred, and a dialog that renders every one of them is a wall rather than
 * an answer — the tail is counted instead.
 */
export function CarriedAdjustments({
	groups,
	tierName,
	maxOrganizations = 5,
}: CarriedAdjustmentsProps) {
	if (groups.length === 0) return null;

	const shown = groups.slice(0, maxOrganizations);
	const hidden = groups.slice(maxOrganizations);
	const hiddenChanges = hidden.reduce(
		(total, group) => total + group.changes.length,
		0,
	);

	return (
		<div className="flex flex-col gap-2">
			<div className="flex items-baseline justify-between gap-3 px-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
				<span>Adjustments kept</span>
				<span className="tabular-nums">
					{groups.length} organization{groups.length === 1 ? "" : "s"}
				</span>
			</div>

			<ul className="divide-y divide-border rounded-lg border border-border">
				{shown.map((group) => (
					<li key={group.organization} className="flex flex-col gap-1.5 p-3">
						<span className="truncate text-sm font-medium">
							{group.organization}
						</span>
						<ul className="flex flex-col gap-1">
							{group.changes.map((change) => (
								<li
									key={change.field}
									className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-baseline gap-x-3"
								>
									<span className="truncate font-mono text-xs text-muted-foreground">
										{change.field}
									</span>
									<span className="flex items-baseline gap-2">
										<span className="text-xs text-muted-foreground">
											{tierName}
										</span>
										<span className="text-sm tabular-nums line-through decoration-muted-foreground/50">
											{formatFieldValue(
												change.type ?? LicenseFieldType.STRING,
												change.from,
											)}
										</span>
									</span>
									<span className="flex items-baseline gap-2">
										<ArrowRight
											aria-hidden
											className="size-4 shrink-0 self-center text-muted-foreground"
										/>
										<span className="text-sm font-medium tabular-nums text-warning">
											{formatFieldValue(
												change.type ?? LicenseFieldType.STRING,
												change.to,
											)}
										</span>
									</span>
								</li>
							))}
						</ul>
					</li>
				))}
			</ul>

			{hidden.length > 0 && (
				<p className="px-3 text-xs text-muted-foreground">
					{hidden.length} more organization{hidden.length === 1 ? "" : "s"} keep
					{hidden.length === 1 ? "s" : ""} {hiddenChanges} adjustment
					{hiddenChanges === 1 ? "" : "s"}.
				</p>
			)}
		</div>
	);
}
