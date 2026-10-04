import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { AuthErrorBoundary } from "./AuthErrorBoundary";
const meta = {
	title: "Auth/Error",
	component: AuthErrorBoundary,
	parameters: { layout: "fullscreen" },
	args: {
		error: {
			type: "network",
			message: "Connection could not be established.",
			retryable: true,
			timestamp: Date.parse("2026-01-01T12:00:00Z"),
		},
		onRetry: fn(),
		onLogout: fn(),
	},
} satisfies Meta<typeof AuthErrorBoundary>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Retryable: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("heading", { name: "Connection Error" }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Try Again" }));
		await expect(args.onRetry).toHaveBeenCalledOnce();
		await userEvent.click(canvas.getByRole("button", { name: "Logout" }));
		await expect(args.onLogout).toHaveBeenCalledOnce();
	},
};
export const SessionExpired: Story = {
	args: {
		error: {
			type: "token",
			message: "Your session is no longer valid.",
			retryable: false,
			timestamp: Date.parse("2026-01-01T12:00:00Z"),
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("heading", { name: "Session Expired" }),
		).toBeVisible();
		await expect(
			canvas.queryByRole("button", { name: "Try Again" }),
		).not.toBeInTheDocument();
	},
};
