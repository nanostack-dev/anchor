import type {
	WorkflowCatalogResponse,
	WorkflowResponse,
	WorkflowRunResponse,
} from "@/client";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	ArrowRight,
	Plus,
	Sparkles,
	Workflow as WorkflowIcon,
} from "lucide-react";
import { WorkflowRunHistory } from "./WorkflowRunHistory";
import { WorkflowStatusBadge } from "./WorkflowStatusBadge";
import { actionGroupIcon } from "./action-icons";
import { type WorkflowRecipe, workflowRecipes } from "./recipes";
import { findAction, findTrigger } from "./workflow-model";

const VISIBLE_STEPS = 6;

function StepIcons({
	workflow,
	catalog,
}: {
	workflow: WorkflowResponse;
	catalog?: WorkflowCatalogResponse;
}) {
	const steps = workflow.definition.steps;
	const hidden = steps.length - VISIBLE_STEPS;
	return (
		<Inline space="xxs" alignY="center" wrap>
			{steps.slice(0, VISIBLE_STEPS).map((step, index) => {
				const action = findAction(catalog, step.action);
				const Icon = actionGroupIcon(action?.group);
				return (
					<Box as="span" key={step.id} className="flex items-center gap-1">
						{index > 0 ? (
							<ArrowRight
								className="size-3 text-muted-foreground"
								aria-hidden
							/>
						) : null}
						<Box
							as="span"
							title={action?.name ?? step.action}
							className="flex items-center gap-1 rounded-md bg-muted px-1.5 py-0.5 text-xs text-muted-foreground"
						>
							<Icon className="size-3" aria-hidden />
							{action?.name ?? step.action}
						</Box>
					</Box>
				);
			})}
			{hidden > 0 ? (
				<Box as="span" className="flex items-center gap-1">
					<ArrowRight className="size-3 text-muted-foreground" aria-hidden />
					<Box as="span" className="text-xs text-muted-foreground">
						+ {hidden} more
					</Box>
				</Box>
			) : null}
		</Inline>
	);
}

export function WorkflowList({
	workflows,
	runs,
	catalog,
	onOpen,
	onCreate,
}: {
	workflows: WorkflowResponse[];
	runs: WorkflowRunResponse[];
	catalog?: WorkflowCatalogResponse;
	onOpen: (workflowId: string) => void;
	onCreate: (recipe?: WorkflowRecipe) => void;
}) {
	const lastRunOf = (workflowId: string) =>
		runs.find((run) => run.workflow_id === workflowId);

	return (
		<Stack space="xxl">
			<Stack space="md">
				<Inline space="sm" alignY="center">
					<Heading level={2}>Your workflows</Heading>
					<Badge variant="soft" tone="neutral">
						{workflows.length}
					</Badge>
				</Inline>
				{workflows.length === 0 ? (
					<Box className="flex flex-col items-start gap-3 rounded-xl border border-dashed border-border p-6">
						<WorkflowIcon
							className="size-6 text-muted-foreground"
							aria-hidden
						/>
						<Text size="sm" tone="muted">
							A workflow reacts to a product event and changes your product's
							resources for you. Start from a recipe below, or from scratch.
						</Text>
						<Button tone="brand" icon={Plus} onClick={() => onCreate()}>
							New workflow
						</Button>
					</Box>
				) : (
					<Box
						as="ul"
						aria-label="Your workflows"
						className="divide-y divide-border rounded-xl border border-border bg-card"
					>
						{workflows.map((workflow) => {
							const trigger = findTrigger(catalog, workflow.trigger_event_type);
							const lastRun = lastRunOf(workflow.id);
							return (
								<Box as="li" key={workflow.id}>
									<button
										type="button"
										onClick={() => onOpen(workflow.id)}
										className="flex w-full flex-col gap-2 px-4 py-3 text-left transition-colors hover:bg-muted/50 sm:flex-row sm:items-center sm:gap-4"
									>
										<Box as="span" className="min-w-0 flex-1 space-y-1">
											<Box
												as="span"
												className="flex flex-wrap items-center gap-2"
											>
												<Box
													as="span"
													className="min-w-0 break-words text-sm font-semibold"
												>
													{workflow.name}
												</Box>
												{workflow.enabled ? null : (
													<Badge variant="outline" tone="neutral">
														Disabled
													</Badge>
												)}
											</Box>
											<Box
												as="span"
												className="block truncate text-xs text-muted-foreground"
											>
												When{" "}
												{trigger?.name.toLowerCase() ??
													workflow.trigger_event_type}
												{workflow.definition.conditions.length > 0
													? ` · ${workflow.definition.conditions.length} condition${workflow.definition.conditions.length > 1 ? "s" : ""}`
													: ""}
											</Box>
											<StepIcons workflow={workflow} catalog={catalog} />
										</Box>
										<Box as="span" className="shrink-0">
											{lastRun ? (
												<WorkflowStatusBadge status={lastRun.status} />
											) : (
												<Text as="span" size="xs" tone="muted">
													No run yet
												</Text>
											)}
										</Box>
									</button>
								</Box>
							);
						})}
					</Box>
				)}
			</Stack>

			<Stack space="md">
				<Inline space="sm" alignY="center">
					<Sparkles className="size-4 text-primary" aria-hidden />
					<Heading level={2}>Start from a recipe</Heading>
				</Inline>
				<Box
					as="ul"
					className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3"
				>
					{workflowRecipes.map((recipe) => (
						<Box as="li" key={recipe.id}>
							<button
								type="button"
								onClick={() => onCreate(recipe)}
								className="flex h-full w-full flex-col gap-2 rounded-xl border border-border bg-card p-4 text-left transition-colors hover:border-primary/40 hover:bg-primary/5"
							>
								<Text as="span" size="sm" weight="semibold">
									{recipe.title}
								</Text>
								<Text as="span" size="xs" tone="muted">
									{recipe.summary}
								</Text>
								<Box as="span" className="mt-auto pt-1">
									<StepIcons
										workflow={{
											...recipe.draft,
											id: recipe.id,
											created_at: "",
											updated_at: "",
										}}
										catalog={catalog}
									/>
								</Box>
							</button>
						</Box>
					))}
				</Box>
			</Stack>

			<Stack space="md">
				<Heading level={2}>Recent runs</Heading>
				<WorkflowRunHistory
					runs={runs}
					catalog={catalog}
					showWorkflowName
					onOpenWorkflow={onOpen}
					emptyLabel="No workflow has run yet."
				/>
			</Stack>
		</Stack>
	);
}
