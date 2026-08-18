import {
	LicenseMigrationOutcome,
	type OrganizationLicenseMigrationResponse,
} from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { LicenseMigrationOutcomes } from "./LicenseMigrationOutcomes";

const ORGANIZATION_NAMES = {
	org_acme: "Acme Corp",
	org_globex: "Globex",
	org_initech: "Initech",
	org_umbrella: "Umbrella",
};

const MIXED_RUN: OrganizationLicenseMigrationResponse = {
	template_id: "ltpl_pro",
	migrated_at: "2026-08-16T10:30:00Z",
	count: 4,
	changed: 2,
	unchanged: 1,
	failed: 1,
	results: [
		{
			organization_id: "org_acme",
			outcome: LicenseMigrationOutcome.CHANGED,
			previous_template_id: "ltpl_beta",
			changes: [],
			count: 3,
		},
		{
			organization_id: "org_globex",
			outcome: LicenseMigrationOutcome.UNCHANGED,
			previous_template_id: "ltpl_pro",
			changes: [],
			count: 0,
		},
		{
			// Held no license before this run — granted one, not moved. See
			// docs/adr/0015-migrate-grants-a-first-license.md.
			organization_id: "org_initech",
			outcome: LicenseMigrationOutcome.CHANGED,
			changes: [],
			count: 4,
		},
		{
			organization_id: "org_umbrella",
			outcome: LicenseMigrationOutcome.FAILED,
			changes: [],
			count: 0,
			error: {
				code: "ORGANIZATION_NOT_FOUND",
				message: "This product has no organization with that identifier",
			},
		},
	],
};

const meta = {
	title: "License/LicenseMigrationOutcomes",
	component: LicenseMigrationOutcomes,
	parameters: { layout: "padded" },
} satisfies Meta<typeof LicenseMigrationOutcomes>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WhatNeedsActingOnComesFirst: Story = {
	args: { migration: MIXED_RUN, organizationNames: ORGANIZATION_NAMES },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// A run keeps going past a failure, so the failures are the part an
		// operator has to act on and they lead.
		const headings = canvas.getAllByRole("heading", { level: 3 });
		await expect(headings[0]).toHaveTextContent("Failed");
		await expect(headings[1]).toHaveTextContent("Set");

		await expect(
			canvas.getByText("This product has no organization with that identifier"),
		).toBeVisible();
		await expect(
			canvas.getByText("Granted this tier — held no license before this run."),
		).toBeVisible();
	},
};

export const EveryOrganizationMoved: Story = {
	args: {
		organizationNames: ORGANIZATION_NAMES,
		migration: {
			...MIXED_RUN,
			count: 1,
			changed: 1,
			unchanged: 0,
			failed: 0,
			results: [MIXED_RUN.results[0]],
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("Acme Corp")).toBeVisible();
		await expect(canvas.getByText("3 license fields changed.")).toBeVisible();
		await expect(canvas.queryByRole("heading", { name: /Failed/ })).toBeNull();
	},
};
