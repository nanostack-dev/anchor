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

/**
 * Fits the column without shrinking it past legibility. When it is still too
 * tall, the top stays in view so the trigger is the first thing read.
 */
function useReadableFit(
	container: React.RefObject<HTMLDivElement | null>,
	narrow: boolean,
) {
	const flow = useReactFlow();
	return useCallback(
		(duration: number) => {
			const element = container.current;
			const nodes = flow.getNodes();
			if (!element || nodes.length === 0) return;
			const bounds = flow.getNodesBounds(nodes);
			const width = element.clientWidth;
			const height = element.clientHeight;
			const zoom = Math.min(
				FIT_MAX_ZOOM,
				Math.max(
					narrow ? FIT_MIN_ZOOM_NARROW : FIT_MIN_ZOOM,
					Math.min(
						(width - FIT_PADDING * 2) / bounds.width,
						(height - FIT_PADDING * 2) / bounds.height,
					),
				),
			);
			const fitsVertically = bounds.height * zoom + FIT_PADDING * 2 <= height;
			void flow.setViewport(
				{
					x: width / 2 - (bounds.x + bounds.width / 2) * zoom,
					y: fitsVertically
						? height / 2 - (bounds.y + bounds.height / 2) * zoom
						: FIT_PADDING - bounds.y * zoom,
					zoom,
				},
				{ duration },
			);
		},
		[container, flow, narrow],
	);
}

function ZoomControls({ fit }: { fit: (duration: number) => void }) {
	const flow = useReactFlow();
	const reduceMotion = useReducedMotion();
	const duration = reduceMotion ? 0 : 200;
	return (
		<Box className="absolute right-3 bottom-3 z-10 flex flex-col overflow-hidden rounded-lg border border-border bg-card shadow-sm">
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

function Canvas({
	nodes,
	edges,
	fitKey,
	actions,
}: {
	nodes: WorkflowGraphNode[];
	edges: WorkflowGraphEdge[];
	fitKey: string;
	actions: Omit<WorkflowCanvasActions, "entranceDelay">;
}) {
	const container = useRef<HTMLDivElement>(null);
	const isMobile = useIsMobile();
	const fit = useReadableFit(container, isMobile);
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
	useEffect(() => {
		if (!paneReady || !measured || fittedFor.current === fitKey) return;
		const firstFit = fittedFor.current === null;
		fittedFor.current = fitKey;
		const timer = window.setTimeout(
			() => fit(firstFit || reduceMotion ? 0 : 280),
			firstFit ? 0 : 50,
		);
		return () => window.clearTimeout(timer);
	}, [fitKey, fit, measured, paneReady, reduceMotion]);

	return (
		<WorkflowCanvasContext.Provider value={context}>
			<Box ref={container} className="h-full w-full">
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

export function WorkflowCanvas(props: {
	nodes: WorkflowGraphNode[];
	edges: WorkflowGraphEdge[];
	fitKey: string;
	actions: Omit<WorkflowCanvasActions, "entranceDelay">;
}) {
	return (
		<Box className="relative h-full min-h-[420px] w-full overflow-hidden rounded-xl border border-border bg-surface-subtle">
			<ReactFlowProvider>
				<Canvas {...props} />
			</ReactFlowProvider>
		</Box>
	);
}
