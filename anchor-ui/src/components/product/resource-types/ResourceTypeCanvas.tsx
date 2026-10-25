import "@xyflow/react/dist/base.css";
import "./resource-type-canvas.css";

import { IconButton } from "@nanostackorg/design-system/components/button";
import {
	ArrowsInIcon,
	MagnifyingGlassMinusIcon,
	MagnifyingGlassPlusIcon,
} from "@phosphor-icons/react";
import {
	Background,
	BackgroundVariant,
	type Edge,
	type NodeTypes,
	Panel,
	Position,
	ReactFlow,
	ReactFlowProvider,
	useReactFlow,
	useStoreApi,
} from "@xyflow/react";
import { animate, useReducedMotion } from "motion/react";
import { useEffect, useMemo, useRef, useState } from "react";

import {
	type CanvasActions,
	CanvasActionsContext,
	type NodeTrace,
	type ResourceTypeFlowNode,
	ResourceTypeNode,
} from "./ResourceTypeNode";
import type { AccessHighlight } from "./access-explainer";
import {
	type Layout,
	NODE_HEIGHT,
	NODE_WIDTH,
	interpolateLayout,
	sameLayout,
} from "./resource-type-layout";
import { type TreeType, addChildBlocker } from "./resource-type-tree";

const nodeTypes: NodeTypes = { resourceType: ResourceTypeNode };
const EASE_OUT = [0.23, 1, 0.32, 1] as const;
const REFLOW_SECONDS = 0.2;
const FIT = {
	padding: { top: "32px", x: "32px", bottom: "72px" },
	maxZoom: 1,
} as const;
const READABLE_FIT = { ...FIT, minZoom: 0.5 };
const HANDLES = [
	{
		type: "target" as const,
		position: Position.Top,
		x: NODE_WIDTH / 2,
		y: 0,
		width: 1,
		height: 1,
	},
	{
		type: "source" as const,
		position: Position.Bottom,
		x: NODE_WIDTH / 2,
		y: NODE_HEIGHT,
		width: 1,
		height: 1,
	},
];

function useAnimatedLayout(target: Layout, instant: boolean) {
	const reduceMotion = useReducedMotion();
	const [frame, setFrame] = useState(target);
	const shown = useRef(target);
	useEffect(() => {
		const from = shown.current;
		if (sameLayout(from, target)) return;
		if (instant || reduceMotion) {
			shown.current = target;
			setFrame(target);
			return;
		}
		const controls = animate(0, 1, {
			duration: REFLOW_SECONDS,
			ease: EASE_OUT,
			onUpdate: (progress) => {
				shown.current = interpolateLayout(from, target, progress);
				setFrame(shown.current);
			},
			onComplete: () => {
				shown.current = target;
				setFrame(target);
			},
		});
		return () => controls.stop();
	}, [target, instant, reduceMotion]);
	return frame.size === target.size &&
		[...target.keys()].every((key) => frame.has(key))
		? frame
		: interpolateLayout(frame, target, 0);
}

const VISIBLE_MARGIN = 16;

function useKeepInView(
	layout: Layout,
	key: string | undefined,
	request: number | undefined,
) {
	const flow = useReactFlow();
	const store = useStoreApi();
	const latest = useRef(layout);
	latest.current = layout;
	// biome-ignore lint/correctness/useExhaustiveDependencies: runs once per focus request, not per layout frame
	useEffect(() => {
		const point = key ? latest.current.get(key) : undefined;
		if (!point) return;
		const { width, height, transform } = store.getState();
		const [x, y, zoom] = transform;
		const left = point.x * zoom + x;
		const top = point.y * zoom + y;
		const inside =
			left >= VISIBLE_MARGIN &&
			top >= VISIBLE_MARGIN &&
			left + NODE_WIDTH * zoom <= width - VISIBLE_MARGIN &&
			top + (NODE_HEIGHT + 16) * zoom <= height - VISIBLE_MARGIN;
		if (!inside)
			void flow.setCenter(point.x + NODE_WIDTH / 2, point.y + NODE_HEIGHT / 2, {
				zoom,
				duration: 0,
			});
	}, [flow, store, key, request]);
}

function ZoomControls() {
	const flow = useReactFlow();
	return (
		<Panel position="bottom-left">
			<div className="nodrag nopan flex gap-1 rounded-xl border border-border bg-card p-1 shadow-xs">
				<IconButton
					icon={MagnifyingGlassPlusIcon}
					label="Zoom in"
					size="sm"
					onClick={() => flow.zoomIn({ duration: 0 })}
				/>
				<IconButton
					icon={MagnifyingGlassMinusIcon}
					label="Zoom out"
					size="sm"
					onClick={() => flow.zoomOut({ duration: 0 })}
				/>
				<IconButton
					icon={ArrowsInIcon}
					label="Fit to view"
					size="sm"
					onClick={() => flow.fitView({ ...FIT, duration: 0 })}
				/>
			</div>
		</Panel>
	);
}

export interface ResourceTypeCanvasProps {
	types: TreeType[];
	layout: Layout;
	editing: boolean;
	selectedKey: string | null;
	namingKey: string | null;
	entering: ReadonlySet<string>;
	instant: boolean;
	focusRequest: { key: string; id: number } | null;
	actions: CanvasActions;
	trace?: AccessHighlight | null;
}

function traceOf(
	slug: string,
	trace: AccessHighlight | null | undefined,
): NodeTrace | undefined {
	if (!trace) return undefined;
	if (trace.granted[slug]) return "granted";
	return trace.path.includes(slug) ? "path" : undefined;
}

function Canvas({
	types,
	layout,
	editing,
	selectedKey,
	namingKey,
	entering,
	instant,
	focusRequest,
	actions,
	trace,
}: ResourceTypeCanvasProps) {
	const positions = useAnimatedLayout(layout, instant);
	const tabbableKey =
		selectedKey && types.some((type) => type.key === selectedKey)
			? selectedKey
			: types[0]?.key;

	const focusKey = focusRequest?.key;
	const focusId = focusRequest?.id;
	useKeepInView(layout, namingKey ?? focusKey, focusId);
	const nodes = useMemo<ResourceTypeFlowNode[]>(
		() =>
			types.map((type) => ({
				id: type.key,
				type: "resourceType",
				position: positions.get(type.key) ?? { x: 0, y: 0 },
				width: NODE_WIDTH,
				height: NODE_HEIGHT,
				measured: { width: NODE_WIDTH, height: NODE_HEIGHT },
				handles: HANDLES,
				data: {
					type,
					editing,
					selected: type.key === selectedKey,
					naming: type.key === namingKey,
					entering: entering.has(type.key),
					tabbable: type.key === tabbableKey,
					focusId: focusKey === type.key ? focusId : undefined,
					addBlocker: editing ? addChildBlocker(types, type.key) : undefined,
					trace: traceOf(type.slug, trace),
					holders: trace?.granted[type.slug],
				},
			})),
		[
			types,
			positions,
			editing,
			selectedKey,
			namingKey,
			entering,
			tabbableKey,
			focusKey,
			focusId,
			trace,
		],
	);

	const edges = useMemo<Edge[]>(() => {
		const slugOf = new Map(types.map((type) => [type.key, type.slug]));
		const onPath = (parent: string, child: string) =>
			trace?.steps.some(
				([from, to]) => from === slugOf.get(parent) && to === slugOf.get(child),
			) ?? false;
		return types.flatMap((type) =>
			type.parents
				.filter((parent) => parent !== type.key)
				.map((parent) => {
					const step = onPath(parent, type.key);
					return {
						id: `${parent}->${type.key}`,
						source: parent,
						target: type.key,
						focusable: false,
						selectable: false,
						className: trace ? (step ? "trace-step" : "trace-dimmed") : "",
						style: step
							? { stroke: "var(--primary)", strokeWidth: 2.5 }
							: { stroke: "var(--border-strong)", strokeWidth: 1.5 },
					};
				}),
		);
	}, [types, trace]);

	return (
		<CanvasActionsContext.Provider value={actions}>
			<section
				className="resource-type-canvas h-full w-full"
				aria-label="Resource type tree"
			>
				<ReactFlow
					nodes={nodes}
					edges={edges}
					nodeTypes={nodeTypes}
					onNodeClick={(_, node) => actions.select(node.id)}
					onPaneClick={() => actions.select(null)}
					nodesDraggable={false}
					nodesConnectable={false}
					nodesFocusable={false}
					edgesFocusable={false}
					elementsSelectable={false}
					deleteKeyCode={null}
					selectionKeyCode={null}
					multiSelectionKeyCode={null}
					zoomOnScroll={false}
					zoomOnDoubleClick={false}
					panOnScroll={false}
					preventScrolling={false}
					minZoom={0.1}
					maxZoom={1.5}
					fitView
					fitViewOptions={READABLE_FIT}
				>
					<Background
						variant={BackgroundVariant.Dots}
						gap={16}
						color="var(--border-strong)"
					/>
					<ZoomControls />
				</ReactFlow>
			</section>
		</CanvasActionsContext.Provider>
	);
}

export function ResourceTypeCanvas(props: ResourceTypeCanvasProps) {
	return (
		<ReactFlowProvider>
			<Canvas {...props} />
		</ReactFlowProvider>
	);
}
