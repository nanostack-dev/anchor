import type {
	WorkflowCatalogResponse,
	WorkflowResponse,
	WorkflowStep,
} from "@/client";
import {
	createWorkflowMutation,
	deleteWorkflowMutation,
	getWorkflowQueryKey,
	listWorkflowsQueryKey,
	updateWorkflowMutation,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrors } from "@/lib/api-error";
import { ConfirmDialog } from "@nanostackorg/design-system/blocks/confirm-dialog";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import {
	Field,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import {
	NativeSelect,
	NativeSelectOptGroup,
	NativeSelectOption,
} from "@nanostackorg/design-system/components/native-select";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { Text } from "@nanostackorg/design-system/components/text";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Column, Columns } from "@nanostackorg/design-system/layout/columns";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CircleAlert, Filter, Save, Trash2, Zap } from "lucide-react";
import { useId, useMemo, useState } from "react";
import { AddStepMenu } from "./AddStepMenu";
import { ConditionEditor } from "./ConditionEditor";
import { StepEditor } from "./StepEditor";
import { WorkflowTestPanel } from "./WorkflowTestPanel";
import { useWorkflowResources } from "./useWorkflowResources";
import {
	type WorkflowDraft,
	draftToRequest,
	findTrigger,
	groupBy,
	moveItem,
	newStep,
	triggerVariables,
	variablesBeforeStep,
} from "./workflow-model";

function FlowConnector() {
	return (
		<Box
			aria-hidden
			className="ml-8 h-6 w-px border-l-2 border-dashed border-border"
		/>
	);
}

function FlowNode({
	icon: Icon,
	eyebrow,
	title,
	children,
}: {
	icon: typeof Zap;
	eyebrow: string;
	title: string;
	children: React.ReactNode;
}) {
	return (
		<Box
			as="section"
			aria-label={title}
			className="rounded-xl border border-border bg-card shadow-xs"
		>
			<Box className="flex items-center gap-3 border-b border-border px-4 py-3">
				<Box className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground">
					<Icon className="size-4" aria-hidden />
				</Box>
				<Box className="min-w-0">
					<Text as="span" size="xs" tone="muted">
						{eyebrow}
					</Text>
					<Text as="span" size="sm" weight="semibold">
						{title}
					</Text>
				</Box>
			</Box>
			<Box className="px-4 py-4">{children}</Box>
		</Box>
	);
}

export function WorkflowBuilder({
	productId,
	catalog,
	initialDraft,
	workflow,
	onSaved,
	onDeleted,
}: {
	productId: string;
	catalog: WorkflowCatalogResponse;
	initialDraft: WorkflowDraft;
	workflow?: WorkflowResponse;
	onSaved: (workflow: WorkflowResponse) => void;
	onDeleted?: () => void;
}) {
	const queryClient = useQueryClient();
	const ids = {
		name: useId(),
		description: useId(),
		trigger: useId(),
		enabled: useId(),
	};
	const [draft, setDraft] = useState<WorkflowDraft>(initialDraft);
	const [savedDraft, setSavedDraft] = useState<WorkflowDraft>(initialDraft);
	const [saveErrors, setSaveErrors] = useState<string[]>([]);
	const resources = useWorkflowResources(productId);
	const trigger = findTrigger(catalog, draft.trigger_event_type);
	const dirty = useMemo(
		() => JSON.stringify(draft) !== JSON.stringify(savedDraft),
		[draft, savedDraft],
	);

	const setSteps = (steps: WorkflowStep[]) =>
		setDraft({ ...draft, definition: { ...draft.definition, steps } });

	const invalidate = (saved?: WorkflowResponse) => {
		void queryClient.invalidateQueries({
			queryKey: listWorkflowsQueryKey({ path: { product_id: productId } }),
		});
		if (saved) {
			queryClient.setQueryData(
				getWorkflowQueryKey({
					path: { product_id: productId, workflow_id: saved.id },
				}),
				saved,
			);
		}
	};
	const onSaveSuccess = (saved: WorkflowResponse) => {
		setSaveErrors([]);
		setSavedDraft(draft);
		invalidate(saved);
		toast.add({ type: "success", title: `Workflow “${saved.name}” saved.` });
		onSaved(saved);
	};
	const onSaveError = (error: unknown) => {
		const errors = getApiErrors(error).map((apiError) => {
			const location = apiError.metadata?.location ?? apiError.field;
			return location ? `${apiError.message} (${location})` : apiError.message;
		});
		setSaveErrors(
			errors.length ? errors : ["The workflow could not be saved."],
		);
	};
	const create = useMutation({
		...createWorkflowMutation(),
		onSuccess: onSaveSuccess,
		onError: onSaveError,
	});
	const update = useMutation({
		...updateWorkflowMutation(),
		onSuccess: onSaveSuccess,
		onError: onSaveError,
	});
	const remove = useMutation({
		...deleteWorkflowMutation(),
		onSuccess: () => {
			invalidate();
			toast.add({ type: "success", title: "Workflow deleted." });
			onDeleted?.();
		},
		onError: () =>
			toast.add({ type: "error", title: "The workflow could not be deleted." }),
	});

	const save = () => {
		const body = draftToRequest(draft);
		if (workflow) {
			update.mutate({
				path: { product_id: productId, workflow_id: workflow.id },
				body,
			});
		} else {
			create.mutate({ path: { product_id: productId }, body });
		}
	};

	const triggerGroups = groupBy(catalog.triggers, (item) => item.group_name);

	return (
		<Columns space="xl" collapseBelow="lg" alignY="start">
			<Column width="2/3">
				<Stack space="none">
					<Card>
						<CardContent>
							<FieldGroup>
								<Field>
									<FieldLabel htmlFor={ids.name}>Name</FieldLabel>
									<Input
										id={ids.name}
										placeholder="Default workspace for new organizations"
										value={draft.name}
										onChange={(event) =>
											setDraft({ ...draft, name: event.target.value })
										}
									/>
								</Field>
								<Field>
									<FieldLabel htmlFor={ids.description}>Description</FieldLabel>
									<Textarea
										id={ids.description}
										rows={2}
										placeholder="Why this workflow exists, for the next person reading it"
										value={draft.description ?? ""}
										onChange={(event) =>
											setDraft({ ...draft, description: event.target.value })
										}
									/>
								</Field>
							</FieldGroup>
						</CardContent>
					</Card>

					<FlowConnector />

					<FlowNode
						icon={Zap}
						eyebrow="When"
						title={trigger?.name ?? "Pick a trigger"}
					>
						<Stack space="sm">
							<Field>
								<FieldLabel htmlFor={ids.trigger} size="sm">
									Product event
								</FieldLabel>
								<NativeSelect
									id={ids.trigger}
									value={draft.trigger_event_type}
									onChange={(event) =>
										setDraft({
											...draft,
											trigger_event_type: event.target.value,
										})
									}
								>
									<NativeSelectOption value="" disabled>
										Choose the event that starts this workflow
									</NativeSelectOption>
									{triggerGroups.map(([group, triggers]) => (
										<NativeSelectOptGroup key={group} label={group}>
											{triggers.map((item) => (
												<NativeSelectOption key={item.type} value={item.type}>
													{item.name} — {item.type}
												</NativeSelectOption>
											))}
										</NativeSelectOptGroup>
									))}
								</NativeSelect>
							</Field>
							{trigger ? (
								<Text size="sm" tone="muted">
									{trigger.description} It carries{" "}
									{trigger.data_fields.map((field, index) => (
										<Box as="span" key={field}>
											{index > 0 ? ", " : ""}
											<Box
												as="span"
												className="rounded bg-muted px-1 font-mono text-xs"
											>
												event.data.{field}
											</Box>
										</Box>
									))}
									.
								</Text>
							) : null}
						</Stack>
					</FlowNode>

					<FlowConnector />

					<FlowNode
						icon={Filter}
						eyebrow="Only if"
						title="Conditions on the event"
					>
						<ConditionEditor
							conditions={draft.definition.conditions}
							variables={triggerVariables(trigger)}
							onChange={(conditions) =>
								setDraft({
									...draft,
									definition: { ...draft.definition, conditions },
								})
							}
							emptyLabel="Every event of this type starts a run. Add a condition to narrow it; a step can also carry its own."
						/>
					</FlowNode>

					{draft.definition.steps.map((step, index) => (
						<Box key={step.id}>
							<FlowConnector />
							<StepEditor
								step={step}
								index={index}
								count={draft.definition.steps.length}
								catalog={catalog}
								variables={variablesBeforeStep(catalog, draft, index)}
								resources={resources}
								onChange={(next) =>
									setSteps(
										draft.definition.steps.map((current, position) =>
											position === index ? next : current,
										),
									)
								}
								onMove={(to) =>
									setSteps(moveItem(draft.definition.steps, index, to))
								}
								onRemove={() =>
									setSteps(
										draft.definition.steps.filter(
											(_, position) => position !== index,
										),
									)
								}
							/>
						</Box>
					))}

					<FlowConnector />
					<AddStepMenu
						actions={catalog.actions}
						onPick={(action) =>
							setSteps([
								...draft.definition.steps,
								newStep(draft.definition.steps, action, trigger),
							])
						}
					/>
				</Stack>
			</Column>

			<Column width="1/3">
				<Box className="space-y-6 lg:sticky lg:top-6">
					<Card>
						<CardHeader>
							<CardTitle>{workflow ? "Workflow" : "New workflow"}</CardTitle>
							<CardDescription>
								Runs as the product, after each matching event, at most once per
								event.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="lg">
								<Inline space="sm" alignY="center">
									<Switch
										id={ids.enabled}
										checked={draft.enabled}
										onCheckedChange={(enabled) =>
											setDraft({ ...draft, enabled })
										}
									/>
									<Label htmlFor={ids.enabled}>
										{draft.enabled
											? "Enabled: runs on every matching event"
											: "Disabled: runs only by hand"}
									</Label>
								</Inline>
								{saveErrors.length > 0 ? (
									<Alert tone="critical" icon={CircleAlert}>
										<AlertTitle>Not saved</AlertTitle>
										<AlertDescription>
											<Box as="ul" className="list-disc space-y-1 pl-4">
												{saveErrors.map((message) => (
													<Box as="li" key={message}>
														{message}
													</Box>
												))}
											</Box>
										</AlertDescription>
									</Alert>
								) : null}
								<Inline space="sm" wrap>
									<Button
										tone="brand"
										icon={Save}
										loading={create.isPending || update.isPending}
										disabled={!dirty && !!workflow}
										onClick={save}
									>
										{workflow ? "Save changes" : "Create workflow"}
									</Button>
									{workflow ? (
										<ConfirmDialog
											tone="critical"
											trigger={
												<Button variant="ghost" tone="critical" icon={Trash2}>
													Delete
												</Button>
											}
											title={`Delete “${workflow.name}”?`}
											description="Its run history goes with it. Runs already started finish."
											confirmLabel="Delete workflow"
											onConfirm={() =>
												remove.mutate({
													path: {
														product_id: productId,
														workflow_id: workflow.id,
													},
												})
											}
										/>
									) : null}
								</Inline>
								{dirty && workflow ? (
									<Text size="xs" tone="warning">
										You have unsaved changes.
									</Text>
								) : null}
							</Stack>
						</CardContent>
					</Card>
					<WorkflowTestPanel
						productId={productId}
						workflowId={workflow?.id}
						draft={draft}
						trigger={trigger}
						catalog={catalog}
						dirty={dirty}
					/>
				</Box>
			</Column>
		</Columns>
	);
}
