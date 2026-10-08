import { StripeIntegrationAuthMethod } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, within } from "storybook/test";
import { StripeConfigForm } from "./StripeConfigForm";

const meta = {
	title: "Integrations/Stripe Form",
	component: StripeConfigForm,
	parameters: { layout: "padded" },
	args: {
		form: {
			authMethod: StripeIntegrationAuthMethod.API_KEY,
			accountId: "",
			apiKey: "",
			webhookSecret: "",
			returnUrl: "http://localhost:3307",
			enabled: true,
		},
		setField: fn(),
		errors: {},
		disabled: false,
		existing: false,
		apiKeyConfigured: false,
		webhookSecretConfigured: false,
	},
} satisfies Meta<typeof StripeConfigForm>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Empty: Story = {};
export const ExistingSecrets: Story = {
	args: {
		existing: true,
		apiKeyConfigured: true,
		webhookSecretConfigured: true,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByLabelText("Stripe test API key")).toHaveValue("");
		await expect(canvas.getByLabelText("Webhook signing secret")).toHaveValue(
			"",
		);
		await expect(canvas.getByRole("switch", { name: "Enabled" })).toBeChecked();
	},
};
export const LocalCLI: Story = {
	args: {
		form: {
			...meta.args.form,
			authMethod: StripeIntegrationAuthMethod.LOCAL_CLI,
			accountId: "acct_sandbox",
		},
		existing: true,
		webhookSecretConfigured: true,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.queryByLabelText("Stripe test API key"),
		).not.toBeInTheDocument();
		await expect(
			canvas.getByRole("combobox", { name: "Connection method" }),
		).toHaveTextContent("Connected Stripe CLI");
	},
};
export const WorstCase: Story = {
	args: {
		form: {
			...meta.args.form,
			accountId: `acct_${"InternationalSandbox".repeat(5)}`,
			returnUrl: `https://example.com/${"international/".repeat(20)}`,
			enabled: false,
		},
		existing: true,
		errors: {
			accountId:
				"This account could not be verified. Check the connected Stripe sandbox account and try again.",
		},
	},
};
