import { IconButton } from "@nanostackorg/design-system/components/button";
import { Box } from "@nanostackorg/design-system/layout/box";
import {
	Background,
	BackgroundVariant,
	type NodeChange,
	ReactFlow,
	ReactFlowProvider,
	applyNodeChanges,
	useNodesInitialized,
	useReactFlow,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { useIsMobile } from "@/hooks/use-mobile";
import { cn } from "@/lib/utils";
import { Maximize, Minus, Plus } from "lucide-react";
import { useReducedMotion } from "motion/react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { workflowEdgeTypes } from "./FlowEdge";
import {
	type WorkflowCanvasActions,
	WorkflowCanvasContext,
} from "./canvas-context";
import { ENTRANCE_STAGGER_SECONDS, MAX_STAGGERED_ORDER } from "./motion";
import { workflowNodeTypes } from "./nodes";
import {
	type WorkflowGraphEdge,
	type WorkflowGraphNode,
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

/**
 * Fits the workflow without shrinking it past legibility. When the chained
 * workflows beside the column do not fit, only the column (and its loop edge)
 * is fitted to the width. A column still too tall keeps the focused step, or
 * else the trigger, in view.
 */
function useReadableFit(
	container: React.RefObject<HTMLDivElement | null>,
	hasLoop: boolean,
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
			const width = element.clientWidth - FIT_PADDING * 2;
			const height = element.clientHeight - FIT_PADDING * 2;
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
			const top = FIT_PADDING - all.y * zoom;
			const bottom =
				element.clientHeight - FIT_PADDING - (all.y + all.height) * zoom;
			const focus = focusId ? flow.getNode(focusId) : undefined;
			const y =
				all.height * zoom <= height
					? element.clientHeight / 2 - (all.y + all.height / 2) * zoom
					: focus
						? Math.min(
								top,
								Math.max(
									bottom,
									element.clientHeight / 2 -
										(focus.position.y + (focus.measured?.height ?? 0) / 2) *
											zoom,
								),
							)
						: top;
			void flow.setViewport(
				{
					x: element.clientWidth / 2 - ((box.left + box.right) / 2) * zoom,
					y,
					zoom,
				},
				{ duration },
			);
		},
		[container, flow, hasLoop],
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

interface WorkflowCanvasProps {
	nodes: WorkflowGraphNode[];
	edges: WorkflowGraphEdge[];
	fitKey: string;
	focusNodeId?: string;
	actions: Omit<WorkflowCanvasActions, "entranceDelay">;
}

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
	const hasLoop = edges.some((edge) => edge.data?.tone === "loop");
	const fit = useReadableFit(container, hasLoop);
	const reduceMotion = useReducedMotion();
	const mountedAt = useRef(Date.now());
	const entranceDelay = useCallback(
		(order: number) =>
			reduceMotion || Date.now() - mountedAt.current > FIRST_PAINT_MS
				? 0
				: Math.min(order, MAX_STAGGERED_ORDER) * ENTRANCE_STAGGER_SECONDS,
		[reduceMotion],
	);
	const context = useMemo(
		() => ({ ...actions, entranceDelay }),
		[actions, entranceDelay],
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
	useEffect(() => {
		setFlowNodes((previous) => {
			const measuredById = new Map(
				previous.map((node) => [node.id, node.measured]),
			);
			return stackColumn(
				nodes.map((node) => ({
					...node,
					measured: measuredById.get(node.id),
				})),
			);
		});
	}, [nodes]);
	const onNodesChange = useCallback(
		(changes: NodeChange<WorkflowGraphNode>[]) =>
			setFlowNodes((current) =>
				stackColumn(
					applyNodeChanges(
						changes.filter((change) => change.type === "dimensions"),
						current,
					),
				),
			),
		[],
	);
	const measured = useNodesInitialized();
	const [paneReady, setPaneReady] = useState(false);
	const fittedFor = useRef<string | null>(null);
	const focusRef = useRef(focusNodeId);
	if (focusNodeId) focusRef.current = focusNodeId;
	const [fitted, setFitted] = useState(false);
	const [shown, setShown] = useState(true);
	useEffect(() => {
		const element = container.current;
		if (!element) return;
		const observer = new ResizeObserver(() =>
			setShown(element.clientWidth > 0 && element.clientHeight > 0),
		);
		observer.observe(element);
		return () => observer.disconnect();
	}, []);
	useEffect(() => {
		if (!shown || !paneReady || !measured || fittedFor.current === fitKey)
			return;
		const firstFit = fittedFor.current === null;
		const timer = window.setTimeout(
			() => {
				fittedFor.current = fitKey;
				fit(
					firstFit || reduceMotion ? 0 : 280,
					firstFit ? undefined : focusRef.current,
				);
				setFitted(true);
			},
			firstFit ? 0 : 50,
		);
		return () => window.clearTimeout(timer);
	}, [fitKey, fit, measured, paneReady, reduceMotion, shown]);

	const panToKeyboardFocus = useCallback(
		(event: React.FocusEvent<HTMLDivElement>) => {
			const element = container.current;
			const target = event.target;
			if (!element || !target.matches(":focus-visible")) return;
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
			const dx = shift(box.left, box.right, view.left, view.right);
			const dy = shift(box.top, box.bottom, view.top, view.bottom);
			if (dx === 0 && dy === 0) return;
			const viewport = flow.getViewport();
			void flow.setViewport(
				{ x: viewport.x + dx, y: viewport.y + dy, zoom: viewport.zoom },
				{ duration: reduceMotion ? 0 : 200 },
			);
		},
		[flow, reduceMotion],
	);

	return (
		<WorkflowCanvasContext.Provider value={context}>
			<Box
				ref={container}
				onFocus={panToKeyboardFocus}
				className={cn(
					"h-full w-full transition-opacity duration-150 ease-out",
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
					nodesDraggable={false}
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
					proOptions={{ hideAttribution: false }}
				>
					<Background
						variant={BackgroundVariant.Dots}
						gap={20}
						size={1.4}
						color="var(--color-border-strong)"
					/>
					<ZoomControls fit={fit} />
				</ReactFlow>
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
