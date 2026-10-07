import type { WorkflowStep } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, within } from "storybook/test";
import { StepEditor } from "./StepEditor";
import { fixtureCatalog, worstCaseStep } from "./workflow-fixtures";
import { variablesBeforeStep } from "./workflow-model";

function StatefulStepEditor({ initial }: { initial: WorkflowStep }) {
	const [step, setStep] = useState(initial);
	const draft = {
		name: "",
		enabled: true,
		trigger_event_type: "product_user.created",
		definition: {
			conditions: [],
			steps: [
				{
					id: "read_the_invited_person_before_anything_x",
					action: "product_user.get",
					params: {},
				},
				step,
			],
		},
	};
	return (
		<StepEditor
			step={step}
			index={1}
			count={2}
			catalog={fixtureCatalog}
			variables={variablesBeforeStep(fixtureCatalog, draft, 1)}
			resources={{}}
			onChange={setStep}
			onMove={fn()}
			onRemove={fn()}
		/>
	);
}

const meta = {
	title: "Workflow/StepEditor",
	component: StatefulStepEditor,
	args: {
		initial: {
			id: "workspace",
			action: "workspace.create",
			params: {
				organization_id: "{{event.data.organization_id}}",
				name: "General",
			},
			when: [],
		},
	},
} satisfies Meta<typeof StatefulStepEditor>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Demo: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByRole("textbox", { name: "Name *" })).toHaveValue(
			"General",
		);
	},
};

/**
 * A 40-character step id (the maximum) whose outputs become long chips, a long
 * step name, an unbreakable URL inside JSON metadata and an `in` condition.
 */
export const WorstCase: Story = {
	args: { initial: worstCaseStep },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText(`steps.${worstCaseStep.id}.organization_id`),
		).toBeInTheDocument();
		await expect(canvas.getByRole("switch")).toBeChecked();
	},
};

export const UnknownAction: Story = {
	args: { initial: { id: "archive", action: "workspace.archive", params: {} } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("This action is not in the catalog any more."),
		).toBeInTheDocument();
	},
};
