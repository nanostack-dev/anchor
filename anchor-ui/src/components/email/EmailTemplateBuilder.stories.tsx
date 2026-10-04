import {
	type EmailTemplateResponse,
	type EmailTemplateVersionResponse,
	EmailTemplateVersionStatus,
	EmailVariableType,
	client,
} from "@/client";
import { StoryRouter } from "@/lib/storybook/story-router";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { EmailTemplateBuilder } from "./EmailTemplateBuilder";

const timestamps = {
	created_at: "2026-10-03T12:00:00Z",
	updated_at: "2026-10-03T12:00:00Z",
};
const template: EmailTemplateResponse = {
	...timestamps,
	id: "emt_story",
	product_id: "prd_story",
	slug: "welcome",
	name: "Welcome email",
	is_active: true,
	draft_version_id: "emv_story",
};
const draft: EmailTemplateVersionResponse = {
	...timestamps,
	id: "emv_story",
	template_id: template.id,
	version_number: 1,
	status: EmailTemplateVersionStatus.DRAFT,
	subject: "Welcome {{ .name }}",
	body_html: "<h1>Hello {{ .name }}</h1>",
	variables: [
		{ name: "name", type: EmailVariableType.STRING, required: false },
	],
};
const saveRequest = fn();

function BuilderHarness() {
	const [queryClient] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	return (
		<StoryRouter>
			<QueryClientProvider client={queryClient}>
				<Box className="h-[640px] w-[1000px] max-w-full">
					<EmailTemplateBuilder
						productId={template.product_id}
						templateId={template.id}
					/>
				</Box>
			</QueryClientProvider>
		</StoryRouter>
	);
}

const meta = {
	title: "Email/EmailTemplateBuilder",
	component: BuilderHarness,
	parameters: { layout: "fullscreen" },
	beforeEach: () => {
		saveRequest.mockClear();
		const previous = client.getConfig();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				const url = new URL(request.url);
				if (url.pathname.endsWith("/integrations"))
					return Response.json({ items: [] });
				if (url.pathname.endsWith("/draft")) {
					if (request.method === "PUT") saveRequest(await request.json());
					return Response.json(draft);
				}
				if (url.pathname.endsWith("/preview"))
					return Response.json({
						subject: "Welcome",
						body_html: "<h1>Hello</h1>",
						warnings: [],
					});
				if (url.pathname.endsWith("/examples"))
					return Response.json({ examples: [] });
				if (url.pathname.endsWith(`/templates/${template.id}`))
					return Response.json(template);
				throw new Error(`Unexpected request ${request.method} ${url.pathname}`);
			},
		});
		return () => client.setConfig(previous);
	},
} satisfies Meta<typeof BuilderHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const BoundedEditorAndDraftSurviveTabSwitch: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const subject = await canvas.findByRole("textbox", { name: "Subject" });
		await expect(
			canvas.getByRole("textbox", { name: "Template name" }),
		).toHaveValue("Welcome email");
		const editor = canvas.getByRole("region", { name: "HTML editor" });
		await expect(
			canvas.getByRole("button", { name: "Copy HTML" }),
		).toBeEnabled();
		await waitFor(() => {
			expect(editor.getBoundingClientRect().height).toBeGreaterThan(200);
			expect(editor.getBoundingClientRect().height).toBeLessThan(640);
		});
		await userEvent.clear(subject);
		await userEvent.click(subject);
		await userEvent.paste("Updated {{ .name }}");
		await userEvent.click(canvas.getByRole("tab", { name: "Variables (1)" }));
		const variableType = canvas.getByRole("combobox", {
			name: "Type for variable name",
		});
		await expect(variableType).toHaveTextContent(EmailVariableType.STRING);
		await userEvent.click(variableType);
		await expect(
			await within(canvasElement.ownerDocument.body).findByRole("listbox", {
				name: "Type for variable name options",
			}),
		).toBeVisible();
		await userEvent.click(
			await within(canvasElement.ownerDocument.body).findByRole("option", {
				name: EmailVariableType.NUMBER,
			}),
		);
		await expect(variableType).toHaveTextContent(EmailVariableType.NUMBER);
		await userEvent.click(canvas.getByRole("checkbox", { name: "Req" }));
		await userEvent.click(canvas.getByRole("tab", { name: "Content" }));
		await expect(canvas.getByRole("textbox", { name: "Subject" })).toHaveValue(
			"Updated {{ .name }}",
		);
		await waitFor(() =>
			expect(saveRequest).toHaveBeenCalledWith(
				expect.objectContaining({
					subject: "Updated {{ .name }}",
					body_html: draft.body_html,
					variables: [
						expect.objectContaining({
							name: "name",
							type: EmailVariableType.NUMBER,
							required: true,
						}),
					],
				}),
			),
		);
		await expect(
			canvas
				.getByRole("region", { name: "HTML editor" })
				.getBoundingClientRect().height,
		).toBeGreaterThan(200);
	},
};
