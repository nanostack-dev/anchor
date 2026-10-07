import type {
	WorkflowCatalogResponse,
	WorkflowRunResponse,
	WorkflowStepResultResponse,
} from "@/client";
import { WorkflowRunTrigger } from "@/client";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@nanostackorg/design-system/components/collapsible";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { ChevronRight } from "lucide-react";
import { WorkflowStatusBadge } from "./WorkflowStatusBadge";
import { findAction } from "./workflow-model";

const triggerLabels: Record<WorkflowRunTrigger, string> = {
	[WorkflowRunTrigger.EVENT]: "Started by event",
	[WorkflowRunTrigger.MANUAL]: "Run by hand",
	[WorkflowRunTrigger.DRY_RUN]: "Dry run",
};

function formatTime(value: string | undefined) {
	return value ? new Date(value).toLocaleString() : "";
}

function durationOf(run: WorkflowRunResponse) {
	if (!run.finished_at) return null;
	const ms = Math.max(
		new Date(run.finished_at).getTime() - new Date(run.started_at).getTime(),
		0,
	);
	if (ms < 1000) return `${ms} ms`;
	if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
	const minutes = Math.floor(ms / 60_000);
	return `${minutes} min ${Math.round((ms % 60_000) / 1000)} s`;
}

function KeyValues({ values }: { values: Record<string, unknown> }) {
	const entries = Object.entries(values);
	if (entries.length === 0) {
		return (
			<Text size="xs" tone="muted">
				Nothing
			</Text>
		);
	}
	return (
		<Box
			as="ul"
			className="grid grid-cols-[minmax(0,10rem)_minmax(0,1fr)] gap-x-3 gap-y-1"
		>
			{entries.map(([key, value]) => (
				<Box as="li" key={key} className="contents">
					<Text as="span" size="xs" tone="muted" font="mono" truncate>
						{key}
					</Text>
					<Box as="span" className="min-w-0 break-all font-mono text-xs">
						{typeof value === "string" ? value : JSON.stringify(value)}
					</Box>
				</Box>
			))}
		</Box>
	);
}

function StepResult({
	result,
	index,
	catalog,
}: {
	result: WorkflowStepResultResponse;
	index: number;
	catalog?: WorkflowCatalogResponse;
}) {
	const action = findAction(catalog, result.action);
	return (
		<Box as="li" className="rounded-lg border border-border bg-card">
			<Collapsible>
				<CollapsibleTrigger
					render={
						<button
							type="button"
							className="group flex w-full items-center gap-3 px-3 py-2 text-left"
						/>
					}
				>
					<ChevronRight className="size-4 shrink-0 text-muted-foreground transition-transform group-data-[panel-open]:rotate-90" />
					<Box
						as="span"
						className="flex size-6 shrink-0 items-center justify-center rounded-full bg-muted font-mono text-xs"
					>
						{index + 1}
					</Box>
					<Box as="span" className="min-w-0 flex-1">
						<Box as="span" className="block truncate text-sm font-medium">
							{action?.name ?? result.action}
						</Box>
						<Box
							as="span"
							className="block truncate font-mono text-xs text-muted-foreground"
						>
							{result.step_id}
						</Box>
					</Box>
					<Box as="span" className="shrink-0">
						<WorkflowStatusBadge status={result.status} />
					</Box>
				</CollapsibleTrigger>
				<CollapsibleContent>
					<Box className="space-y-3 border-t border-border px-3 py-3">
						{result.error ? (
							<Text size="sm" tone="critical">
								{result.error}
							</Text>
						) : null}
						<Stack space="xs">
							<Text size="xs" weight="semibold" tone="muted">
								Parameters
							</Text>
							<KeyValues values={result.params ?? {}} />
						</Stack>
						{result.output ? (
							<Stack space="xs">
								<Text size="xs" weight="semibold" tone="muted">
									Output
								</Text>
								<KeyValues values={result.output} />
							</Stack>
						) : null}
					</Box>
				</CollapsibleContent>
			</Collapsible>
		</Box>
	);
}

export function WorkflowRunView({
	run,
	catalog,
}: {
	run: WorkflowRunResponse;
	catalog?: WorkflowCatalogResponse;
}) {
	const duration = durationOf(run);
	return (
		<Stack space="md">
			<Spread alignY="center">
				<Inline space="sm" alignY="center" wrap>
					<WorkflowStatusBadge status={run.status} />
					<Text as="span" size="sm" tone="muted">
						{triggerLabels[run.trigger]}
					</Text>
				</Inline>
				<Text as="span" size="xs" tone="muted" tabular>
					{formatTime(run.started_at)}
					{duration ? ` · ${duration}` : ""}
				</Text>
			</Spread>
			{run.error ? (
				<Text size="sm" tone="critical">
					{run.error}
				</Text>
			) : null}
			<Stack space="xs">
				<Text size="xs" weight="semibold" tone="muted">
					{run.event_type}
				</Text>
				<KeyValues values={run.event_data} />
			</Stack>
			{run.steps.length > 0 ? (
				<Box as="ol" className="space-y-2">
					{run.steps.map((result, index) => (
						<StepResult
							key={result.step_id}
							result={result}
							index={index}
							catalog={catalog}
						/>
					))}
				</Box>
			) : (
				<Text size="sm" tone="muted">
					The workflow's conditions did not hold, so no step ran.
				</Text>
			)}
		</Stack>
	);
}
