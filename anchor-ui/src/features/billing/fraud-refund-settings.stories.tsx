import { Button, Stack } from "@nanostackorg/design-system";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import {
	FraudRefundCurrency,
	type State,
	type UpdateSettingsRequest,
} from "./billing-types";
import {
	createFixtureAPI,
	fixtureState,
	fraudEnabledFixtureState,
	fraudWorstCaseFixtureState,
} from "./fixtures";
import { FraudRefundSettings } from "./fraud-refund-settings";

function FixtureRefundSettings({
	state,
	connectionAvailable = true,
	failRefreshOnce = false,
	failSaveOnce = false,
	holdInitial = false,
	onUpdate,
}: {
	state: State;
	connectionAvailable?: boolean;
	failRefreshOnce?: boolean;
	failSaveOnce?: boolean;
	holdInitial?: boolean;
	onUpdate: (body: UpdateSettingsRequest) => void;
}) {
	const [initialGate] = useState(() => {
		let release = () => {};
		const promise = new Promise<void>((resolve) => {
			release = resolve;
		});
		return { promise, release };
	});
	const [api] = useState(() => {
		const fixture = createFixtureAPI(state);
		let loads = 0;
		let saves = 0;
		return {
			...fixture,
			load: async () => {
				loads += 1;
				if (holdInitial && loads === 1) await initialGate.promise;
				if (failRefreshOnce && loads === 2)
					throw new Error(
						"Billing is temporarily unavailable. Reload before making changes.",
					);
				return fixture.load();
			},
			updateSettings: async (body: UpdateSettingsRequest) => {
				saves += 1;
				onUpdate(body);
				if (failSaveOnce && saves === 1)
					throw new Error(
						"The refund policy could not be saved. Try again after the service recovers.",
					);
				return fixture.updateSettings(body);
			},
		};
	});
	const [client] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	return (
		<QueryClientProvider client={client}>
			<Stack space="lg">
				{holdInitial && (
					<Button onClick={initialGate.release}>Release initial policy</Button>
				)}
				<FraudRefundSettings
					api={api}
					productId={state.product.id}
					connectionAvailable={connectionAvailable}
				/>
			</Stack>
		</QueryClientProvider>
	);
}

const meta = {
	title: "Billing/Fraud Refund Settings",
	component: FixtureRefundSettings,
	parameters: { layout: "padded" },
	args: { state: fixtureState, onUpdate: fn() },
} satisfies Meta<typeof FixtureRefundSettings>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("switch", {
					name: "Automatically refund fraud warnings",
				}),
			).not.toHaveAttribute("aria-disabled", "true"),
		);
		await expect(
			canvas.getByRole("switch", {
				name: "Automatically refund fraud warnings",
			}),
		).not.toBeChecked();
		await expect(
			canvas.getByLabelText("Maximum payment amount (minor units)"),
		).toHaveValue("0");
		await expect(
			canvas.getByText("No fraud refund activity yet"),
		).toBeVisible();
	},
};

export const ValidationSaveAndReload: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		const toggle = canvas.getByRole("switch", {
			name: "Automatically refund fraud warnings",
		});
		await waitFor(() =>
			expect(toggle).not.toHaveAttribute("aria-disabled", "true"),
		);
		await userEvent.click(toggle);
		await userEvent.click(
			canvas.getByRole("button", { name: "Save refund policy" }),
		);
		await expect(
			await canvas.findByText(
				"Enter a whole amount greater than zero before enabling refunds.",
			),
		).toBeVisible();
		await expect(args.onUpdate).not.toHaveBeenCalled();
		const amount = canvas.getByLabelText(
			"Maximum payment amount (minor units)",
		);
		await userEvent.clear(amount);
		await userEvent.type(amount, "1500");
		await userEvent.click(
			canvas.getByRole("combobox", { name: "Refund currency" }),
		);
		const options = await within(document.body).findByRole("listbox", {
			name: "Refund currency options",
		});
		await userEvent.click(within(options).getByRole("option", { name: "CAD" }));
		await waitFor(() =>
			expect(
				within(document.body).queryByRole("listbox", {
					name: "Refund currency options",
				}),
			).not.toBeInTheDocument(),
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "Save refund policy" }),
		);
		await expect(await canvas.findByText("Refund policy saved.")).toBeVisible();
		await expect(args.onUpdate).toHaveBeenLastCalledWith({
			fraud_refund_policy: {
				enabled: true,
				currency: FraudRefundCurrency.CAD,
				max_amount: 1500,
			},
		});
		await userEvent.click(
			canvas.getByRole("button", { name: "Reload refund policy" }),
		);
		await expect(amount).toHaveValue("1500");
		await expect(toggle).toBeChecked();
		await expect(
			canvas.getByRole("combobox", { name: "Refund currency" }),
		).toHaveTextContent("CAD");
	},
};

export const InitialLoading: Story = {
	args: { state: fraudEnabledFixtureState, holdInitial: true },
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		const save = canvas.getByRole("button", { name: "Save refund policy" });
		await expect(save).toBeDisabled();
		await expect(
			canvas.getByLabelText("Maximum payment amount (minor units)"),
		).toBeDisabled();
		await expect(args.onUpdate).not.toHaveBeenCalled();
		await userEvent.click(
			canvas.getByRole("button", { name: "Release initial policy" }),
		);
		await waitFor(() => expect(save).toBeEnabled());
		await expect(
			canvas.getByLabelText("Maximum payment amount (minor units)"),
		).toHaveValue("1500");
		await expect(
			canvas.getByRole("switch", {
				name: "Automatically refund fraud warnings",
			}),
		).toBeChecked();
	},
};

export const FailedRefresh: Story = {
	args: { state: fraudEnabledFixtureState, failRefreshOnce: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const amount = canvas.getByLabelText(
			"Maximum payment amount (minor units)",
		);
		await waitFor(() => expect(amount).toHaveValue("1500"));
		await userEvent.clear(amount);
		await userEvent.type(amount, "2500");
		await userEvent.click(
			canvas.getByRole("button", { name: "Reload refund policy" }),
		);
		await expect(
			await canvas.findByText(
				"Billing is temporarily unavailable. Reload before making changes.",
			),
		).toBeVisible();
		await expect(amount).toHaveValue("2500");
		await expect(
			canvas.getByRole("button", { name: "Save refund policy" }),
		).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: "Save refund policy" }),
		).not.toHaveAttribute("aria-busy", "true");
		await expect(canvas.getByText("Refunded", { exact: true })).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Reload refund policy" }),
		).toBeEnabled();
		await userEvent.click(
			canvas.getByRole("button", { name: "Reload refund policy" }),
		);
		await waitFor(() =>
			expect(
				canvas.getByRole("button", { name: "Save refund policy" }),
			).toBeEnabled(),
		);
		await expect(amount).toHaveValue("2500");
		await userEvent.click(
			canvas.getByRole("button", { name: "Save refund policy" }),
		);
		await expect(await canvas.findByText("Refund policy saved.")).toBeVisible();
	},
};

export const FailedSave: Story = {
	args: { state: fraudEnabledFixtureState, failSaveOnce: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const amount = canvas.getByLabelText(
			"Maximum payment amount (minor units)",
		);
		await waitFor(() => expect(amount).toHaveValue("1500"));
		await userEvent.clear(amount);
		await userEvent.type(amount, "2500");
		await userEvent.click(
			canvas.getByRole("button", { name: "Save refund policy" }),
		);
		await expect(
			await canvas.findByText(
				"The refund policy could not be saved. Try again after the service recovers.",
			),
		).toBeVisible();
		await expect(amount).toHaveValue("2500");
		await userEvent.click(
			canvas.getByRole("button", { name: "Save refund policy" }),
		);
		await expect(await canvas.findByText("Refund policy saved.")).toBeVisible();
	},
};

export const One: Story = {
	args: {
		state: {
			...structuredClone(fraudEnabledFixtureState),
			fraud_refunds: [fraudEnabledFixtureState.fraud_refunds[0]],
		},
	},
	play: async ({ canvasElement }) => {
		const list = await within(canvasElement).findByRole("list", {
			name: "Fraud refund activity",
		});
		await expect(within(list).getAllByRole("listitem")).toHaveLength(1);
		await expect(within(list).getByText("CAD 12.00")).toBeVisible();
	},
};

export const WorstCase: Story = {
	args: { state: fraudWorstCaseFixtureState },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("EUR 999,999.99")).toBeVisible();
		await expect(canvas.getByText("Amount unavailable")).toBeVisible();
		await expect(
			canvas.getAllByText("Needs review", { exact: true }),
		).toHaveLength(2);
	},
};

export const PausedConnection: Story = {
	args: { state: fraudEnabledFixtureState, connectionAvailable: false },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByText("Refunded", { exact: true }),
		).toBeVisible();
		await expect(
			canvas.getByRole("switch", {
				name: "Automatically refund fraud warnings",
			}),
		).toHaveAttribute("aria-disabled", "true");
		await expect(
			canvas.getByRole("button", { name: "Save refund policy" }),
		).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: "Reload refund policy" }),
		).toBeEnabled();
	},
};
