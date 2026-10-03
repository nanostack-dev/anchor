import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, screen, userEvent, waitFor, within } from "storybook/test";
import { SmtpConfigForm } from "./SmtpConfigForm";
const meta = {
	title: "Integrations/SMTP Form",
	component: SmtpConfigForm,
	args: {
		form: {
			host: "smtp.example.com",
			port: "587",
			encryption: "STARTTLS",
			authMethod: "PLAIN",
			username: "mailer@example.com",
			password: "",
			fromAddress: "mail@example.com",
			fromName: "Anchor",
			replyTo: "",
			enabled: false,
		},
		setField: fn(),
		errors: {},
		isNew: true,
	},
	parameters: { layout: "padded" },
} satisfies Meta<typeof SmtpConfigForm>;
export default meta;
type Story = StoryObj<typeof meta>;
export const NewConnection: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByRole("textbox", { name: "Host *" })).toHaveValue(
			"smtp.example.com",
		);
		await expect(canvas.getByLabelText("Password *")).toHaveValue("");
		await expect(
			canvas.getByRole("combobox", { name: "Encryption" }),
		).toHaveTextContent("STARTTLS (587)");
		await expect(
			canvas.getByRole("combobox", { name: "Auth Method" }),
		).toHaveTextContent("PLAIN");
		await userEvent.click(canvas.getByRole("combobox", { name: "Encryption" }));
		const encryptionOptions = await screen.findByRole("listbox", {
			name: "Encryption options",
		});
		await expect(encryptionOptions).toBeVisible();
		await userEvent.click(
			within(encryptionOptions).getByRole("option", {
				name: "Implicit TLS (465)",
			}),
		);
		await expect(args.setField).toHaveBeenCalledWith("encryption", "TLS");
		await waitFor(() =>
			expect(screen.queryByRole("listbox")).not.toBeInTheDocument(),
		);
		await userEvent.click(
			canvas.getByRole("combobox", { name: "Auth Method" }),
		);
		const authenticationOptions = await screen.findByRole("listbox", {
			name: "Authentication method options",
		});
		await expect(authenticationOptions).toBeVisible();
		await userEvent.click(
			within(authenticationOptions).getByRole("option", { name: "LOGIN" }),
		);
		await expect(args.setField).toHaveBeenCalledWith("authMethod", "LOGIN");
		await waitFor(() =>
			expect(screen.queryByRole("listbox")).not.toBeInTheDocument(),
		);
	},
};
export const ExistingSecret: Story = {
	args: { isNew: false },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const secret = canvas.getByLabelText("Password");
		await expect(secret).toHaveValue("");
		await expect(secret).toHaveAttribute(
			"placeholder",
			"Leave blank to keep existing",
		);
	},
};
