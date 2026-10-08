import type { WorkflowActionResponse } from "@/client";
import { createContext, useContext } from "react";
import type { Selection } from "./workflow-graph";

/** The drag payload a palette action carries: its action type. */
export const ACTION_DRAG_TYPE = "application/x-anchor-workflow-action";

export const carriesAction = (event: React.DragEvent) =>
	event.dataTransfer.types.includes(ACTION_DRAG_TYPE);

export interface WorkflowCanvasActions {
	actions: WorkflowActionResponse[];
	select: (selection: Selection) => void;
	insertStep: (index: number, action: WorkflowActionResponse) => void;
	moveStep: (from: number, to: number) => void;
	openWorkflow?: (workflowId: string) => void;
}

export interface WorkflowCanvasContextValue extends WorkflowCanvasActions {
	entranceDelay: (order: number) => number;
	/** A palette action is being dragged over the page. */
	draggingAction: boolean;
	/** Inserts the action a drop event carries at `index`. */
	dropAction: (index: number, event: React.DragEvent) => void;
	/** Pans the canvas until `element` is in view. */
	reveal: (element: HTMLElement) => void;
}

export const WorkflowCanvasContext = createContext<WorkflowCanvasContextValue>({
	actions: [],
	select: () => {},
	insertStep: () => {},
	moveStep: () => {},
	entranceDelay: () => 0,
	draggingAction: false,
	dropAction: () => {},
	reveal: () => {},
});

export const useWorkflowCanvas = () => useContext(WorkflowCanvasContext);
