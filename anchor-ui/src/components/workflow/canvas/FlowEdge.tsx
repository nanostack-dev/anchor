import { cn } from "@/lib/utils";
import { Box } from "@nanostackorg/design-system/layout/box";
import {
	BaseEdge,
	EdgeLabelRenderer,
	type EdgeProps,
	getBezierPath,
	getSmoothStepPath,
} from "@xyflow/react";
import { Plus, Repeat } from "lucide-react";
import { AddStepMenu } from "../AddStepMenu";
import { useWorkflowCanvas } from "./canvas-context";
import type { FlowEdgeData, FlowEdgeTone } from "./workflow-graph";
import "./workflow-canvas.css";

const strokeByTone: Record<FlowEdgeTone, string> = {
	default: "!stroke-border-strong",
	active: "!stroke-primary workflow-edge-flowing",
	done: "!stroke-success/70",
	chain: "!stroke-muted-foreground/50 [stroke-dasharray:4_5]",
	loop: "!stroke-destructive workflow-edge-loop",
};

export function FlowEdge({
	id,
	sourceX,
	sourceY,
	targetX,
	targetY,
	sourcePosition,
	targetPosition,
	data,
}: EdgeProps & { data?: FlowEdgeData }) {
	const { actions, insertStep } = useWorkflowCanvas();
	const tone = data?.tone ?? "default";
	const [path, labelX, labelY] =
		tone === "chain"
			? getBezierPath({
					sourceX,
					sourceY,
					sourcePosition,
					targetX,
					targetY,
					targetPosition,
				})
			: getSmoothStepPath({
					sourceX,
					sourceY,
					sourcePosition,
					targetX,
					targetY,
					targetPosition,
					borderRadius: 18,
					offset: tone === "loop" ? 64 : 24,
				});
	const insertAt = data?.insertAt;

	return (
		<>
			<BaseEdge
				id={id}
				path={path}
				className={cn(strokeByTone[tone], "transition-[stroke] duration-200")}
				style={{ strokeWidth: tone === "loop" ? 2 : 1.5 }}
			/>
			<EdgeLabelRenderer>
				{tone === "loop" ? (
					<Box
						className="nodrag nopan pointer-events-none absolute flex items-center gap-1 rounded-full border border-destructive/30 bg-background px-2 py-0.5 text-[11px] font-semibold text-destructive-on-tint shadow-xs"
						style={{
							transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
						}}
					>
						<Repeat className="size-3" aria-hidden />
						{data?.label}
					</Box>
				) : null}
				{tone === "chain" && data?.label ? (
					<Box
						className="nodrag nopan pointer-events-none absolute max-w-48 truncate rounded-full border border-border bg-background px-2 py-0.5 font-mono text-[10px] text-muted-foreground"
						style={{
							transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
						}}
					>
						{data.label}
					</Box>
				) : null}
				{insertAt !== undefined ? (
					<Box
						className="nodrag nopan pointer-events-auto absolute"
						style={{
							transform: `translate(-50%, -50%) translate(${labelX}px, ${labelY}px)`,
						}}
					>
						<AddStepMenu
							actions={actions}
							onPick={(action) => insertStep(insertAt, action)}
							trigger={
								<button
									type="button"
									aria-label={`Insert a step before step ${insertAt + 1}`}
									className="nodrag nopan flex size-6 items-center justify-center rounded-full border border-border bg-card text-muted-foreground shadow-xs outline-none transition-[transform,color,border-color] duration-150 ease-out hover:border-primary/60 hover:text-primary focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.94] motion-reduce:active:scale-100"
								>
									<Plus className="size-3.5" aria-hidden />
								</button>
							}
						/>
					</Box>
				) : null}
			</EdgeLabelRenderer>
		</>
	);
}

export const workflowEdgeTypes = { flow: FlowEdge };
