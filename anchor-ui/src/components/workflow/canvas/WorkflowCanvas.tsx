import type { WorkflowActionResponse } from "@/client";
import { IconButton } from "@nanostackorg/design-system/components/button";
import { Box } from "@nanostackorg/design-system/layout/box";
import {
	Background,
	BackgroundVariant,
	type NodeChange,
	ReactFlow,
	ReactFlowProvider,
	ViewportPortal,
	applyNodeChanges,
	useNodesInitialized,
	useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { Maximize, Minus, Plus } from "lucide-react";
import { motion, useReducedMotion } from "motion/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { workflowEdgeTypes } from "./FlowEdge";
import {
	PALETTE_PANEL_WIDTH,
	StepPalette,
	type StepPalettePlacement,
} from "./StepPalette";
import {
	ACTION_DRAG_TYPE,
	type WorkflowCanvasActions,
	WorkflowCanvasContext,
	carriesAction,
} from "./canvas-context";
import {
	EASE_OUT_CLASS,
	ENTRANCE_STAGGER_SECONDS,
	MAX_STAGGERED_ORDER,
	easeOut,
} from "./motion";
import { workflowNodeTypes } from "./nodes";
import {
	NODE_WIDTH,
	type WorkflowGraphEdge,
	type WorkflowGraphNode,
	slotIndexAt,
	slotLineY,
	stackColumn,
} from "./workflow-graph";

const FIRST_PAINT_MS = 600;

/**
 * React Flow drops pointer events on a node that is neither selectable,
 * draggable nor clickable. Each node handles its own click; this handler only
 * keeps the node reachable.
 */
const keepNodesInteractive = () => {};
const FIT_PADDING = 32;
const FIT_MIN_ZOOM = 0.85;
const FIT_MIN_ZOOM_NARROW = 0.65;
const FIT_MAX_ZOOM = 1;
const LOOP_ROOM = 80;
const PANEL_MARGIN = 12;
const PALETTE_INSET = PALETTE_PANEL_WIDTH + PANEL_MARGIN * 2;
/** Below this width the open palette would cover the column, so it docks at the bottom. */
const PANEL_MIN_CANVAS_WIDTH =
	PALETTE_INSET + NODE_WIDTH * FIT_MIN_ZOOM_NARROW + FIT_PADDING * 2;
/** Keeps the top of the column clear of the "Steps" toggle. */
const FIT_PADDING_TOP = 56;
/**
 * The palette starts open from this width, where it floats beside the column;
 * on a canvas too narrow for that it would dock over the steps, so it waits
 * for its toggle.
 */
const WIDE_VIEWPORT = "(min-width: 48rem)";
const DRAGGED_NODE_Z = 1000;
const DROP_LINE_OVERHANG = 20;

/**
 * Fits the workflow without shrinking it past legibility, in the part of the
 * canvas right of `inset`. When the chained workflows beside the column do
 * not fit, only the column (and its loop edge) is fitted to the width. A
 * column still too tall keeps the focused step, or else the trigger, in view.
 */
function useReadableFit(
	container: React.RefObject<HTMLDivElement | null>,
	hasLoop: boolean,
	inset: number,
) {
	const flow = useReactFlow();
	return useCallback(
		(duration: number, focusId?: string) => {
			const element = container.current;
			const nodes = flow.getNodes();
			if (!element || nodes.length === 0) return;
			const all = flow.getNodesBounds(nodes);
			const column = flow.getNodesBounds(
				nodes.filter((node) => node.type !== "workflowRef"),
			);
			const columnRight = column.x + column.width + (hasLoop ? LOOP_ROOM : 0);
			const width = element.clientWidth - inset - FIT_PADDING * 2;
			const height = element.clientHeight - FIT_PADDING_TOP - FIT_PADDING;
			const middle = FIT_PADDING_TOP + height / 2;
			const wide = {
				left: all.x,
				right: Math.max(all.x + all.width, columnRight),
			};
			const box =
				width / (wide.right - wide.left) >= FIT_MIN_ZOOM
					? wide
					: { left: column.x, right: columnRight };
			const zoom = Math.max(
				FIT_MIN_ZOOM_NARROW,
				Math.min(
					FIT_MAX_ZOOM,
					width / (box.right - box.left),
					Math.max(height / all.height, FIT_MIN_ZOOM),
				),
			);
			const top = FIT_PADDING_TOP - all.y * zoom;
			const bottom =
				element.clientHeight - FIT_PADDING - (all.y + all.height) * zoom;
			const focus = focusId ? flow.getNode(focusId) : undefined;
			const y =
				all.height * zoom <= height
					? middle - (all.y + all.height / 2) * zoom
					: focus
						? Math.min(
								top,
								Math.max(
									bottom,
									middle -
										(focus.position.y + (focus.measured?.height ?? 0) / 2) *
											zoom,
								),
							)
						: top;
			void flow.setViewport(
				{
					x:
						inset +
						(element.clientWidth - inset) / 2 -
						((box.left + box.right) / 2) * zoom,
					y,
					zoom,
				},
				{ duration },
			);
		},
		[container, flow, hasLoop, inset],
	);
}

function ZoomControls({ fit }: { fit: (duration: number) => void }) {
	const flow = useReactFlow();
	const reduceMotion = useReducedMotion();
	const duration = reduceMotion ? 0 : 200;
	return (
		<Box className="absolute bottom-3 left-3 z-10 flex flex-col overflow-hidden rounded-lg border border-border bg-card shadow-sm">
			<IconButton
				variant="ghost"
				size="sm"
				icon={Plus}
				label="Zoom in"
				onClick={() => void flow.zoomIn({ duration })}
			/>
			<IconButton
				variant="ghost"
				size="sm"
				icon={Minus}
				label="Zoom out"
				onClick={() => void flow.zoomOut({ duration })}
			/>
			<IconButton
				variant="ghost"
				size="sm"
				icon={Maximize}
				label="Fit the workflow"
				onClick={() => fit(reduceMotion ? 0 : 280)}
			/>
		</Box>
	);
}

/**
 * Where a dragged step would land, drawn in the gap it would fill. It reaches
 * past the column so its ends still show beside a lifted card covering it.
 */
function DropIndicator({ y }: { y: number }) {
	const reduceMotion = useReducedMotion();
	return (
		<Box
			aria-hidden
			className={cn(
				"pointer-events-none absolute top-0 left-0 -z-10 transition-transform duration-150 motion-reduce:transition-none",
				EASE_OUT_CLASS,
			)}
			style={{
				width: NODE_WIDTH + DROP_LINE_OVERHANG * 2,
				transform: `translate(${-DROP_LINE_OVERHANG}px, ${y}px)`,
			}}
		>
			<motion.div
				initial={reduceMotion ? { opacity: 0 } : { opacity: 0, scaleX: 0.96 }}
				animate={reduceMotion ? { opacity: 1 } : { opacity: 1, scaleX: 1 }}
				transition={{ duration: 0.15, ease: easeOut }}
				className="relative -mt-px h-0.5 rounded-full bg-primary"
			>
				<Box
					as="span"
					className="absolute top-1/2 -left-1 size-2 -translate-y-1/2 rounded-full border-2 border-primary bg-card"
				/>
				<Box
					as="span"
					className="absolute top-1/2 -right-1 size-2 -translate-y-1/2 rounded-full border-2 border-primary bg-card"
				/>
			</motion.div>
		</Box>
	);
}

interface WorkflowCanvasProps {
	nodes: WorkflowGraphNode[];
	edges: WorkflowGraphEdge[];
	fitKey: string;
	focusNodeId?: string;
	actions: WorkflowCanvasActions;
}

function relayout(
	nodes: WorkflowGraphNode[],
	previous: WorkflowGraphNode[],
): WorkflowGraphNode[] {
	const measuredById = new Map(
		previous.map((node) => [node.id, node.measured]),
	);
	return stackColumn(
		nodes.map((node) => ({ ...node, measured: measuredById.get(node.id) })),
	);
}

const centerY = (node: WorkflowGraphNode) =>
	node.position.y + (node.measured?.height ?? 0) / 2;

function Canvas({
	nodes,
	edges,
	fitKey,
	focusNodeId,
	actions,
}: WorkflowCanvasProps) {
	const container = useRef<HTMLDivElement>(null);
	const flow = useReactFlow();
	const isMobile = useIsMobile();
	const reduceMotion = useReducedMotion();
	const [size, setSize] = useState({ width: 0, height: 0 });
	const shown = size.width > 0 && size.height > 0;
	const [wideViewport] = useState(
		() => window.matchMedia(WIDE_VIEWPORT).matches,
	);
	const placement: StepPalettePlacement =
		size.width >= PANEL_MIN_CANVAS_WIDTH ? "panel" : "tray";
	const [paletteChoice, setPaletteOpen] = useState<boolean | null>(null);
	const paletteOpen = paletteChoice ?? (wideViewport && placement === "panel");
	const inset = paletteOpen && placement === "panel" ? PALETTE_INSET : 0;
	const hasLoop = edges.some((edge) => edge.data?.tone === "loop");
	const fit = useReadableFit(container, hasLoop, inset);
	const mountedAt = useRef(Date.now());
	const entranceDelay = useCallback(
		(order: number) =>
			reduceMotion || Date.now() - mountedAt.current > FIRST_PAINT_MS
				? 0
				: Math.min(order, MAX_STAGGERED_ORDER) * ENTRANCE_STAGGER_SECONDS,
		[reduceMotion],
	);
	const routedEdges = useMemo(
		() =>
			edges.map((edge) => ({
				...edge,
				sourceHandle: edge.sourceHandle ?? "bottom",
				targetHandle: edge.targetHandle ?? "top",
			})),
		[edges],
	);

	const [flowNodes, setFlowNodes] = useState<WorkflowGraphNode[]>(nodes);
	const flowNodesRef = useRef(flowNodes);
	flowNodesRef.current = flowNodes;
	const nodesRef = useRef(nodes);
	nodesRef.current = nodes;
	useEffect(() => {
		setFlowNodes((previous) => relayout(nodes, previous));
	}, [nodes]);
	const onNodesChange = useCallback(
		(changes: NodeChange<WorkflowGraphNode>[]) =>
			setFlowNodes((current) => {
				const next = applyNodeChanges(
					changes.filter(
						(change) =>
							change.type === "dimensions" || change.type === "position",
					),
					current,
				);
				return next.some((node) => node.dragging) ? next : stackColumn(next);
			}),
		[],
	);
	const stepCount = nodes.filter((node) => node.type === "step").length;

	const [drop, setDrop] = useState<{ index: number; ignoreId?: string } | null>(
		null,
	);
	const showDrop = useCallback(
		(index: number | null, ignoreId?: string) =>
			setDrop((current) =>
				index === null
					? null
					: current?.index === index && current.ignoreId === ignoreId
						? current
						: { index, ignoreId },
			),
		[],
	);
	const [draggingAction, setDraggingAction] = useState(false);
	const dragStartTimer = useRef<number | undefined>(undefined);
	const endActionDrag = useCallback(() => {
		window.clearTimeout(dragStartTimer.current);
		setDraggingAction(false);
		showDrop(null);
	}, [showDrop]);
	const onPaletteDrag = useCallback(
		(dragging: boolean) => {
			if (!dragging) return endActionDrag();
			window.clearTimeout(dragStartTimer.current);
			dragStartTimer.current = window.setTimeout(
				() => setDraggingAction(true),
				0,
			);
		},
		[endActionDrag],
	);
	useEffect(() => {
		if (!draggingAction) return;
		window.addEventListener("dragend", endActionDrag);
		window.addEventListener("drop", endActionDrag);
		return () => {
			window.removeEventListener("dragend", endActionDrag);
			window.removeEventListener("drop", endActionDrag);
		};
	}, [draggingAction, endActionDrag]);

	const [announcement, setAnnouncement] = useState("");
	const moveStep = useCallback(
		(from: number, to: number) => {
			if (to === from || to < 0 || to >= stepCount) return;
			actions.moveStep(from, to);
			setAnnouncement(`Step moved to position ${to + 1}`);
		},
		[actions, stepCount],
	);
	const addStep = useCallback(
		(index: number, action: WorkflowActionResponse) => {
			actions.insertStep(index, action);
			if (placement === "tray") setPaletteOpen(false);
		},
		[actions, placement],
	);
	const dropAction = useCallback(
		(index: number, event: React.DragEvent) => {
			const type = event.dataTransfer.getData(ACTION_DRAG_TYPE);
			const action = actions.actions.find((item) => item.type === type);
			endActionDrag();
			if (action) addStep(index, action);
		},
		[actions.actions, addStep, endActionDrag],
	);
	const slotUnderPointer = (event: React.DragEvent) =>
		slotIndexAt(
			flowNodesRef.current,
			flow.screenToFlowPosition({ x: event.clientX, y: event.clientY }).y,
		);

	const measured = useNodesInitialized();
	const [paneReady, setPaneReady] = useState(false);
	const fittedFor = useRef<string | null>(null);
	const focusRef = useRef(focusNodeId);
	if (focusNodeId) focusRef.current = focusNodeId;
	const [fitted, setFitted] = useState(false);
	useEffect(() => {
		const element = container.current;
		if (!element) return;
		const observer = new ResizeObserver(() =>
			setSize((current) =>
				current.width === element.clientWidth &&
				current.height === element.clientHeight
					? current
					: { width: element.clientWidth, height: element.clientHeight },
			),
		);
		observer.observe(element);
		return () => observer.disconnect();
	}, []);
	const fitTarget = `${fitKey}:${inset}`;
	useEffect(() => {
		if (!shown || !paneReady || !measured || fittedFor.current === fitTarget)
			return;
		const firstFit = fittedFor.current === null;
		const timer = window.setTimeout(
			() => {
				fittedFor.current = fitTarget;
				fit(
					firstFit || reduceMotion ? 0 : 280,
					firstFit ? undefined : focusRef.current,
				);
				setFitted(true);
			},
			firstFit ? 0 : 50,
		);
		return () => window.clearTimeout(timer);
	}, [fitTarget, fit, measured, paneReady, reduceMotion, shown]);

	const reveal = useCallback(
		(target: HTMLElement) => {
			const element = container.current;
			if (!element) return;
			for (
				let ancestor = target.parentElement;
				ancestor && ancestor !== element;
				ancestor = ancestor.parentElement
			) {
				ancestor.scrollTop = 0;
				ancestor.scrollLeft = 0;
			}
			const view = element.getBoundingClientRect();
			const box = target.getBoundingClientRect();
			const shift = (start: number, end: number, min: number, max: number) =>
				start < min + FIT_PADDING
					? min + FIT_PADDING - start
					: end > max - FIT_PADDING
						? max - FIT_PADDING - end
						: 0;
			const dx = shift(box.left, box.right, view.left + inset, view.right);
			const dy = shift(
				box.top,
				box.bottom,
				view.top + FIT_PADDING_TOP - FIT_PADDING,
				view.bottom,
			);
			if (dx === 0 && dy === 0) return;
			const viewport = flow.getViewport();
			void flow.setViewport(
				{ x: viewport.x + dx, y: viewport.y + dy, zoom: viewport.zoom },
				{ duration: reduceMotion ? 0 : 200 },
			);
		},
		[flow, inset, reduceMotion],
	);
	const panToKeyboardFocus = useCallback(
		(event: React.FocusEvent<HTMLDivElement>) => {
			const target = event.target;
			if (
				target.matches(":focus-visible") &&
				target.closest(".react-flow__node")
			)
				reveal(target);
		},
		[reveal],
	);

	const context = useMemo(
		() => ({
			...actions,
			moveStep,
			entranceDelay,
			draggingAction,
			dropAction,
			reveal,
		}),
		[actions, moveStep, entranceDelay, draggingAction, dropAction, reveal],
	);

	return (
		<WorkflowCanvasContext.Provider value={context}>
			<Box
				ref={container}
				onFocus={panToKeyboardFocus}
				className={cn(
					"relative h-full w-full transition-opacity duration-150 ease-out",
					!fitted && "opacity-0",
				)}
			>
				<ReactFlow
					aria-label="Workflow canvas"
					nodes={flowNodes}
					onNodesChange={onNodesChange}
					edges={routedEdges}
					nodeTypes={workflowNodeTypes}
					edgeTypes={workflowEdgeTypes}
					minZoom={0.35}
					maxZoom={1.5}
					nodesDraggable
					nodesConnectable={false}
					nodesFocusable={false}
					edgesFocusable={false}
					elementsSelectable={false}
					zoomOnScroll={false}
					zoomOnDoubleClick={false}
					panOnScroll={!isMobile}
					preventScrolling={!isMobile}
					onInit={() => setPaneReady(true)}
					onPaneClick={() => actions.select({ kind: "workflow" })}
					onNodeClick={keepNodesInteractive}
					onNodeDragStart={(_, node) =>
						setFlowNodes((current) =>
							current.map((item) =>
								item.id === node.id
									? { ...item, zIndex: DRAGGED_NODE_Z }
									: item,
							),
						)
					}
					onNodeDrag={(_, node) => {
						if (node.type !== "step") return;
						const index = slotIndexAt(
							flowNodesRef.current,
							centerY(node),
							node.id,
						);
						showDrop(index === node.data.index ? null : index, node.id);
					}}
					onNodeDragStop={(_, node) => {
						showDrop(null);
						setFlowNodes((previous) => relayout(nodesRef.current, previous));
						if (node.type !== "step") return;
						moveStep(
							node.data.index,
							slotIndexAt(flowNodesRef.current, centerY(node), node.id),
						);
					}}
					onDragOver={(event) => {
						if (!carriesAction(event)) return;
						event.preventDefault();
						event.dataTransfer.dropEffect = "copy";
						showDrop(slotUnderPointer(event));
					}}
					onDragLeave={(event) => {
						if (
							!event.currentTarget.contains(event.relatedTarget as Node | null)
						)
							showDrop(null);
					}}
					onDrop={(event) => {
						if (!carriesAction(event)) return;
						event.preventDefault();
						dropAction(slotUnderPointer(event), event);
					}}
					proOptions={{ hideAttribution: false }}
				>
					<Background
						variant={BackgroundVariant.Dots}
						gap={20}
						size={1.4}
						color="var(--color-border-strong)"
					/>
					<ZoomControls fit={fit} />
					{drop ? (
						<ViewportPortal>
							<DropIndicator
								y={slotLineY(flowNodes, drop.index, drop.ignoreId)}
							/>
						</ViewportPortal>
					) : null}
				</ReactFlow>
				{shown ? (
					<StepPalette
						actions={actions.actions}
						open={paletteOpen}
						onOpenChange={setPaletteOpen}
						placement={placement}
						onAdd={(action) => addStep(stepCount, action)}
						onDragChange={onPaletteDrag}
						dragging={draggingAction}
					/>
				) : null}
				<Box aria-live="polite" aria-atomic="true" className="sr-only">
					{announcement}
				</Box>
			</Box>
		</WorkflowCanvasContext.Provider>
	);
}

export function WorkflowCanvas(props: WorkflowCanvasProps) {
	return (
		<Box className="relative h-full min-h-[420px] w-full overflow-hidden rounded-xl border border-border bg-surface-subtle">
			<ReactFlowProvider>
				<Canvas {...props} />
			</ReactFlowProvider>
		</Box>
	);
}
