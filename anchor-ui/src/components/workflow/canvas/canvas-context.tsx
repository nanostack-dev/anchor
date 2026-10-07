import type { WorkflowActionResponse } from "@/client";
import { createContext, useContext } from "react";
import type { Selection } from "./workflow-graph";

export interface WorkflowCanvasActions {
	actions: WorkflowActionResponse[];
	select: (selection: Selection) => void;
	insertStep: (index: number, action: WorkflowActionResponse) => void;
	openWorkflow?: (workflowId: string) => void;
	entranceDelay: (order: number) => number;
}

export const WorkflowCanvasContext = createContext<WorkflowCanvasActions>({
	actions: [],
	select: () => {},
	insertStep: () => {},
	entranceDelay: () => 0,
});

export const useWorkflowCanvas = () => useContext(WorkflowCanvasContext);
