import type {
	OrganizationLicenseMigrationResponse,
	OrganizationLicenseMigrationResult,
} from "@/client";
import { LicenseMigrationOutcome } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { OUTCOME_LABELS, OUTCOME_TONES } from "./license-migration-format";

interface LicenseMigrationOutcomesProps {
	migration: OrganizationLicenseMigrationResponse;
	organizationNames: Record<string, string>;
}

const OUTCOME_ORDER: LicenseMigrationOutcome[] = [
	LicenseMigrationOutcome.FAILED,
	LicenseMigrationOutcome.CHANGED,
	LicenseMigrationOutcome.UNCHANGED,
];

function resultDetail(result: OrganizationLicenseMigrationResult): string {
	if (result.outcome === LicenseMigrationOutcome.FAILED) {
		return result.error?.message ?? "The write was refused.";
	}
	if (result.outcome === LicenseMigrationOutcome.UNCHANGED) {
		return "Already held these values from this tier.";
	}
	// No previous_template_id means this run granted the organization its
	// first license rather than moving it — see
	// docs/adr/0015-migrate-grants-a-first-license.md.
	if (!result.previous_template_id) {
		return "Granted this tier — held no license before this run.";
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
		<Stack space="lg">
			<dl className="grid grid-cols-3 gap-3">
				{[
					{
						label: OUTCOME_LABELS[LicenseMigrationOutcome.CHANGED],
						value: migration.changed,
					},
					{
						label: OUTCOME_LABELS[LicenseMigrationOutcome.UNCHANGED],
						value: migration.unchanged,
					},
					{
						label: OUTCOME_LABELS[LicenseMigrationOutcome.FAILED],
						value: migration.failed,
					},
				].map((tally) => (
					<Box key={tally.label} className="rounded-lg bg-muted/50 p-3">
						<Text as="dt" size="xs" tone="muted">
							{tally.label}
						</Text>
						<Text as="dd" size="lg" weight="semibold" tabular>
							{tally.value}
						</Text>
					</Box>
				))}
			</dl>

			{groups.map((group) => (
				<Stack key={group.outcome} space="sm" as="section">
					<Heading level={3}>
						{OUTCOME_LABELS[group.outcome]}
						<Box
							as="span"
							className="ml-2 font-normal text-muted-foreground tabular-nums"
						>
							{group.results.length}
						</Box>
					</Heading>
					<Box
						as="ul"
						className="divide-y divide-border rounded-lg border border-border"
					>
						{group.results.map((result) => (
							<Box
								as="li"
								key={result.organization_id}
								className="flex items-start justify-between gap-3 px-3 py-2.5"
							>
								<Box className="min-w-0">
									<Text as="span" weight="medium" truncate>
										{organizationNames[result.organization_id] ??
											result.organization_id}
									</Text>
									<Text as="span" size="xs" tone="muted">
										{resultDetail(result)}
									</Text>
								</Box>
								<StatusBadge tone={OUTCOME_TONES[group.outcome]}>
									{OUTCOME_LABELS[group.outcome]}
								</StatusBadge>
							</Box>
						))}
					</Box>
				</Stack>
			))}
		</Stack>
	);
}
