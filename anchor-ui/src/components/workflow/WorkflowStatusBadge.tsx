import { WorkflowRunStatus, WorkflowStepStatus } from "@/client";
import {
	Badge,
	type BadgeTone,
} from "@nanostackorg/design-system/components/badge";
import {
	CircleCheck,
	CircleDashed,
	CircleSlash,
	CircleX,
	FlaskConical,
	LoaderCircle,
} from "lucide-react";

type Status = WorkflowRunStatus | WorkflowStepStatus;

const appearance: Record<
	Status,
	{ tone: BadgeTone; label: string; icon: typeof CircleCheck }
> = {
	[WorkflowRunStatus.SUCCEEDED]: {
		tone: "success",
		label: "Succeeded",
		icon: CircleCheck,
	},
	[WorkflowRunStatus.FAILED]: {
		tone: "critical",
		label: "Failed",
		icon: CircleX,
	},
	[WorkflowRunStatus.SKIPPED]: {
		tone: "neutral",
		label: "Skipped",
		icon: CircleSlash,
	},
	[WorkflowRunStatus.RUNNING]: {
		tone: "info",
		label: "Running",
		icon: LoaderCircle,
	},
	[WorkflowStepStatus.SIMULATED]: {
		tone: "info",
		label: "Simulated",
		icon: FlaskConical,
	},
};

export function WorkflowStatusBadge({ status }: { status: Status }) {
	const { tone, label, icon } = appearance[status] ?? {
		tone: "neutral",
		label: status,
		icon: CircleDashed,
	};
	return (
		<Badge variant="soft" tone={tone} icon={icon}>
			{label}
		</Badge>
	);
}
