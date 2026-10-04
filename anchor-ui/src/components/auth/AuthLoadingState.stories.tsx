import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor, within } from "storybook/test";
import { AuthLoadingState } from "./AuthLoadingState";
const meta = {
	title: "Auth/Loading",
	component: AuthLoadingState,
	parameters: { layout: "fullscreen" },
	args: { authLoading: true, tenantLoading: true },
} satisfies Meta<typeof AuthLoadingState>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Initializing: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(canvas.getByText("Initializing application...")).toBeVisible(),
		);
		await expect(
			canvas.getByText("Authentication in progress..."),
		).toBeVisible();
	},
};
export const AuthenticationVerified: Story = {
	args: { authLoading: false, tenantLoading: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(canvas.getByText("Checking system status...")).toBeVisible(),
		);
		await expect(canvas.getByText("Authentication verified")).toBeVisible();
	},
};
