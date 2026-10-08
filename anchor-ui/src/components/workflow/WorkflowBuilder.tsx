import type {
	WorkflowActionResponse,
	WorkflowCatalogResponse,
	WorkflowResponse,
	WorkflowRunResponse,
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
import { useIsMobile } from "@/hooks/use-mobile";
import { getApiErrors } from "@/lib/api-error";
import { ConfirmDialog } from "@nanostackorg/design-system/blocks/confirm-dialog";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
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
import { ScrollArea } from "@nanostackorg/design-system/components/scroll-area";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { Text } from "@nanostackorg/design-system/components/text";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import {
	ArrowLeft,
	CircleAlert,
	Filter,
	Repeat,
	Save,
	Trash2,
	X,
	Zap,
} from "lucide-react";
import { AnimatePresence, motion, useReducedMotion } from "motion/react";
import {
	type ReactNode,
	useCallback,
	useEffect,
	useId,
	useMemo,
	useRef,
	useState,
} from "react";
import { ConditionEditor } from "./ConditionEditor";
import { StepEditor, type StepStart } from "./StepEditor";
import { WorkflowTestPanel } from "./WorkflowTestPanel";
import { WorkflowCanvas } from "./canvas/WorkflowCanvas";
import { easeOut } from "./canvas/motion";
import {
	type RunStatusByStep,
	type Selection,
	buildWorkflowGraph,
	selectionNodeId,
} from "./canvas/workflow-graph";
import { fieldTypeIcons } from "./field-icons";
import { fieldTypeLabels } from "./fields";
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
	findAction,
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
const PLAYBACK_STEP_MS = 220;
const SIDE_BY_SIDE = "(min-width: 64rem)";

interface SaveProblem {
	message: string;
	detail: string;
	stepId?: string;
	param?: string;
	field?: string;
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

function selectionKey(selection: Selection) {
	return selection.kind === "step"
		? `step:${selection.stepId}`
		: selection.kind;
}

function InspectorHeader({
	eyebrow,
	title,
	icon: Icon,
	onClose,
	closeLabel,
}: {
	eyebrow: string;
	title: string;
	icon: typeof Zap;
	onClose: () => void;
	closeLabel: string;
}) {
	return (
		<Box className="flex items-center gap-3 border-b border-border px-4 py-3">
			<Box className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted text-foreground">
				<Icon className="size-4" aria-hidden />
			</Box>
			<Box className="min-w-0 flex-1">
				<Box
					as="span"
					className="block text-[11px] font-medium uppercase tracking-wide text-muted-foreground"
				>
					{eyebrow}
				</Box>
				<Box as="span" className="block truncate text-sm font-semibold">
					{title}
				</Box>
			</Box>
			<Button
				variant="ghost"
				size="sm"
				icon={closeLabel === "Back to flow" ? ArrowLeft : X}
				onClick={onClose}
			>
				{closeLabel}
			</Button>
		</Box>
	);
}

function InspectorTransition({
	selection,
	children,
}: {
	selection: Selection;
	children: ReactNode;
}) {
	const reduceMotion = useReducedMotion();
	return (
		<AnimatePresence mode="wait" initial={false}>
			<motion.div
				key={selectionKey(selection)}
				initial={
					reduceMotion
						? { opacity: 0 }
						: { opacity: 0, transform: "translateX(6px)", filter: "blur(2px)" }
				}
				animate={
					reduceMotion
						? { opacity: 1 }
						: { opacity: 1, transform: "translateX(0px)", filter: "blur(0px)" }
				}
				exit={{ opacity: 0, transition: { duration: 0.08 } }}
				transition={{ duration: 0.16, ease: easeOut }}
			>
				{children}
			</motion.div>
		</AnimatePresence>
	);
}

export function WorkflowBuilder({
	productId,
	catalog,
	initialDraft,
	workflow,
	workflows,
	onSaved,
	onDeleted,
	onOpenWorkflow,
}: {
	productId: string;
	catalog: WorkflowCatalogResponse;
	initialDraft: WorkflowDraft;
	workflow?: WorkflowResponse;
	workflows: WorkflowResponse[];
	onSaved: (workflow: WorkflowResponse) => void;
	onDeleted?: () => void;
	onOpenWorkflow?: (workflowId: string) => void;
}) {
	const queryClient = useQueryClient();
	const isMobile = useIsMobile();
	const reduceMotion = useReducedMotion();
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
	const [selection, setSelection] = useState<Selection>({ kind: "workflow" });
	const canvasRef = useRef<HTMLDivElement>(null);
	const inspectorRef = useRef<HTMLDivElement>(null);
	const [reveal, setReveal] = useState<
		{ target: "inspector" } | { target: "canvas"; nodeId?: string } | null
	>(null);
	const inspect = useCallback((next: Selection) => {
		setSelection(next);
		setReveal({ target: "inspector" });
	}, []);
	useEffect(() => {
		if (!reveal) return;
		const area =
			reveal.target === "canvas" ? canvasRef.current : inspectorRef.current;
		if (!window.matchMedia(SIDE_BY_SIDE).matches) {
			area?.scrollIntoView({
				block: "nearest",
				behavior: reduceMotion ? "auto" : "smooth",
			});
		}
		let frame = 0;
		let attempts = 0;
		const focusWhenRendered = () => {
			const target =
				reveal.target === "canvas"
					? area?.querySelector<HTMLElement>(
							`[data-workflow-node="${reveal.nodeId}"]`,
						)
					: area;
			target?.focus({ preventScroll: true });
			if (document.activeElement !== target && attempts++ < 30)
				frame = requestAnimationFrame(focusWhenRendered);
		};
		focusWhenRendered();
		return () => cancelAnimationFrame(frame);
	}, [reveal, reduceMotion]);
	const [playback, setPlayback] = useState<{
		statuses: RunStatusByStep;
		revealed: number;
	} | null>(null);
	const playbackTimer = useRef<number | null>(null);

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
	const dirtyRef = useRef(dirty);
	dirtyRef.current = dirty;
	const leaveGuard = useBlocker({
		shouldBlockFn: ({ current, next }) =>
			dirtyRef.current && current.pathname !== next.pathname,
		enableBeforeUnload: false,
		withResolver: true,
	});

	const others = useMemo(
		() => workflows.filter((other) => other.id !== workflow?.id),
		[workflows, workflow?.id],
	);
	const otherChains = useMemo(() => others.map(chainOf), [others]);
	const draftName = draft.name.trim() || "This workflow";
	const emitted = draftEmits(catalog, draft).join("\n");
	const loop = useMemo(
		() =>
			findLoop(
				{
					id: workflow?.id ?? "new",
					name: draftName,
					enabled: draft.enabled,
					trigger_event_type: draft.trigger_event_type,
					emits: emitted ? emitted.split("\n") : [],
				},
				otherChains,
			),
		[
			workflow?.id,
			draftName,
			draft.enabled,
			draft.trigger_event_type,
			emitted,
			otherChains,
		],
	);

	const nameMissing = draft.name.trim().length < 2;
	const triggerMissing = draft.trigger_event_type === "";
	const customTriggerInvalid =
		newCustomTrigger && !isValidCustomEvent(draft.trigger_event_type);

	const problemsByStep = useMemo(() => {
		const counts: Record<string, number> = {};
		for (const problem of problems) {
			if (problem.stepId !== undefined)
				counts[problem.stepId] = (counts[problem.stepId] ?? 0) + 1;
		}
		return counts;
	}, [problems]);

	const stopPlayback = useCallback(() => {
		if (playbackTimer.current !== null)
			window.clearInterval(playbackTimer.current);
		playbackTimer.current = null;
	}, []);
	useEffect(() => stopPlayback, [stopPlayback]);

	const changeDraft = (next: WorkflowDraft) => {
		stopPlayback();
		setPlayback(null);
		setDraft(next);
	};

	const playRun = (run: WorkflowRunResponse | null) => {
		stopPlayback();
		if (!run) {
			setPlayback(null);
			return;
		}
		const statuses = Object.fromEntries(
			run.steps.map((step) => [step.step_id, step.status]),
		);
		const total = draft.definition.steps.length;
		if (reduceMotion) {
			setPlayback({ statuses, revealed: total });
			return;
		}
		setPlayback({ statuses, revealed: 0 });
		playbackTimer.current = window.setInterval(() => {
			setPlayback((current) => {
				if (!current) return current;
				if (current.revealed >= total) {
					stopPlayback();
					return current;
				}
				return { ...current, revealed: current.revealed + 1 };
			});
		}, PLAYBACK_STEP_MS);
	};

	const graph = useMemo(
		() =>
			buildWorkflowGraph({
				draft,
				catalog,
				selection,
				others: otherChains,
				loop,
				problemsByStep,
				triggerInvalid: attempted && (triggerMissing || customTriggerInvalid),
				run: playback ?? undefined,
			}),
		[
			draft,
			catalog,
			selection,
			otherChains,
			loop,
			problemsByStep,
			attempted,
			triggerMissing,
			customTriggerInvalid,
			playback,
		],
	);

	const setSteps = (steps: WorkflowStep[]) =>
		changeDraft({ ...draft, definition: { ...draft.definition, steps } });

	const insertStep = (index: number, action: WorkflowActionResponse) => {
		const steps = draft.definition.steps;
		const step = newStep(steps, action, trigger);
		setSteps([...steps.slice(0, index), step, ...steps.slice(index)]);
		inspect({ kind: "step", stepId: step.id });
	};

	const insertStepRef = useRef(insertStep);
	insertStepRef.current = insertStep;
	const canvasActions = useMemo(
		() => ({
			actions: catalog.actions,
			select: (next: Selection) =>
				next.kind === "workflow" ? setSelection(next) : inspect(next),
			insertStep: (index: number, action: WorkflowActionResponse) =>
				insertStepRef.current(index, action),
			openWorkflow: onOpenWorkflow,
		}),
		[catalog.actions, inspect, onOpenWorkflow],
	);

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

	const stepErrors = (stepId: string) =>
		Object.fromEntries(
			problems
				.filter((problem) => problem.stepId === stepId)
				.map((problem) => [problem.param ?? "", problem.detail]),
		);
	const stepFieldErrors = (stepId: string) =>
		Object.fromEntries(
			problems
				.filter((problem) => problem.stepId === stepId && problem.field)
				.map((problem) => [problem.field ?? "", problem.detail]),
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
		dirtyRef.current = false;
		onSaved(saved);
	};
	const onSaveError = (error: unknown) => {
		const found = getApiErrors(error).map((apiError): SaveProblem => {
			const location = String(
				apiError.metadata?.location ?? apiError.field ?? "",
			);
			if (!location)
				return { message: apiError.message, detail: apiError.message };
			const described = describeLocation(location, catalog, draft);
			return {
				message:
					described.stepIndex === undefined
						? apiError.message
						: `${described.label}: ${apiError.message}`,
				detail: apiError.message,
				stepId:
					described.stepIndex === undefined
						? undefined
						: draft.definition.steps[described.stepIndex]?.id,
				param: location.match(/\.params\.(\w+)/)?.[1],
				field:
					typeof apiError.metadata?.field === "string"
						? apiError.metadata.field
						: undefined,
			};
		});
		const next = found.length
			? found
			: [
					{
						message: "The workflow could not be saved.",
						detail: "The workflow could not be saved.",
					},
				];
		setProblems(next);
		const firstStepId = next.find(
			(problem) => problem.stepId !== undefined,
		)?.stepId;
		inspect(
			firstStepId
				? { kind: "step", stepId: firstStepId }
				: { kind: "workflow" },
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
			dirtyRef.current = false;
			onDeleted?.();
		},
		onError: () =>
			toast.add({ type: "error", title: "The workflow could not be deleted." }),
	});

	const save = () => {
		setAttempted(true);
		if (nameMissing || customTriggerInvalid) {
			setProblems([]);
			inspect({ kind: "workflow" });
			return;
		}
		if (triggerMissing) {
			setProblems([]);
			inspect({ kind: "trigger" });
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

	const closeLabel = isMobile ? "Back to flow" : "Close";
	const backToWorkflow = () => {
		setSelection({ kind: "workflow" });
		setReveal({ target: "canvas", nodeId: selectionNodeId(selection) });
	};
	const catalogGroups = groupBy(
		catalog.triggers.filter(
			(item) => item.group_type !== ProductEventGroupType.CUSTOM,
		),
		(item) => item.group_name,
	);

	const workflowPanel = (
		<Stack space="none">
			<Box className="space-y-4 px-4 py-4">
				<FieldGroup>
					<Field invalid={attempted && nameMissing}>
						<FieldLabel htmlFor={ids.name}>Name</FieldLabel>
						<Input
							id={ids.name}
							placeholder="Default workspace for new organizations"
							aria-invalid={(attempted && nameMissing) || undefined}
							value={draft.name}
							onChange={(event) =>
								changeDraft({ ...draft, name: event.target.value })
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
								changeDraft({ ...draft, description: event.target.value })
							}
						/>
					</Field>
				</FieldGroup>
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
				{loop && problems.length === 0 ? (
					<Alert tone="warning" icon={Repeat}>
						<AlertTitle>This would loop</AlertTitle>
						<AlertDescription>
							<Box className="[overflow-wrap:anywhere]">
								{describeLoop(loop)}. Anchor refuses to save an enabled workflow
								that can start itself again: change a trigger or a step, or
								disable a workflow in the loop.
							</Box>
						</AlertDescription>
					</Alert>
				) : null}
			</Box>
			<Box className="border-t border-border px-4 py-4">
				<WorkflowTestPanel
					productId={productId}
					workflowId={workflow?.id}
					draft={draft}
					trigger={trigger}
					catalog={catalog}
					dirty={dirty}
					onResult={playRun}
					embedded
				/>
			</Box>
		</Stack>
	);

	const triggerPanel = (
		<Stack space="none">
			<InspectorHeader
				eyebrow="When"
				title={trigger?.name ?? (draft.trigger_event_type || "Pick a trigger")}
				icon={Zap}
				onClose={backToWorkflow}
				closeLabel={closeLabel}
			/>
			<Box className="space-y-3 px-4 py-4">
				<Field invalid={attempted && triggerMissing}>
					<FieldLabel htmlFor={ids.trigger} size="sm">
						Event
					</FieldLabel>
					<NativeSelect
						id={ids.trigger}
						aria-invalid={(attempted && triggerMissing) || undefined}
						value={
							newCustomTrigger ? NEW_CUSTOM_EVENT : draft.trigger_event_type
						}
						onChange={(event) => {
							const value = event.target.value;
							setNewCustomTrigger(value === NEW_CUSTOM_EVENT);
							changeDraft({
								...draft,
								trigger_event_type:
									value === NEW_CUSTOM_EVENT ? CUSTOM_EVENT_PREFIX : value,
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
						<FieldError>Pick the event that starts this workflow.</FieldError>
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
							aria-invalid={(attempted && customTriggerInvalid) || undefined}
							value={draft.trigger_event_type.slice(CUSTOM_EVENT_PREFIX.length)}
							onChange={(event) =>
								changeDraft({
									...draft,
									trigger_event_type: customEventType(event.target.value),
								})
							}
						/>
						<FieldDescription>
							Another workflow starts this one with a “Start other workflows”
							step that emits this name.
						</FieldDescription>
						{attempted && customTriggerInvalid ? (
							<FieldError>
								Use lowercase words joined by dots, such as onboarding.started.
							</FieldError>
						) : null}
					</Field>
				) : null}
				{trigger ? (
					<Text size="sm" tone="muted">
						{trigger.description}
					</Text>
				) : null}
				{trigger && trigger.fields.length > 0 ? (
					<Box
						as="ul"
						aria-label="Fields the event carries"
						className="space-y-1"
					>
						{trigger.fields.map((field) => {
							const FieldIcon = fieldTypeIcons[field.type];
							return (
								<Box
									as="li"
									key={field.name}
									title={field.description}
									className="flex items-center gap-2 text-xs"
								>
									<FieldIcon
										className="size-3.5 shrink-0 text-muted-foreground"
										aria-hidden
									/>
									<Box as="span" className="break-all font-mono">
										event.data.{field.name}
									</Box>
									<Box as="span" className="shrink-0 text-muted-foreground">
										{fieldTypeLabels[field.type]}
									</Box>
								</Box>
							);
						})}
					</Box>
				) : null}
				{startedBy.length > 0 ? (
					<Text size="xs" tone="muted">
						Started after{" "}
						{startedBy.map((other) => `“${other.name}”`).join(", ")}.
					</Text>
				) : null}
			</Box>
		</Stack>
	);

	const conditionsPanel = (
		<Stack space="none">
			<InspectorHeader
				eyebrow="Only if"
				title="Conditions on the event"
				icon={Filter}
				onClose={backToWorkflow}
				closeLabel={closeLabel}
			/>
			<Box className="px-4 py-4">
				<ConditionEditor
					label="Event condition"
					conditions={draft.definition.conditions}
					variables={triggerVariables(trigger)}
					resources={resources}
					onChange={(conditions) =>
						changeDraft({
							...draft,
							definition: { ...draft.definition, conditions },
						})
					}
					emptyLabel="Every event of this type starts a run. Add a condition to narrow it; a step can also carry its own."
				/>
			</Box>
		</Stack>
	);

	const selectedIndex =
		selection.kind === "step"
			? draft.definition.steps.findIndex((step) => step.id === selection.stepId)
			: -1;
	const selectedStep =
		selectedIndex >= 0 ? draft.definition.steps[selectedIndex] : undefined;
	const stepPanel = selectedStep ? (
		<Stack space="none">
			<StepEditor
				onClose={backToWorkflow}
				closeLabel={closeLabel}
				step={selectedStep}
				index={selectedIndex}
				count={draft.definition.steps.length}
				catalog={catalog}
				variables={variablesBeforeStep(catalog, draft, selectedIndex)}
				resources={resources}
				errors={stepErrors(selectedStep.id)}
				fieldErrors={stepFieldErrors(selectedStep.id)}
				starts={startsOf(selectedStep)}
				loopWarning={loopWarningFor(selectedStep)}
				onChange={(next) => {
					setProblems((current) =>
						current.filter(
							(problem) =>
								problem.stepId !== selectedStep.id ||
								(problem.param !== undefined &&
									next.params[problem.param] ===
										selectedStep.params[problem.param]),
						),
					);
					setSteps(
						draft.definition.steps.map((current, position) =>
							position === selectedIndex ? next : current,
						),
					);
				}}
				onMove={(to) =>
					setSteps(moveItem(draft.definition.steps, selectedIndex, to))
				}
				onRemove={() => {
					setSteps(
						draft.definition.steps.filter(
							(_, position) => position !== selectedIndex,
						),
					);
					backToWorkflow();
				}}
			/>
		</Stack>
	) : (
		workflowPanel
	);

	const inspector =
		selection.kind === "trigger"
			? triggerPanel
			: selection.kind === "conditions"
				? conditionsPanel
				: selection.kind === "step"
					? stepPanel
					: workflowPanel;
	const inspecting = selection.kind !== "workflow";
	const inspectorTitle =
		selection.kind === "workflow"
			? "Workflow settings"
			: selection.kind === "step" && selectedStep
				? `Step ${selectedIndex + 1}: ${findAction(catalog, selectedStep.action)?.name ?? selectedStep.action}`
				: selection.kind === "trigger"
					? "Trigger settings"
					: "Condition settings";

	return (
		<Stack space="md">
			<Box
				role="toolbar"
				aria-label="Workflow actions"
				className="flex flex-wrap items-center gap-3 rounded-xl border border-border bg-card px-3 py-2 shadow-xs"
			>
				<Inline space="sm" alignY="center">
					<Switch
						id={ids.enabled}
						checked={draft.enabled}
						onCheckedChange={(enabled) => changeDraft({ ...draft, enabled })}
					/>
					<Label htmlFor={ids.enabled}>Enabled</Label>
					<Box as="span" className="hidden sm:inline">
						<Text as="span" size="xs" tone="muted">
							{draft.enabled
								? "Runs on every matching event."
								: "Runs only by hand."}
						</Text>
					</Box>
				</Inline>
				{loop ? (
					<Badge variant="soft" tone="critical" icon={Repeat}>
						Loop
					</Badge>
				) : null}
				<Box className="ml-auto flex flex-wrap items-center gap-2">
					{dirty && workflow ? (
						<Text as="span" size="xs" tone="warning">
							Unsaved changes
						</Text>
					) : null}
					<ConfirmDialog
						open={leaveGuard.status === "blocked"}
						onOpenChange={(open) => {
							if (!open) leaveGuard.reset?.();
						}}
						tone="critical"
						title="Leave without saving?"
						description="Your changes to this workflow are not saved. Leaving discards them."
						confirmLabel="Discard and leave"
						cancelLabel="Stay"
						onConfirm={() => leaveGuard.proceed?.()}
					/>
					{workflow ? (
						<ConfirmDialog
							tone="critical"
							trigger={
								<Button variant="ghost" tone="critical" size="sm" icon={Trash2}>
									Delete
								</Button>
							}
							title={`Delete “${workflow.name}”?`}
							description="Its run history goes with it. Runs already started finish."
							confirmLabel="Delete workflow"
							onConfirm={() =>
								remove.mutate({
									path: { product_id: productId, workflow_id: workflow.id },
								})
							}
						/>
					) : null}
					<Button
						tone="brand"
						size="sm"
						icon={Save}
						loading={create.isPending || update.isPending}
						disabled={!dirty && !!workflow}
						onClick={save}
					>
						{workflow ? "Save changes" : "Create workflow"}
					</Button>
				</Box>
				{attempted &&
				(nameMissing || triggerMissing || customTriggerInvalid) ? (
					<Box className="w-full">
						<Text size="xs" tone="critical">
							Fix the highlighted fields before saving.
						</Text>
					</Box>
				) : null}
			</Box>

			<Box className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,400px)]">
				<Box
					ref={canvasRef}
					className={
						isMobile && inspecting
							? "hidden"
							: "h-[58vh] lg:h-[min(76vh,820px)]"
					}
				>
					<WorkflowCanvas
						nodes={graph.nodes}
						edges={graph.edges}
						fitKey={`${draft.definition.steps.length}:${draft.trigger_event_type}`}
						focusNodeId={selectionNodeId(selection)}
						actions={canvasActions}
					/>
				</Box>
				<Box
					ref={inspectorRef}
					as="section"
					tabIndex={-1}
					aria-label={inspectorTitle}
					className="flex flex-col overflow-hidden rounded-xl border border-border bg-card shadow-xs outline-none lg:h-[min(76vh,820px)]"
				>
					<ScrollArea height="fill">
						<InspectorTransition selection={selection}>
							{inspector}
						</InspectorTransition>
					</ScrollArea>
				</Box>
			</Box>
		</Stack>
	);
}
