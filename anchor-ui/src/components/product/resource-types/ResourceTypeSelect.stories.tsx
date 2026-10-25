import type { ProductResourceTypeResponse } from "@/client";
import { client } from "@/client/client.gen";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { ResourceTypeSelect } from "./ResourceTypeSelect";

const PRODUCT_ID = "prd_2Nq8xKf3pLmR";

function resourceType(
	slug: string,
	name: string,
	parentTypes: string[] = ["organization"],
): ProductResourceTypeResponse {
	return {
		id: `rtype_${slug}`,
		product_id: PRODUCT_ID,
		slug,
		name,
		parent_types: parentTypes,
		nests_in_itself: false,
		custom_roles: false,
		built_in: slug === "organization",
		created_at: "2026-10-24T09:12:31Z",
		updated_at: "2026-10-24T09:12:31Z",
	};
}

const ORGANIZATION = resourceType("organization", "Organization", []);

type Reply = ProductResourceTypeResponse[] | "fail" | "pending";

function mockResourceTypes(reply: Reply) {
	const previousConfig = client.getConfig();
	client.setConfig({
		baseUrl: window.location.origin,
		fetch: async () => {
			if (reply === "pending") return new Promise<Response>(() => {});
			if (reply === "fail") {
				return Response.json(
					{ errors: [{ code: "UNEXPECTED_ERROR", message: "Unavailable" }] },
					{ status: 500 },
				);
			}
			return Response.json({
				items: reply,
				total: reply.length,
				count: reply.length,
			});
		},
	});
	return () =>
		client.setConfig({
			...previousConfig,
			fetch: previousConfig.fetch ?? globalThis.fetch.bind(globalThis),
		});
}

function Harness({ disabled }: { disabled?: boolean }) {
	const [value, setValue] = useState("organization");
	return (
		<div style={{ width: 320 }}>
			<label htmlFor="resource-type">Resource type</label>
			<ResourceTypeSelect
				id="resource-type"
				productId={PRODUCT_ID}
				value={value}
				onChange={setValue}
				disabled={disabled}
			/>
			<output aria-label="Selected type">{value}</output>
		</div>
	);
}

const meta = {
	title: "Product/ResourceTypeSelect",
	component: Harness,
	beforeEach: () => mockResourceTypes([ORGANIZATION]),
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof Harness>;

export default meta;

type Story = StoryObj<typeof meta>;

async function trigger(canvasElement: HTMLElement) {
	const combobox = within(canvasElement).getByRole("combobox", {
		name: "Resource type",
	});
	await waitFor(() => expect(combobox).toBeEnabled());
	return combobox;
}

/** A product that declared no types offers only the built-in root. */
export const OrganizationOnly: Story = {
	play: async ({ canvasElement }) => {
		const combobox = await trigger(canvasElement);
		await expect(combobox).toHaveTextContent("Organization");
		await userEvent.click(combobox);
		const options = await screen.findAllByRole("option");
		await expect(options.map((option) => option.textContent)).toEqual([
			"Organization",
		]);
	},
};

/** Choosing a type reports its slug, not its display name. */
export const ChoosesBySlug: Story = {
	beforeEach: () =>
		mockResourceTypes([
			ORGANIZATION,
			resourceType("project", "Project"),
			resourceType("folder", "Folder", ["project"]),
		]),
	play: async ({ canvasElement }) => {
		await userEvent.click(await trigger(canvasElement));
		await userEvent.click(
			await screen.findByRole("option", { name: "Folder" }),
		);
		await expect(
			within(canvasElement).getByRole("status", { name: "Selected type" }),
		).toHaveTextContent("folder");
	},
};

/** The largest tree a product may declare: fifty types and the root. */
export const FiftyOneTypes: Story = {
	beforeEach: () =>
		mockResourceTypes([
			ORGANIZATION,
			...Array.from({ length: 50 }, (_, index) =>
				resourceType(`type_${index}`, `Resource type number ${index}`),
			),
		]),
	play: async ({ canvasElement }) => {
		await userEvent.click(await trigger(canvasElement));
		await expect(await screen.findAllByRole("option")).toHaveLength(51);
	},
};

/** A long unbroken name stays inside the trigger. */
export const LongName: Story = {
	beforeEach: () =>
		mockResourceTypes([
			ORGANIZATION,
			resourceType(
				"evidence",
				"RegulatoryComplianceEvidenceBundleWithAttachmentsAndSignatures",
			),
		]),
	play: async ({ canvasElement }) => {
		await userEvent.click(await trigger(canvasElement));
		await userEvent.click(
			await screen.findByRole("option", {
				name: "RegulatoryComplianceEvidenceBundleWithAttachmentsAndSignatures",
			}),
		);
		const combobox = await trigger(canvasElement);
		await expect(combobox.scrollWidth).toBeLessThanOrEqual(
			combobox.clientWidth + 1,
		);
	},
};

/** While the types load, the field can't be changed. */
export const Loading: Story = {
	beforeEach: () => mockResourceTypes("pending"),
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByRole("combobox", { name: "Resource type" }),
		).toBeDisabled();
	},
};

/** When the types can't load, Organization stays selectable. */
export const LoadFails: Story = {
	beforeEach: () => mockResourceTypes("fail"),
	play: async ({ canvasElement }) => {
		await userEvent.click(await trigger(canvasElement));
		await expect(
			await screen.findByRole("option", { name: "Organization" }),
		).toBeInTheDocument();
	},
};

/** A role's scope never changes, so the edit form shows it read-only. */
export const Disabled: Story = {
	args: { disabled: true },
	play: async ({ canvasElement }) => {
		const combobox = within(canvasElement).getByRole("combobox", {
			name: "Resource type",
		});
		await waitFor(() => expect(combobox).toHaveTextContent("Organization"));
		await expect(combobox).toBeDisabled();
	},
};
