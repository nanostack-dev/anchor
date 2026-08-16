import type {
	OrganizationLicenseMigrationResponse,
	OrganizationLicenseMigrationResult,
} from "@/client";
import { LicenseMigrationOutcome } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import {
	OUTCOME_LABELS,
	OUTCOME_TONES,
	SKIP_REASON_LABELS,
} from "./license-migration-format";

interface LicenseMigrationOutcomesProps {
	migration: OrganizationLicenseMigrationResponse;
	organizationNames: Record<string, string>;
}

const OUTCOME_ORDER: LicenseMigrationOutcome[] = [
	LicenseMigrationOutcome.FAILED,
	LicenseMigrationOutcome.SKIPPED,
	LicenseMigrationOutcome.MIGRATED,
	LicenseMigrationOutcome.UNCHANGED,
];

function resultDetail(result: OrganizationLicenseMigrationResult): string {
	if (result.outcome === LicenseMigrationOutcome.FAILED) {
		return result.error?.message ?? "The write was refused.";
	}
	if (result.outcome === LicenseMigrationOutcome.SKIPPED) {
		return result.reason
			? SKIP_REASON_LABELS[result.reason]
			: "Left alone by this run.";
	}
	if (result.outcome === LicenseMigrationOutcome.UNCHANGED) {
		return "Already held these values from this tier.";
	}
	return result.count === 0
		? "Every value it held already matched."
		: `${result.count} license field${result.count === 1 ? "" : "s"} changed.`;
}

/**
 * What a migration run did, worst outcome first: a run reports per organization
 * and keeps going, so the failures and the skips are the part an operator has
 * to act on and the successes are the part they only need counted.
 */
export function LicenseMigrationOutcomes({
	migration,
	organizationNames,
}: LicenseMigrationOutcomesProps) {
	const groups = OUTCOME_ORDER.map((outcome) => ({
		outcome,
		results: migration.results.filter((result) => result.outcome === outcome),
	})).filter((group) => group.results.length > 0);

	return (
		<div className="flex flex-col gap-4">
			<dl className="grid grid-cols-2 gap-3 sm:grid-cols-4">
				{[
					{ label: "Moved", value: migration.migrated },
					{ label: "Already there", value: migration.unchanged },
					{ label: "Skipped", value: migration.skipped },
					{ label: "Failed", value: migration.failed },
				].map((tally) => (
					<div key={tally.label} className="rounded-lg bg-muted/50 p-3">
						<dt className="text-xs text-muted-foreground">{tally.label}</dt>
						<dd className="text-lg font-semibold tabular-nums">
							{tally.value}
						</dd>
					</div>
				))}
			</dl>

			{groups.map((group) => (
				<section key={group.outcome} className="flex flex-col gap-2">
					<h3 className="text-sm font-medium">
						{OUTCOME_LABELS[group.outcome]}
						<span className="ml-2 font-normal text-muted-foreground tabular-nums">
							{group.results.length}
						</span>
					</h3>
					<ul className="divide-y divide-border rounded-lg border border-border">
						{group.results.map((result) => (
							<li
								key={result.organization_id}
								className="flex items-start justify-between gap-3 px-3 py-2.5"
							>
								<div className="min-w-0">
									<span className="block truncate text-sm font-medium">
										{organizationNames[result.organization_id] ??
											result.organization_id}
									</span>
									<span className="block text-xs text-muted-foreground">
										{resultDetail(result)}
									</span>
								</div>
								<StatusBadge tone={OUTCOME_TONES[group.outcome]}>
									{OUTCOME_LABELS[group.outcome]}
								</StatusBadge>
							</li>
						))}
					</ul>
				</section>
			))}
		</div>
	);
}
