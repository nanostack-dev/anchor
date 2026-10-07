import type { WorkflowCatalogResponse, WorkflowRunResponse } from "@/client";
import { Button } from "@nanostackorg/design-system/components/button";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { History } from "lucide-react";
import { useState } from "react";
import { WorkflowRunView } from "./WorkflowRunView";
import { WorkflowStatusBadge } from "./WorkflowStatusBadge";

function relativeTime(value: string) {
	const seconds = Math.round((Date.now() - new Date(value).getTime()) / 1000);
	const format = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });
	if (Math.abs(seconds) < 60) return format.format(-seconds, "second");
	if (Math.abs(seconds) < 3600)
		return format.format(-Math.round(seconds / 60), "minute");
	if (Math.abs(seconds) < 86400)
		return format.format(-Math.round(seconds / 3600), "hour");
	return format.format(-Math.round(seconds / 86400), "day");
}

export function WorkflowRunHistory({
	runs,
	catalog,
	showWorkflowName = false,
	onOpenWorkflow,
	emptyLabel = "No run yet. Runs appear here as matching events happen.",
}: {
	runs: WorkflowRunResponse[];
	catalog?: WorkflowCatalogResponse;
	showWorkflowName?: boolean;
	onOpenWorkflow?: (workflowId: string) => void;
	emptyLabel?: string;
}) {
	const [openRunId, setOpenRunId] = useState<string | null>(
		runs[0]?.id ?? null,
	);
	const openRun = runs.find((run) => run.id === openRunId) ?? runs[0];

	if (runs.length === 0) {
		return (
			<Box className="flex items-center gap-3 rounded-xl border border-dashed border-border p-6">
				<History className="size-5 text-muted-foreground" aria-hidden />
				<Text size="sm" tone="muted">
					{emptyLabel}
				</Text>
			</Box>
		);
	}

	return (
		<Box className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,20rem)_minmax(0,1fr)]">
			<Box as="ul" aria-label="Runs" className="space-y-1">
				{runs.map((run) => {
					const selected = run.id === openRun?.id;
					return (
						<Box as="li" key={run.id}>
							<button
								type="button"
								aria-current={selected ? "true" : undefined}
								onClick={() => setOpenRunId(run.id)}
								className={`flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left transition-colors ${
									selected ? "bg-muted" : "hover:bg-muted/60"
								}`}
							>
								<Box as="span" className="min-w-0 flex-1">
									<Box as="span" className="block truncate text-sm font-medium">
										{showWorkflowName
											? (run.workflow_name ?? run.workflow_id)
											: run.event_type}
									</Box>
									<Box
										as="span"
										className="block truncate text-xs text-muted-foreground"
									>
										{relativeTime(run.started_at)}
										{showWorkflowName ? ` · ${run.event_type}` : ""}
									</Box>
								</Box>
								<WorkflowStatusBadge status={run.status} />
							</button>
						</Box>
					);
				})}
			</Box>
			{openRun ? (
				<Box className="rounded-xl border border-border bg-card p-4">
					<Stack space="md">
						{showWorkflowName && onOpenWorkflow ? (
							<Box>
								<Button
									variant="ghost"
									size="sm"
									onClick={() => onOpenWorkflow(openRun.workflow_id)}
								>
									Open workflow
								</Button>
							</Box>
						) : null}
						<WorkflowRunView run={openRun} catalog={catalog} />
					</Stack>
				</Box>
			) : null}
		</Box>
	);
}
