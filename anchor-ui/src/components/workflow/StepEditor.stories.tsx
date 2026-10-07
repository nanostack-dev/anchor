import type { WorkflowStep } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, within } from "storybook/test";
import { StepEditor, type StepStart } from "./StepEditor";
import { fixtureCatalog, worstCaseStep } from "./workflow-fixtures";
import { variablesBeforeStep } from "./workflow-model";

function StatefulStepEditor({
	initial,
	errors,
	starts,
}: {
	initial: WorkflowStep;
	errors?: Record<string, string>;
	starts?: StepStart[];
}) {
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
			errors={errors}
			starts={starts}
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

/**
 * A server error pinned to its parameter, and the workflows the step's event
 * starts, with names long enough to wrap.
 */
export const ErrorsAndStarts: Story = {
	args: {
		initial: {
			id: "workspace",
			action: "workspace.create",
			params: { name: "General" },
			when: [],
		},
		errors: {
			organization_id:
				'Step 2 · Organization: Parameter "organization_id" of action workspace.create is required.',
		},
		starts: [
			{
				event: "workspace.created",
				workflowNames: [
					"Provision enterprise onboarding: workspace, license, owner membership and welcome email for EMEA",
					"Jo",
				],
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText(
				/Parameter "organization_id" of action workspace.create is required/,
			),
		).toBeInTheDocument();
		await expect(
			canvas.getByRole("list", { name: "Events step 2 emits" }),
		).toHaveTextContent("“Jo”");
	},
};
