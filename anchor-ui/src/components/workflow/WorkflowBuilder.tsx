import type {
	WorkflowCatalogResponse,
	WorkflowResponse,
	WorkflowStep,
} from "@/client";
import { ProductEventGroupType } from "@/client";
import {
	createWorkflowMutation,
	deleteWorkflowMutation,
	getWorkflowCatalogQueryKey,
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
	FieldDescription,
	FieldError,
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
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
	CircleAlert,
	Filter,
	Repeat,
	Save,
	Trash2,
	TriangleAlert,
	Zap,
} from "lucide-react";
import { type ReactNode, useId, useMemo, useState } from "react";
import { AddStepMenu } from "./AddStepMenu";
import { ConditionEditor } from "./ConditionEditor";
import { StepEditor, type StepStart } from "./StepEditor";
import { WorkflowTestPanel } from "./WorkflowTestPanel";
import { useWorkflowResources } from "./useWorkflowResources";
import {
	CUSTOM_EVENT_PREFIX,
	type ChainWorkflow,
	type WorkflowDraft,
	customEventType,
	describeLocation,
	describeLoop,
	draftEmits,
	draftToRequest,
	findLoop,
	findTrigger,
	groupBy,
	isValidCustomEvent,
	moveItem,
	newStep,
	stepEmits,
	triggerVariables,
	variablesBeforeStep,
} from "./workflow-model";

const NEW_CUSTOM_EVENT = "__new_custom_event__";

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
	children: ReactNode;
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
					<Box as="span" className="block text-xs text-muted-foreground">
						{eyebrow}
					</Box>
					<Box as="span" className="block truncate text-sm font-semibold">
						{title}
					</Box>
				</Box>
			</Box>
			<Box className="px-4 py-4">{children}</Box>
		</Box>
	);
}

interface SaveProblem {
	message: string;
	detail: string;
	stepIndex?: number;
	param?: string;
}

function chainOf(workflow: WorkflowResponse): ChainWorkflow {
	return {
		id: workflow.id,
		name: workflow.name,
		enabled: workflow.enabled,
		trigger_event_type: workflow.trigger_event_type,
		emits: workflow.emits,
	};
}

export function WorkflowBuilder({
	productId,
	catalog,
	initialDraft,
	workflow,
	workflows,
	onSaved,
	onDeleted,
}: {
	productId: string;
	catalog: WorkflowCatalogResponse;
	initialDraft: WorkflowDraft;
	workflow?: WorkflowResponse;
	workflows: WorkflowResponse[];
	onSaved: (workflow: WorkflowResponse) => void;
	onDeleted?: () => void;
}) {
	const queryClient = useQueryClient();
	const ids = {
		name: useId(),
		description: useId(),
		trigger: useId(),
		customTrigger: useId(),
		enabled: useId(),
	};
	const [draft, setDraft] = useState<WorkflowDraft>(initialDraft);
	const [savedDraft, setSavedDraft] = useState<WorkflowDraft>(initialDraft);
	const [problems, setProblems] = useState<SaveProblem[]>([]);
	const [attempted, setAttempted] = useState(false);
	const customTriggers = catalog.triggers.filter(
		(item) => item.group_type === ProductEventGroupType.CUSTOM,
	);
	const [newCustomTrigger, setNewCustomTrigger] = useState(
		draft.trigger_event_type.startsWith(CUSTOM_EVENT_PREFIX) &&
			!customTriggers.some((item) => item.type === draft.trigger_event_type),
	);
	const resources = useWorkflowResources(
		productId,
		customTriggers.map((item) => ({
			value: item.type.slice(CUSTOM_EVENT_PREFIX.length),
			label: item.type,
		})),
	);
	const trigger = findTrigger(catalog, draft.trigger_event_type);
	const dirty = useMemo(
		() => JSON.stringify(draft) !== JSON.stringify(savedDraft),
		[draft, savedDraft],
	);

	const others = workflows.filter((other) => other.id !== workflow?.id);
	const candidate: ChainWorkflow = {
		id: workflow?.id ?? "new",
		name: draft.name.trim() || "This workflow",
		enabled: draft.enabled,
		trigger_event_type: draft.trigger_event_type,
		emits: draftEmits(catalog, draft),
	};
	const loop = findLoop(candidate, others.map(chainOf));
	const startsOf = (step: WorkflowStep): StepStart[] =>
		stepEmits(catalog, step).map((event) => ({
			event,
			workflowNames: [
				...(draft.enabled && draft.trigger_event_type === event
					? ["this workflow"]
					: []),
				...others
					.filter(
						(other) => other.enabled && other.trigger_event_type === event,
					)
					.map((other) => other.name),
			],
		}));
	const loopWarningFor = (step: WorkflowStep) =>
		loop && stepEmits(catalog, step).includes(loop[0].emits)
			? `${describeLoop(loop)}. Change this step, the trigger, or disable a workflow in the loop: Anchor will not save it.`
			: undefined;
	const startedBy = others.filter(
		(other) => other.enabled && other.emits.includes(draft.trigger_event_type),
	);

	const nameMissing = draft.name.trim().length < 2;
	const triggerMissing = draft.trigger_event_type === "";
	const customTriggerInvalid =
		newCustomTrigger && !isValidCustomEvent(draft.trigger_event_type);

	const setSteps = (steps: WorkflowStep[]) =>
		setDraft({ ...draft, definition: { ...draft.definition, steps } });

	const stepErrors = (index: number) =>
		Object.fromEntries(
			problems
				.filter((problem) => problem.stepIndex === index)
				.map((problem) => [problem.param ?? "", problem.detail]),
		);

	const invalidate = (saved?: WorkflowResponse) => {
		void queryClient.invalidateQueries({
			queryKey: listWorkflowsQueryKey({ path: { product_id: productId } }),
		});
		void queryClient.invalidateQueries({
			queryKey: getWorkflowCatalogQueryKey({ path: { product_id: productId } }),
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
		setProblems([]);
		setSavedDraft(draft);
		invalidate(saved);
		toast.add({ type: "success", title: `Workflow “${saved.name}” saved.` });
		onSaved(saved);
	};
	const onSaveError = (error: unknown) => {
		const found = getApiErrors(error).map((apiError): SaveProblem => {
			const location = String(
				apiError.metadata?.location ?? apiError.field ?? "",
			);
			if (!location) {
				return { message: apiError.message, detail: apiError.message };
			}
			const described = describeLocation(location, catalog, draft);
			const param = location.match(/\.params\.(\w+)/)?.[1];
			return {
				message:
					described.stepIndex === undefined
						? apiError.message
						: `${described.label}: ${apiError.message}`,
				detail: apiError.message,
				stepIndex: described.stepIndex,
				param,
			};
		});
		setProblems(
			found.length
				? found
				: [
						{
							message: "The workflow could not be saved.",
							detail: "The workflow could not be saved.",
						},
					],
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
		setAttempted(true);
		if (nameMissing || triggerMissing || customTriggerInvalid) {
			setProblems([]);
			return;
		}
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

	const catalogGroups = groupBy(
		catalog.triggers.filter(
			(item) => item.group_type !== ProductEventGroupType.CUSTOM,
		),
		(item) => item.group_name,
	);
	const triggerSelectValue = newCustomTrigger
		? NEW_CUSTOM_EVENT
		: draft.trigger_event_type;

	return (
		<Box className="grid grid-cols-1 items-start gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
			<Box className="order-2 lg:order-none">
				<Stack space="none">
					<Card>
						<CardContent>
							<FieldGroup>
								<Field invalid={attempted && nameMissing}>
									<FieldLabel htmlFor={ids.name}>Name</FieldLabel>
									<Input
										id={ids.name}
										placeholder="Default workspace for new organizations"
										aria-invalid={(attempted && nameMissing) || undefined}
										value={draft.name}
										onChange={(event) =>
											setDraft({ ...draft, name: event.target.value })
										}
									/>
									{attempted && nameMissing ? (
										<FieldError>
											Give the workflow a name of at least 2 characters.
										</FieldError>
									) : null}
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
						title={
							trigger?.name ?? (draft.trigger_event_type || "Pick a trigger")
						}
					>
						<Stack space="sm">
							<Field invalid={attempted && triggerMissing}>
								<FieldLabel htmlFor={ids.trigger} size="sm">
									Event
								</FieldLabel>
								<NativeSelect
									id={ids.trigger}
									aria-invalid={(attempted && triggerMissing) || undefined}
									value={triggerSelectValue}
									onChange={(event) => {
										const value = event.target.value;
										setNewCustomTrigger(value === NEW_CUSTOM_EVENT);
										setDraft({
											...draft,
											trigger_event_type:
												value === NEW_CUSTOM_EVENT
													? CUSTOM_EVENT_PREFIX
													: value,
										});
									}}
								>
									<NativeSelectOption value="" disabled>
										Choose the event that starts this workflow
									</NativeSelectOption>
									{catalogGroups.map(([group, triggers]) => (
										<NativeSelectOptGroup key={group} label={group}>
											{triggers.map((item) => (
												<NativeSelectOption key={item.type} value={item.type}>
													{item.name} — {item.type}
												</NativeSelectOption>
											))}
										</NativeSelectOptGroup>
									))}
									<NativeSelectOptGroup label="Custom events from your workflows">
										{customTriggers.map((item) => (
											<NativeSelectOption key={item.type} value={item.type}>
												{item.type}
											</NativeSelectOption>
										))}
										<NativeSelectOption value={NEW_CUSTOM_EVENT}>
											New custom event…
										</NativeSelectOption>
									</NativeSelectOptGroup>
								</NativeSelect>
								{attempted && triggerMissing ? (
									<FieldError>
										Pick the event that starts this workflow.
									</FieldError>
								) : null}
							</Field>
							{newCustomTrigger ? (
								<Field invalid={attempted && customTriggerInvalid}>
									<FieldLabel htmlFor={ids.customTrigger} size="sm">
										Custom event name
									</FieldLabel>
									<Input
										id={ids.customTrigger}
										size="sm"
										font="mono"
										placeholder="onboarding.started"
										aria-invalid={
											(attempted && customTriggerInvalid) || undefined
										}
										value={draft.trigger_event_type.slice(
											CUSTOM_EVENT_PREFIX.length,
										)}
										onChange={(event) =>
											setDraft({
												...draft,
												trigger_event_type: customEventType(event.target.value),
											})
										}
									/>
									<FieldDescription>
										Another workflow starts this one with a “Start other
										workflows” step that emits this name.
									</FieldDescription>
									{attempted && customTriggerInvalid ? (
										<FieldError>
											Use lowercase words joined by dots, such as
											onboarding.started.
										</FieldError>
									) : null}
								</Field>
							) : null}
							{trigger && trigger.data_fields.length > 0 ? (
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
							{startedBy.length > 0 ? (
								<Text size="xs" tone="muted">
									Started after{" "}
									{startedBy.map((other) => `“${other.name}”`).join(", ")}.
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
							label="Event condition"
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
								errors={stepErrors(index)}
								starts={startsOf(step)}
								loopWarning={loopWarningFor(step)}
								onChange={(next) => {
									setProblems((current) =>
										current.filter(
											(problem) =>
												problem.stepIndex !== index ||
												(problem.param !== undefined &&
													next.params[problem.param] ===
														step.params[problem.param]),
										),
									);
									setSteps(
										draft.definition.steps.map((current, position) =>
											position === index ? next : current,
										),
									);
								}}
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
			</Box>

			<Box className="contents lg:block lg:space-y-6">
				<Box className="order-1 lg:order-none">
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
								<Field orientation="horizontal">
									<Switch
										id={ids.enabled}
										checked={draft.enabled}
										onCheckedChange={(enabled) =>
											setDraft({ ...draft, enabled })
										}
									/>
									<Stack space="none">
										<Label htmlFor={ids.enabled}>Enabled</Label>
										<Text size="xs" tone="muted">
											{draft.enabled
												? "Runs on every matching event."
												: "Runs only by hand."}
										</Text>
									</Stack>
								</Field>
								{loop && problems.length === 0 ? (
									<Alert tone="warning" icon={Repeat}>
										<AlertTitle>This would loop</AlertTitle>
										<AlertDescription>
											<Box className="[overflow-wrap:anywhere]">
												{describeLoop(loop)}. Anchor refuses to save an enabled
												workflow that can start itself again: change a trigger
												or a step, or disable a workflow in the loop.
											</Box>
										</AlertDescription>
									</Alert>
								) : null}
								{problems.length > 0 ? (
									<Alert tone="critical" icon={CircleAlert}>
										<AlertTitle>Not saved</AlertTitle>
										<AlertDescription>
											<Box
												as="ul"
												className="list-disc space-y-1 pl-4 [overflow-wrap:anywhere]"
											>
												{problems.map((problem) => (
													<Box as="li" key={problem.message}>
														{problem.message}
													</Box>
												))}
											</Box>
										</AlertDescription>
									</Alert>
								) : null}
								{attempted &&
								(nameMissing || triggerMissing || customTriggerInvalid) ? (
									<Alert tone="critical" icon={TriangleAlert}>
										<AlertDescription>
											Fix the highlighted fields before saving.
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
				</Box>
				<Box className="order-3 lg:order-none">
					<WorkflowTestPanel
						productId={productId}
						workflowId={workflow?.id}
						draft={draft}
						trigger={trigger}
						catalog={catalog}
						dirty={dirty}
					/>
				</Box>
			</Box>
		</Box>
	);
}
