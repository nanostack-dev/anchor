import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, within } from "storybook/test";
import LocalEditor from "./LocalEditor";

function EditorFixture({
	initialValue,
	narrow = false,
	largeText = false,
}: { initialValue: string; narrow?: boolean; largeText?: boolean }) {
	const [value, setValue] = useState(initialValue);
	return (
		<Box
			as="section"
			aria-label="HTML editor"
			className={narrow ? "h-80 w-80" : "h-80 w-full"}
		>
			<LocalEditor
				value={value}
				onChange={(next) => setValue(next ?? "")}
				height="100%"
				options={{
					fontSize: largeText ? 24 : 12,
					automaticLayout: true,
					minimap: { enabled: false },
					wordWrap: "on",
					scrollBeyondLastLine: false,
				}}
			/>
		</Box>
	);
}

const meta = {
	title: "Email/LocalEditor",
	component: EditorFixture,
	parameters: { layout: "fullscreen" },
	args: { initialValue: "<h1>Hello {{ .name }}</h1>" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole(
				"textbox",
				{ name: /Editor content/ },
				{ timeout: 5000 },
			),
		).toBeVisible();
	},
} satisfies Meta<typeof EditorFixture>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = { args: { initialValue: "" } };
export const One: Story = {};

export const WorstCase: Story = {
	args: {
		initialValue: [
			"<!doctype html>",
			'<html lang="en"><body>',
			"<h1>Invoice for Aleksandra Wiśniewska-Kowalczyk</h1>",
			"<p>王秀英 — نور الهدى عبد الرحمن — {{ .customer.name }}</p>",
			'<a href="https://example.test/workspaces/acme/projects/q3-launch/docs/invoice?region=international&amp;currency=EUR">View invoice</a>',
			"<table><tbody>",
			...Array.from(
				{ length: 200 },
				(_, index) =>
					`<tr><td>Subscription line ${index + 1}</td><td>{{ .items.${index}.total }}</td></tr>`,
			),
			"</tbody></table></body></html>",
		].join("\n"),
	},
	play: async (context) => {
		await meta.play(context);
		await expect(
			await within(context.canvasElement).findByText(
				/Invoice/,
				{},
				{ timeout: 5000 },
			),
		).toBeVisible();
	},
};

export const NarrowWorstCase: Story = {
	...WorstCase,
	args: { ...WorstCase.args, narrow: true },
};

export const LargeTextWorstCase: Story = {
	...NarrowWorstCase,
	args: { ...NarrowWorstCase.args, largeText: true },
};
