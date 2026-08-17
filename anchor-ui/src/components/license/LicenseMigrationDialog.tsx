import {
	type LicenseFieldResponse,
	LicenseMigrationDifferencePolicy,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
	type OrganizationLicenseMigrationResponse,
	type OrganizationLicenseSummaryResponse,
} from "@/client";
import { migrateOrganizationLicensesMutation } from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useMemo, useState } from "react";
import { toast } from "sonner";
import { CarriedAdjustments } from "./CarriedAdjustments";
import { LicenseMigrationOutcomes } from "./LicenseMigrationOutcomes";
import { TemplateValuesDiff } from "./TemplateValuesDiff";
import { TemplateValuesDiffSummary } from "./TemplateValuesDiffSummary";
import {
	carriedForwardChanges,
	templateValueChanges,
} from "./license-migration-format";

interface LicenseMigrationDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	productId: string;
	selection: OrganizationLicenseSummaryResponse[];
	templates: LicenseTemplateResponse[];
	fields: LicenseFieldResponse[];
	onMigrated?: () => void;
}

/**
 * Moves the selected organizations onto one license template.
 *
 * Anchor has no dry run. What stands in for one is the comparison right here
 * between the tier the selection is on and the tier it is moving to, which is
 * the question an operator actually asks — plus the per-organization outcomes
 * the run reports back, which replace the dialog body once it has run.
 */
export function LicenseMigrationDialog({
	open,
	onOpenChange,
	productId,
	selection,
	templates,
	fields,
	onMigrated,
}: LicenseMigrationDialogProps) {
	const queryClient = useQueryClient();
	const [targetId, setTargetId] = useState<string | null>(null);
	const [discardDifferences, setDiscardDifferences] = useState(false);
	const [migration, setMigration] =
		useState<OrganizationLicenseMigrationResponse | null>(null);

	const activeTemplates = useMemo(
		() =>
			templates.filter(
				(template) => template.status === LicenseTemplateStatus.ACTIVE,
			),
		[templates],
	);
	const target = activeTemplates.find((template) => template.id === targetId);

	const templatesById = useMemo(
		() => new Map(templates.map((template) => [template.id, template])),
		[templates],
	);

	/**
	 * Names are snapshotted when the run starts, not read from `selection`:
	 * a successful run clears the selection, and the results list still has to
	 * say which customers moved.
	 */
	const [migratedNames, setMigratedNames] = useState<Record<string, string>>(
		{},
	);

	/**
	 * The tiers the selection is spread across. One is the ordinary case and
	 * lets the dialog show a real before-and-after; several means no single
	 * "from" exists, so only the target's own values can be shown.
	 */
	const sourceTemplates = useMemo(() => {
		const ids = new Set(
			selection
				.map((item) => item.license?.template_id)
				.filter((id): id is string => Boolean(id)),
		);
		return [...ids]
			.map((id) => templatesById.get(id))
			.filter(Boolean) as LicenseTemplateResponse[];
	}, [selection, templatesById]);

	const singleSource =
		sourceTemplates.length === 1 ? sourceTemplates[0] : undefined;

	const tierChanges = useMemo(
		() =>
			target
				? templateValueChanges(singleSource?.values, target.values, fields)
				: [],
		[singleSource, target, fields],
	);

	const carried = useMemo(() => {
		if (!target || discardDifferences) return [];
		return selection
			.map((item) => ({
				organization: item.organization_name,
				changes: carriedForwardChanges(
					item,
					templatesById.get(item.license?.template_id ?? "")?.values,
					target.values,
					fields,
				),
			}))
			.filter((group) => group.changes.length > 0);
	}, [selection, target, templatesById, fields, discardDifferences]);

	const carriedCount = carried.reduce(
		(total, group) => total + group.changes.length,
		0,
	);

	const unlicensedCount = selection.filter((item) => !item.license).length;
	// A migration moves a customer between tiers and never puts one on their
	// first, so the button counts what will actually move rather than what was
	// ticked. All-unlicensed is a guaranteed no-op and does not run.
	const movable = selection.length - unlicensedCount;

	const migrate = useMutation({
		...migrateOrganizationLicensesMutation(),
		onSuccess: (result: OrganizationLicenseMigrationResponse) => {
			setMigration(result);
			void queryClient.invalidateQueries({
				predicate: (query) =>
					(query.queryKey[0] as { _id?: string } | undefined)?._id ===
					"searchOrganizationLicenses",
			});
			if (result.failed > 0) {
				toast.warning(
					`${result.migrated} moved, ${result.failed} failed. Review the results.`,
				);
			} else {
				toast.success(
					`${result.migrated} organization${result.migrated === 1 ? "" : "s"} moved to ${target?.name}.`,
				);
			}
			onMigrated?.();
		},
	});

	const close = (next: boolean) => {
		onOpenChange(next);
		if (!next) {
			setMigration(null);
			setTargetId(null);
			setDiscardDifferences(false);
			setMigratedNames({});
			migrate.reset();
		}
	};

	const run = () => {
		if (!target) return;
		setMigratedNames(
			Object.fromEntries(
				selection.map((item) => [item.organization_id, item.organization_name]),
			),
		);
		migrate.mutate({
			path: { product_id: productId },
			body: {
				template_id: target.id,
				organization_ids: selection.map((item) => item.organization_id),
				on_difference: discardDifferences
					? LicenseMigrationDifferencePolicy.DISCARD
					: LicenseMigrationDifferencePolicy.CARRY_FORWARD,
			},
		});
	};

	return (
		<Dialog open={open} onOpenChange={close}>
			{/* The body scrolls, not the dialog: with a long carried-adjustments
				list the confirming button would otherwise sit below the fold with
				nothing pointing to it. */}
			<DialogContent className="flex max-h-[85vh] flex-col sm:max-w-2xl">
				<DialogHeader>
					<DialogTitle>
						{migration
							? "Migration results"
							: `Move ${selection.length} organization${selection.length === 1 ? "" : "s"}`}
					</DialogTitle>
					<DialogDescription>
						{migration
							? `Run at ${new Date(migration.migrated_at).toLocaleString()}. Every organization moved keeps this in its license history.`
							: "The license takes the target tier's values and its provenance is restamped, so the record says which tier the customer is on now."}
					</DialogDescription>
				</DialogHeader>

				{migration ? (
					<div className="min-h-0 flex-1 overflow-y-auto">
						<LicenseMigrationOutcomes
							migration={migration}
							organizationNames={migratedNames}
						/>
					</div>
				) : (
					<div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto">
						<div className="flex flex-col gap-2">
							<Label htmlFor="license-migration-target">Move to tier</Label>
							<Select
								items={activeTemplates.map((template) => ({
									value: template.id,
									label: template.name,
								}))}
								value={targetId}
								onValueChange={setTargetId}
							>
								<SelectTrigger id="license-migration-target" className="w-full">
									<SelectValue placeholder="Select a tier..." />
								</SelectTrigger>
								<SelectContent>
									{activeTemplates.map((template) => (
										<SelectItem key={template.id} value={template.id}>
											{template.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							<p className="text-xs text-muted-foreground">
								Only active tiers can be moved onto. An archived tier is
								withdrawn, so nobody new can be put on it.
							</p>
						</div>

						{target && (
							<TemplateValuesDiffSummary
								target={target}
								singleSourceName={singleSource?.name}
								sourceCount={sourceTemplates.length}
							>
								<TemplateValuesDiff
									changes={tierChanges}
									fromLabel={singleSource?.name ?? "Current tier"}
									toLabel={target.name}
									emptyMessage={
										singleSource
											? "The two tiers grant exactly the same values."
											: "Select organizations on a single tier to compare the two."
									}
								/>
							</TemplateValuesDiffSummary>
						)}

						{target && (
							<div className="flex items-start justify-between gap-4 rounded-lg border border-border px-3 py-3">
								<div className="flex flex-col gap-1">
									<Label htmlFor="license-migration-discard">
										Discard customer adjustments
									</Label>
									<p className="text-xs text-muted-foreground">
										{discardDifferences
											? `Every selected organization takes ${target.name} exactly, adjustments included. An adjustment made for one customer is lost.`
											: carried.length > 0
												? `Every value moves to ${target.name}, except ${carriedCount} adjustment${carriedCount === 1 ? "" : "s"} held by ${carried.length} organization${carried.length === 1 ? "" : "s"}, which ${carriedCount === 1 ? "is" : "are"} kept.`
												: `No organization in the selection is adjusted, so every one of them takes ${target.name} whole.`}
									</p>
								</div>
								<Switch
									id="license-migration-discard"
									checked={discardDifferences}
									onCheckedChange={setDiscardDifferences}
								/>
							</div>
						)}

						{target && !discardDifferences && (
							<CarriedAdjustments groups={carried} tierName={target.name} />
						)}

						{unlicensedCount > 0 && (
							<FormAlert
								variant="warning"
								title="Some organizations hold no license"
								message={`${unlicensedCount} of the ${selection.length} selected will be skipped. A migration moves a customer between tiers; it never puts one on their first.`}
							/>
						)}

						<FormAlert message={getApiErrorMessage(migrate.error)} />
					</div>
				)}

				<DialogFooter>
					{migration ? (
						<Button onClick={() => close(false)}>Done</Button>
					) : (
						<>
							<Button variant="outline" onClick={() => close(false)}>
								Cancel
							</Button>
							<Button
								onClick={run}
								disabled={!target || migrate.isPending || movable === 0}
								variant={discardDifferences ? "destructive" : "default"}
							>
								{migrate.isPending && <Spinner />}
								Move {movable} organization{movable === 1 ? "" : "s"}
							</Button>
						</>
					)}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
