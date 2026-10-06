import {
	type EmailTemplateResponse,
	type EmailTemplateVersionResponse,
	EmailTemplateVersionStatus,
	EmailVariableType,
	type TemplateExample,
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
const saveExamplesRequest = fn();
const examplesRequest = fn();
const releaseExamples = fn<() => void>();
const savedExample = {
	id: "ex_saved",
	name: "Welcome for Aleksandra Wiśniewska-Kowalczyk",
	variables: { name: "Aleksandra Wiśniewska-Kowalczyk" },
} satisfies TemplateExample;
const worstCaseExamples = [
	savedExample,
	{ id: "ex_short", name: "Jo", variables: { name: "Jo" } },
	{
		id: "ex_cjk",
		name: "王秀英 — 国際請求書のプレビュー",
		variables: { name: "王秀英" },
	},
] satisfies TemplateExample[];

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
	beforeEach: ({ parameters }) => {
		saveRequest.mockClear();
		saveExamplesRequest.mockClear();
		examplesRequest.mockClear();
		releaseExamples.mockReset();
		let examples: TemplateExample[] = parameters.initialExamples ?? [];
		let exampleFailures = parameters.failExamplesOnce ? 1 : 0;
		const examplesReady = parameters.deferExamples
			? new Promise<void>((resolve) =>
					releaseExamples.mockImplementation(resolve),
				)
			: Promise.resolve();
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
				if (url.pathname.endsWith("/examples")) {
					if (request.method === "PUT") {
						const payload: { examples: TemplateExample[] } =
							await request.json();
						saveExamplesRequest(payload);
						examples = payload.examples;
					} else {
						examplesRequest();
						if (exampleFailures > 0) {
							exampleFailures -= 1;
							return Response.json(
								{
									errors: [
										{
											code: "INTERNAL_ERROR",
											message: "Examples storage unavailable",
										},
									],
								},
								{ status: 503 },
							);
						}
						await examplesReady;
					}
					return Response.json({ examples });
				}
				if (url.pathname.endsWith(`/templates/${template.id}`))
					return Response.json(template);
				throw new Error(`Unexpected request ${request.method} ${url.pathname}`);
			},
		});
		return () => {
			releaseExamples();
			client.setConfig(previous);
		};
	},
} satisfies Meta<typeof BuilderHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ExamplesLoading: Story = {
	parameters: { deferExamples: true, initialExamples: [savedExample] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("textbox", { name: "Subject" });
		await userEvent.click(canvas.getByRole("tab", { name: "Examples" }));
		await waitFor(() => expect(examplesRequest).toHaveBeenCalledTimes(1));
	},
};

export const FailedExamplesRetryWithoutReplacingSavedData: Story = {
	parameters: { failExamplesOnce: true, initialExamples: [savedExample] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("textbox", { name: "Subject" });
		await userEvent.click(canvas.getByRole("tab", { name: "Examples" }));
		const panel = within(canvas.getByRole("tabpanel", { name: "Examples" }));
		await expect(
			await panel.findByText("Could not load examples.", { exact: true }),
		).toBeVisible();
		await expect(
			panel.getByRole("button", { name: "New Example" }),
		).toBeDisabled();
		await expect(
			panel.getByRole("button", { name: "Save Examples" }),
		).toBeDisabled();
		await expect(
			panel.queryByText("No examples yet", { exact: true }),
		).not.toBeInTheDocument();
		await userEvent.click(panel.getByRole("button", { name: "Retry" }));
		await waitFor(() => expect(examplesRequest).toHaveBeenCalledTimes(2));
		await waitFor(() =>
			expect(panel.getByPlaceholderText("Example name")).toHaveValue(
				savedExample.name,
			),
		);
		await expect(panel.getByPlaceholderText("value for .name")).toHaveValue(
			"Aleksandra Wiśniewska-Kowalczyk",
		);
		await expect(
			panel.getByRole("button", { name: "New Example" }),
		).toBeEnabled();
		await expect(
			panel.getByRole("button", { name: "Save Examples" }),
		).toBeEnabled();
		await expect(
			panel.queryByText("Could not load examples.", { exact: true }),
		).not.toBeInTheDocument();
	},
};

export const Empty: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("textbox", { name: "Subject" });
		await userEvent.click(canvas.getByRole("tab", { name: "Examples" }));
		const panel = within(canvas.getByRole("tabpanel", { name: "Examples" }));
		await waitFor(() =>
			expect(panel.getByRole("button", { name: "New Example" })).toBeEnabled(),
		);
		await expect(
			panel.getByText("No examples yet", { exact: true }),
		).toBeVisible();
		await expect(
			panel.getByRole("button", { name: "Save Examples" }),
		).toBeEnabled();
	},
};

export const One: Story = {
	parameters: { initialExamples: [savedExample] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("textbox", { name: "Subject" });
		await userEvent.click(canvas.getByRole("tab", { name: "Examples" }));
		const panel = within(canvas.getByRole("tabpanel", { name: "Examples" }));
		await waitFor(() =>
			expect(panel.getByPlaceholderText("Example name")).toHaveValue(
				savedExample.name,
			),
		);
		await expect(
			panel.getByText("Examples (1)", { exact: true }),
		).toBeVisible();
		await expect(panel.getByPlaceholderText("value for .name")).toHaveValue(
			"Aleksandra Wiśniewska-Kowalczyk",
		);
	},
};

export const WorstCase: Story = {
	parameters: { initialExamples: worstCaseExamples },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("textbox", { name: "Subject" });
		await userEvent.click(canvas.getByRole("tab", { name: "Examples" }));
		const panel = within(canvas.getByRole("tabpanel", { name: "Examples" }));
		await waitFor(() =>
			expect(panel.getByPlaceholderText("Example name")).toHaveValue(
				savedExample.name,
			),
		);
		await expect(
			panel.getByText("Examples (3)", { exact: true }),
		).toBeVisible();
		for (const example of worstCaseExamples) {
			await userEvent.click(panel.getByRole("button", { name: example.name }));
			await expect(panel.getByPlaceholderText("Example name")).toHaveValue(
				example.name,
			);
			await expect(panel.getByPlaceholderText("value for .name")).toHaveValue(
				example.variables.name,
			);
		}
		await userEvent.click(panel.getByRole("button", { name: "Save Examples" }));
		await waitFor(() =>
			expect(saveExamplesRequest).toHaveBeenCalledWith({
				examples: worstCaseExamples,
			}),
		);
	},
};

export const DelayedExamplesPreserveSavedAndNew: Story = {
	parameters: { deferExamples: true, initialExamples: [savedExample] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("textbox", { name: "Subject" });
		await userEvent.click(canvas.getByRole("tab", { name: "Examples" }));
		const panel = within(canvas.getByRole("tabpanel", { name: "Examples" }));
		await waitFor(() => expect(examplesRequest).toHaveBeenCalledTimes(1));
		await expect(
			panel.getByRole("button", { name: "New Example" }),
		).toBeDisabled();
		await expect(
			panel.getByRole("button", { name: "Save Examples" }),
		).toBeDisabled();
		await expect(
			panel.getByText("Loading examples…", { exact: true }),
		).toBeVisible();
		await expect(
			panel.queryByText("No examples yet", { exact: true }),
		).not.toBeInTheDocument();
		releaseExamples();
		await waitFor(() =>
			expect(panel.getByRole("button", { name: "New Example" })).toBeEnabled(),
		);
		await expect(panel.getByPlaceholderText("Example name")).toHaveValue(
			savedExample.name,
		);
		await expect(panel.getByPlaceholderText("value for .name")).toHaveValue(
			"Aleksandra Wiśniewska-Kowalczyk",
		);
		await userEvent.click(panel.getByRole("button", { name: "New Example" }));
		const name = panel.getByPlaceholderText("Example name");
		await userEvent.clear(name);
		await userEvent.click(name);
		await userEvent.paste("Invoice preview for 王秀英");
		const value = panel.getByPlaceholderText("value for .name");
		await userEvent.clear(value);
		await userEvent.click(value);
		await userEvent.paste("王秀英");
		await userEvent.click(panel.getByRole("button", { name: "Save Examples" }));
		await waitFor(() =>
			expect(saveExamplesRequest).toHaveBeenCalledWith({
				examples: [
					savedExample,
					{
						id: expect.any(String),
						name: "Invoice preview for 王秀英",
						variables: { name: "王秀英" },
					},
				],
			}),
		);
		await expect(
			panel.getByRole("button", { name: savedExample.name }),
		).toBeVisible();
		await expect(panel.getByPlaceholderText("Example name")).toHaveValue(
			"Invoice preview for 王秀英",
		);
	},
};

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
