import { WorkflowStepStatus } from "@/client";
import { cn } from "@/lib/utils";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Handle, type NodeProps, Position } from "@xyflow/react";
import {
	ArrowDownToLine,
	ArrowUpRight,
	CircleAlert,
	CircleCheck,
	CircleSlash,
	CircleX,
	Filter,
	FlaskConical,
	GripVertical,
	Plus,
	Zap,
} from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { type ReactNode, useId, useState } from "react";
import { AddStepMenu } from "../AddStepMenu";
import { actionGroupIcon, actionGroupTone } from "../action-icons";
import { carriesAction, useWorkflowCanvas } from "./canvas-context";
import { EASE_OUT_CLASS, easeOut } from "./motion";
import {
	type AddNodeData,
	COLUMN_GAP,
	type ConditionsNodeData,
	NODE_WIDTH,
	STEP_DRAG_HANDLE,
	type StepNodeData,
	type TriggerNodeData,
	type WorkflowRefNodeData,
} from "./workflow-graph";

const hiddenHandle = "!pointer-events-none !opacity-0";

function Handles({
	top = false,
	bottom = false,
	right,
	left = false,
}: {
	top?: boolean;
	bottom?: boolean;
	right?: "source" | "target";
	left?: boolean;
}) {
	return (
		<>
			{top ? (
				<Handle
					id="top"
					type="target"
					position={Position.Top}
					isConnectable={false}
					className={hiddenHandle}
				/>
			) : null}
			{bottom ? (
				<Handle
					id="bottom"
					type="source"
					position={Position.Bottom}
					isConnectable={false}
					className={hiddenHandle}
				/>
			) : null}
			{right ? (
				<Handle
					id="right"
					type={right}
					position={Position.Right}
					isConnectable={false}
					className={hiddenHandle}
				/>
			) : null}
			{left ? (
				<Handle
					id="left"
					type="target"
					position={Position.Left}
					isConnectable={false}
					className={hiddenHandle}
				/>
			) : null}
		</>
	);
}

function Entrance({ order, children }: { order: number; children: ReactNode }) {
	const { entranceDelay } = useWorkflowCanvas();
	const reduceMotion = useReducedMotion();
	return (
		<motion.div
			initial={
				reduceMotion
					? { opacity: 0 }
					: { opacity: 0, transform: "translateY(6px) scale(0.98)" }
			}
			animate={
				reduceMotion
					? { opacity: 1 }
					: { opacity: 1, transform: "translateY(0px) scale(1)" }
			}
			transition={{
				duration: 0.22,
				ease: easeOut,
				delay: entranceDelay(order),
			}}
			className="relative"
			style={{ width: NODE_WIDTH }}
		>
			{children}
		</motion.div>
	);
}

const statusTone: Record<WorkflowStepStatus, string> = {
	[WorkflowStepStatus.SUCCEEDED]: "border-success/60 ring-2 ring-success/15",
	[WorkflowStepStatus.FAILED]:
		"border-destructive/70 ring-2 ring-destructive/15",
	[WorkflowStepStatus.SIMULATED]: "border-info/60 ring-2 ring-info/15",
	[WorkflowStepStatus.SKIPPED]: "border-dashed opacity-70",
};

const statusBadge: Record<
	WorkflowStepStatus,
	{ label: string; icon: typeof CircleCheck; className: string }
> = {
	[WorkflowStepStatus.SUCCEEDED]: {
		label: "Succeeded",
		icon: CircleCheck,
		className: "bg-success/12 text-success-on-tint",
	},
	[WorkflowStepStatus.FAILED]: {
		label: "Failed",
		icon: CircleX,
		className: "bg-destructive/10 text-destructive-on-tint",
	},
	[WorkflowStepStatus.SIMULATED]: {
		label: "Simulated",
		icon: FlaskConical,
		className: "bg-info/12 text-info-on-tint",
	},
	[WorkflowStepStatus.SKIPPED]: {
		label: "Skipped",
		icon: CircleSlash,
		className: "bg-muted text-muted-foreground",
	},
};

const cardBase =
	"nodrag nopan group relative flex w-full cursor-pointer flex-col gap-2 rounded-xl border bg-card px-3.5 py-3 text-left shadow-xs outline-none " +
	"transition-[transform,box-shadow,border-color] duration-150 ease-out active:scale-[0.98] " +
	"hover:border-border-strong hover:shadow-sm focus-visible:ring-2 focus-visible:ring-ring/60 motion-reduce:active:scale-100";

function Eyebrow({ children }: { children: ReactNode }) {
	return (
		<Box
			as="span"
			className="block text-[11px] font-medium uppercase tracking-wide text-muted-foreground"
		>
			{children}
		</Box>
	);
}

function Tile({
	icon: Icon,
	className,
}: { icon: typeof Zap; className: string }) {
	return (
		<Box
			as="span"
			className={cn(
				"flex size-8 shrink-0 items-center justify-center rounded-lg",
				className,
			)}
		>
			<Icon className="size-4" aria-hidden />
		</Box>
	);
}

export function TriggerNode({
	id,
	data,
}: NodeProps & { data: TriggerNodeData }) {
	const { select } = useWorkflowCanvas();
	return (
		<Entrance order={data.order}>
			<Handles bottom top right="target" />
			<button
				type="button"
				data-workflow-node={id}
				aria-label={`Trigger: ${data.title}`}
				aria-pressed={data.selected}
				onClick={() => select({ kind: "trigger" })}
				className={cn(
					cardBase,
					data.selected
						? "border-primary shadow-md ring-2 ring-primary/25"
						: "border-border",
					data.invalid && "border-destructive ring-2 ring-destructive/20",
				)}
			>
				<Box as="span" className="flex items-center gap-3">
					<Tile icon={Zap} className="bg-primary text-primary-foreground" />
					<Box as="span" className="min-w-0">
						<Eyebrow>When</Eyebrow>
						<Box as="span" className="block truncate text-sm font-semibold">
							{data.title}
						</Box>
					</Box>
				</Box>
				{data.dataFields.length > 0 ? (
					<Box as="span" className="flex flex-wrap gap-1">
						{data.dataFields.map((field) => (
							<Box
								as="span"
								key={field}
								className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground"
							>
								{field}
							</Box>
						))}
					</Box>
				) : null}
			</button>
		</Entrance>
	);
}

export function ConditionsNode({
	id,
	data,
}: NodeProps & { data: ConditionsNodeData }) {
	const { select } = useWorkflowCanvas();
	return (
		<Entrance order={data.order}>
			<Handles top bottom />
			<button
				type="button"
				data-workflow-node={id}
				aria-label="Conditions on the event"
				aria-pressed={data.selected}
				onClick={() => select({ kind: "conditions" })}
				className={cn(
					cardBase,
					data.count === 0 && !data.selected && "border-dashed bg-card/70",
					data.selected
						? "border-primary shadow-md ring-2 ring-primary/25"
						: "border-border",
				)}
			>
				<Box as="span" className="flex items-center gap-3">
					<Tile icon={Filter} className="bg-muted text-foreground" />
					<Box as="span" className="min-w-0 flex-1">
						<Eyebrow>Only if</Eyebrow>
						<Box as="span" className="block truncate text-sm text-foreground">
							{data.summary}
						</Box>
					</Box>
					{data.count > 0 ? (
						<Box
							as="span"
							className="rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary tabular-nums"
						>
							{data.count}
						</Box>
					) : null}
				</Box>
			</button>
		</Entrance>
	);
}

function StatusBadge({ status }: { status: WorkflowStepStatus }) {
	const reduceMotion = useReducedMotion();
	const { label, icon: Icon, className } = statusBadge[status];
	return (
		<motion.span
			key={status}
			initial={
				reduceMotion ? { opacity: 0 } : { opacity: 0, transform: "scale(0.9)" }
			}
			animate={
				reduceMotion ? { opacity: 1 } : { opacity: 1, transform: "scale(1)" }
			}
			transition={{ duration: 0.2, ease: easeOut }}
			className={cn(
				"inline-flex shrink-0 items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium",
				className,
			)}
		>
			<Icon className="size-3" aria-hidden />
			{label}
		</motion.span>
	);
}

/**
 * While a palette step is dragged, an insert button doubles as the drop slot
 * for its position.
 */
function useDropSlot(index: number) {
	const { draggingAction, dropAction } = useWorkflowCanvas();
	const [over, setOver] = useState(false);
	return {
		active: draggingAction,
		over: draggingAction && over,
		handlers: {
			onDragOver: (event: React.DragEvent) => {
				if (!carriesAction(event)) return;
				event.preventDefault();
				event.dataTransfer.dropEffect = "copy";
				setOver(true);
			},
			onDragLeave: (event: React.DragEvent) => {
				if (!event.currentTarget.contains(event.relatedTarget as Node | null))
					setOver(false);
			},
			onDrop: (event: React.DragEvent) => {
				if (!carriesAction(event)) return;
				event.preventDefault();
				event.stopPropagation();
				setOver(false);
				dropAction(index, event);
			},
		},
	};
}

function InsertBefore({ index, hidden }: { index: number; hidden: boolean }) {
	const { actions, insertStep } = useWorkflowCanvas();
	const slot = useDropSlot(index);
	return (
		<Box
			as="span"
			className={cn(
				"absolute left-1/2 z-10 -translate-x-1/2 -translate-y-1/2 transition-opacity duration-150",
				hidden && "pointer-events-none opacity-0",
			)}
			style={{ top: -COLUMN_GAP / 2 }}
		>
			<AddStepMenu
				actions={actions}
				onPick={(action) => insertStep(index, action)}
				trigger={
					<button
						type="button"
						aria-label={`Insert a step before step ${index + 1}`}
						{...slot.handlers}
						className={cn(
							"nodrag nopan flex size-6 items-center justify-center rounded-full border bg-card shadow-xs outline-none",
							"transition-[scale,color,border-color,background-color,box-shadow] duration-150 hover:border-primary/60 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.94] motion-reduce:active:scale-100",
							EASE_OUT_CLASS,
							slot.active
								? "scale-[1.34] border-primary/60 bg-primary/10 text-primary motion-reduce:scale-100"
								: "border-border text-muted-foreground",
							slot.over &&
								"border-primary bg-primary text-primary-foreground ring-4 ring-primary/25 hover:text-primary-foreground",
						)}
					>
						<Plus className="size-3.5" aria-hidden />
					</button>
				}
			/>
		</Box>
	);
}

function stepDescription(data: StepNodeData) {
	return [
		data.status ? statusBadge[data.status].label : undefined,
		data.problemCount > 0
			? `${data.problemCount} ${data.problemCount === 1 ? "problem" : "problems"} to fix`
			: undefined,
		data.summary,
		data.writes ? undefined : "Read only",
		data.conditionCount > 0
			? `${data.conditionCount} ${data.conditionCount === 1 ? "condition" : "conditions"}`
			: undefined,
		data.step.continue_on_error ? "Keeps going on failure" : undefined,
	]
		.filter(Boolean)
		.join(". ");
}

function DragHandle({ data }: { data: StepNodeData }) {
	const { moveStep, reveal } = useWorkflowCanvas();
	return (
		<button
			type="button"
			data-workflow-handle={data.step.id}
			aria-label={`Reorder step ${data.index + 1}`}
			aria-keyshortcuts="ArrowUp ArrowDown"
			onKeyDown={(event) => {
				const offset =
					event.key === "ArrowUp" ? -1 : event.key === "ArrowDown" ? 1 : 0;
				if (offset === 0) return;
				event.preventDefault();
				const handle = event.currentTarget;
				moveStep(data.index, data.index + offset);
				requestAnimationFrame(() =>
					requestAnimationFrame(() => {
						if (document.activeElement !== handle)
							handle.focus({ preventScroll: true });
						reveal(handle);
					}),
				);
			}}
			className={cn(
				STEP_DRAG_HANDLE,
				"nopan absolute inset-y-2 left-1 z-10 flex w-6 cursor-grab touch-none items-center justify-center rounded-md text-muted-foreground outline-none",
				"transition-colors duration-150 hover:bg-muted hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60 active:cursor-grabbing",
			)}
		>
			<GripVertical className="size-4" aria-hidden />
		</button>
	);
}

export function StepNode({
	id,
	data,
	dragging,
}: NodeProps & { data: StepNodeData }) {
	const { select } = useWorkflowCanvas();
	const reduceMotion = useReducedMotion();
	const descriptionId = useId();
	const Icon = actionGroupIcon(data.group);
	const invalid = data.problemCount > 0;
	return (
		<Entrance order={data.order}>
			<Handles top bottom right="source" />
			<InsertBefore index={data.index} hidden={dragging} />
			<motion.div
				className="relative"
				animate={
					reduceMotion
						? { opacity: dragging ? 0.8 : 1 }
						: { scale: dragging ? 1.02 : 1 }
				}
				transition={{ duration: 0.15, ease: easeOut }}
			>
				<button
					type="button"
					data-workflow-node={id}
					aria-label={`Step ${data.index + 1}: ${data.title}`}
					aria-describedby={descriptionId}
					aria-pressed={data.selected}
					onClick={() => select({ kind: "step", stepId: data.step.id })}
					className={cn(
						cardBase,
						"pl-9",
						data.status ? statusTone[data.status] : "border-border",
						data.selected && "border-primary shadow-md ring-2 ring-primary/25",
						invalid && "border-destructive ring-2 ring-destructive/20",
						dragging && "shadow-lg",
					)}
				>
					<Box as="span" className="flex items-center gap-3">
						<Tile icon={Icon} className={actionGroupTone(data.group)} />
						<Box as="span" className="min-w-0 flex-1">
							<Box
								as="span"
								className="flex items-center justify-between gap-2"
							>
								<Eyebrow>Step {data.index + 1}</Eyebrow>
								{invalid ? (
									<Box
										as="span"
										className="inline-flex shrink-0 items-center gap-1 rounded-full bg-destructive/10 px-2 py-0.5 text-[11px] font-medium text-destructive-on-tint"
									>
										<CircleAlert className="size-3" aria-hidden />
										{data.problemCount}
									</Box>
								) : data.status ? (
									<StatusBadge status={data.status} />
								) : null}
							</Box>
							<Box as="span" className="block truncate text-sm font-semibold">
								{data.title}
							</Box>
						</Box>
					</Box>
					<Box
						as="span"
						className="block truncate font-mono text-xs text-muted-foreground"
					>
						{data.summary}
					</Box>
					{data.conditionCount > 0 ||
					!data.writes ||
					data.step.continue_on_error ? (
						<Box as="span" className="flex flex-wrap gap-1">
							{!data.writes ? <Chip>Read only</Chip> : null}
							{data.conditionCount > 0 ? (
								<Chip>
									{data.conditionCount} condition
									{data.conditionCount > 1 ? "s" : ""}
								</Chip>
							) : null}
							{data.step.continue_on_error ? (
								<Chip>Keeps going on failure</Chip>
							) : null}
						</Box>
					) : null}
					<Box as="span" id={descriptionId} className="sr-only">
						{stepDescription(data)}
					</Box>
				</button>
				<DragHandle data={data} />
			</motion.div>
		</Entrance>
	);
}

function Chip({ children }: { children: ReactNode }) {
	return (
		<Box
			as="span"
			className="rounded-md border border-border px-1.5 py-0.5 text-[11px] text-muted-foreground"
		>
			{children}
		</Box>
	);
}

export function AddNode({ data }: NodeProps & { data: AddNodeData }) {
	const { actions, insertStep } = useWorkflowCanvas();
	const slot = useDropSlot(data.index);
	return (
		<Entrance order={data.order}>
			<Handles top />
			<AddStepMenu
				actions={actions}
				onPick={(action) => insertStep(data.index, action)}
				trigger={
					<button
						type="button"
						{...slot.handlers}
						className={cn(
							"nodrag nopan flex w-full items-center justify-center gap-2 rounded-xl border border-dashed px-3 py-3 text-sm font-medium outline-none",
							"transition-[scale,color,border-color,background-color,box-shadow] duration-150 hover:border-primary/50 hover:bg-primary/5 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.98] motion-reduce:active:scale-100",
							EASE_OUT_CLASS,
							slot.active
								? "scale-[1.02] border-primary/60 bg-primary/5 text-primary motion-reduce:scale-100"
								: "border-border-strong bg-card/60 text-muted-foreground",
							slot.over &&
								"border-primary bg-primary/10 ring-4 ring-primary/20",
						)}
					>
						<Plus className="size-4" aria-hidden />
						Add a step
					</button>
				}
			/>
		</Entrance>
	);
}

export function WorkflowRefNode({
	data,
}: NodeProps & { data: WorkflowRefNodeData }) {
	const { openWorkflow } = useWorkflowCanvas();
	const incoming = data.relation === "starts-this";
	return (
		<Entrance order={data.order}>
			{incoming ? <Handles bottom /> : <Handles left />}
			<button
				type="button"
				aria-label={`Open “${data.workflowName}”`}
				onClick={() => openWorkflow?.(data.workflowId)}
				className="nodrag nopan flex w-full items-center gap-2.5 rounded-xl border border-dashed border-border-strong bg-background/80 px-3 py-2.5 text-left text-sm outline-none transition-[transform,border-color] duration-150 ease-out hover:border-primary/50 focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.98] motion-reduce:active:scale-100"
			>
				{incoming ? (
					<ArrowDownToLine
						className="size-4 shrink-0 text-muted-foreground"
						aria-hidden
					/>
				) : (
					<ArrowUpRight
						className="size-4 shrink-0 text-muted-foreground"
						aria-hidden
					/>
				)}
				<Box as="span" className="min-w-0">
					<Eyebrow>{incoming ? "Started after" : "Starts"}</Eyebrow>
					<Box
						as="span"
						title={data.workflowName}
						className="block truncate font-medium"
					>
						{data.workflowName}
					</Box>
					{incoming ? null : (
						<Box
							as="span"
							className="block truncate font-mono text-[11px] text-muted-foreground"
						>
							on {data.event}
						</Box>
					)}
				</Box>
			</button>
		</Entrance>
	);
}

export const workflowNodeTypes = {
	trigger: TriggerNode,
	conditions: ConditionsNode,
	step: StepNode,
	add: AddNode,
	workflowRef: WorkflowRefNode,
};
