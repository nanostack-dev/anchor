import type {
	CreatedOrganizationApiKeyResponse,
	OrganizationInvitationResponse,
	ProductWorkspaceResponse,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { selectProduct } from "../../support/ui";
import {
	bulkDelete,
	facet,
	organization,
	permission,
	role,
	row,
	selectOrganization,
	user,
} from "./helpers";

// Covers: PRODUCT_USERS
test("access.product-users: scoped users support search, status filtering and pagination without edit controls", async ({
	page,
	world,
}) => {
	const users = await Promise.all(
		Array.from({ length: 12 }, (_, index) =>
			user(world, `user-${index}`, index === 0 ? "suspended" : "active"),
		),
	);
	await selectProduct(page, world.product);
	await page.goto("/products/users");
	await expect(
		page.getByRole("heading", { name: "Users", exact: true }),
	).toBeVisible();
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(
		page.getByRole("button", { name: "Previous", exact: true }),
	).toBeEnabled();
	await page.getByRole("textbox").fill(users[0].email);
	await expect(row(page, users[0].email)).toBeVisible();
	await expect(page.getByText("1 total", { exact: true })).toBeVisible();
	await facet(page, "Status", "Suspended");
	await expect(row(page, users[0].email)).toBeVisible();
	await page.getByRole("textbox").fill(world.name("missing-user"));
	await expect(
		page.getByText("No users match your search and filters", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Clear search and filters", exact: true })
		.click();
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	await expect(
		page.getByRole("button", { name: /Create user|Delete user|Edit user/i }),
	).toHaveCount(0);
});

// Covers: ORGANIZATIONS
test("access.organizations: overview search, facets, sorting and pagination show only this product", async ({
	page,
	world,
}) => {
	const organizations = await Promise.all(
		Array.from({ length: 12 }, (_, index) =>
			organization(world, `Organization ${String(index).padStart(2, "0")}`),
		),
	);
	await selectProduct(page, world.product);
	await page.goto("/organizations");
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	await page
		.getByRole("columnheader", { name: "Name", exact: true })
		.getByRole("button")
		.click();
	await expect(row(page, organizations[0].name)).toBeVisible();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(row(page, organizations[11].name)).toBeVisible();
	await page.getByRole("textbox").fill(organizations[0].name);
	await expect(row(page, organizations[0].name)).toBeVisible();
	await expect(page.getByText("1 total", { exact: true })).toBeVisible();
	await facet(page, "Name", organizations[0].name);
	await expect(row(page, organizations[0].name)).toBeVisible();
	await expect(page.getByRole("checkbox")).toHaveCount(0);
	await expect(
		page.getByRole("button", {
			name: /Create organization|Delete organization|Edit organization/i,
		}),
	).toHaveCount(0);
});

// Covers: ORGANIZATION_MEMBERSHIPS
test("access.organization-members: organization selection scopes pagination and search by email or name", async ({
	page,
	world,
}) => {
	const organizations = await Promise.all([
		organization(world, "Organization A"),
		organization(world, "Organization B"),
	]);
	const memberRole = await role(world);
	const users = await Promise.all([
		user(world, "first-member"),
		user(world, "other-member"),
		...Array.from({ length: 10 }, (_, index) => user(world, `member-${index}`)),
	]);
	const tenantAPI = await world.productAPI();
	await Promise.all(
		users.map((member) =>
			tenantAPI.post(
				`${world.productPath}/organizations/${organizations[0].id}/members`,
				{ product_user_id: member.id, role_id: memberRole.id },
			),
		),
	);
	await selectProduct(page, world.product);
	await page.goto("/organization-memberships");
	await expect(
		page.getByText("No Organization Selected", { exact: true }),
	).toBeVisible();
	await selectOrganization(page, organizations[0].name);
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(
		page.getByRole("row").filter({ has: page.getByRole("cell") }),
	).toHaveCount(2);
	await page.getByRole("button", { name: "Previous", exact: true }).click();
	await expect(
		page.getByRole("row").filter({ has: page.getByRole("cell") }),
	).toHaveCount(10);
	await page.getByRole("textbox").fill(users[0].email);
	await expect(row(page, users[0].email)).toContainText(memberRole.name);
	await expect(page.getByText("1 total", { exact: true })).toBeVisible();
	await expect(row(page, users[1].email)).toHaveCount(0);
	await page.getByRole("textbox").fill(users[0].name);
	await expect(row(page, users[0].email)).toBeVisible();
	await expect(page.getByText("1 total", { exact: true })).toBeVisible();
	await selectOrganization(page, organizations[1].name);
	await expect(
		page.getByText("No members match your search", { exact: true }),
	).toBeVisible();
	await page.getByRole("button", { name: "Clear search", exact: true }).click();
	await expect(page.getByText("No members yet", { exact: true })).toBeVisible();
});

// Covers: ORGANIZATION_MEMBERSHIPS
test("access.organization-invitations: status filter, cancel, single and bulk deletion respect organization scope", async ({
	page,
	world,
}) => {
	const organizations = await Promise.all([
		organization(world, "Invitations A"),
		organization(world, "Invitations B"),
	]);
	const inviteRole = await role(world);
	const invitationPath = `${world.productPath}/organizations/${organizations[0].id}/invitations`;
	const tenantAPI = await world.productAPI();
	const invitations = await Promise.all(
		Array.from({ length: 3 }, (_, index) =>
			tenantAPI.post<OrganizationInvitationResponse>(invitationPath, {
				email: `${world.name(`invite-${index}`)}@example.test`,
				role_id: inviteRole.id,
			}),
		),
	);
	const unrelated = await tenantAPI.post<OrganizationInvitationResponse>(
		`${world.productPath}/organizations/${organizations[1].id}/invitations`,
		{
			email: `${world.name("unrelated-invite")}@example.test`,
			role_id: inviteRole.id,
		},
	);
	await selectProduct(page, world.product);
	await page.goto("/organization-memberships");
	await selectOrganization(page, organizations[0].name);
	await page.getByRole("tab", { name: "Invitations", exact: true }).click();
	await expect(row(page, invitations[0].email)).toContainText(inviteRole.name);
	await expect(row(page, unrelated.email)).toHaveCount(0);
	await facet(page, "Status", "Pending");
	await expect(row(page, invitations[0].email)).toBeVisible();
	await row(page, invitations[0].email)
		.getByRole("button", {
			name: `Delete invitation for ${invitations[0].email}`,
			exact: true,
		})
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Cancel", exact: true })
		.click();
	await expect(row(page, invitations[0].email)).toBeVisible();
	await row(page, invitations[0].email)
		.getByRole("button", {
			name: `Delete invitation for ${invitations[0].email}`,
			exact: true,
		})
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete", exact: true })
		.click();
	await expect(row(page, invitations[0].email)).toHaveCount(0);
	await bulkDelete(page, [invitations[1].email]);
	await expect(row(page, invitations[2].email)).toBeVisible();
	for (const deleted of invitations.slice(0, 2))
		expect(
			(await world.api.context.get(`${invitationPath}/${deleted.id}`)).status(),
		).toBe(404);
	await world.api.get(`${invitationPath}/${invitations[2].id}`);
	await world.api.get(
		`${world.productPath}/organizations/${organizations[1].id}/invitations/${unrelated.id}`,
	);
});

// Covers: ORGANIZATIONS_APIS_KEYS
test("access.organization-keys: organization selection, search and status show obfuscated keys and grants read-only", async ({
	page,
	world,
}) => {
	const organizations = await Promise.all([
		organization(world, "Keys A"),
		organization(world, "Keys B"),
	]);
	const grant = await permission(world);
	const keyPath = `${world.productPath}/organizations/${organizations[0].id}/api-keys`;
	const tenantAPI = await world.productAPI();
	const keys = await Promise.all(
		["API first", "API second"].map((prefix) =>
			tenantAPI.post<CreatedOrganizationApiKeyResponse>(keyPath, {
				name: world.name(prefix),
				description: "Local integration key",
				permissions: [grant.name],
			}),
		),
	);
	await selectProduct(page, world.product);
	await page.goto("/organizations/api-keys");
	await expect(
		page.getByText("No Organization Selected", { exact: true }),
	).toBeVisible();
	await selectOrganization(page, organizations[0].name);
	await expect(row(page, keys[0].name)).toContainText(keys[0].obfuscated_value);
	await expect(row(page, keys[0].name)).toContainText(grant.name);
	await expect(page.getByText(keys[0].value, { exact: true })).toHaveCount(0);
	await page.getByRole("textbox").fill(keys[0].name);
	await expect(row(page, keys[1].name)).toHaveCount(0);
	await facet(page, "Status", "Active");
	await expect(row(page, keys[0].name)).toBeVisible();
	await expect(
		page.getByRole("button", { name: /^(Create|Delete|Edit)( |$)/ }),
	).toHaveCount(0);
	await selectOrganization(page, organizations[1].name);
	await expect(
		page.getByText("No organization API keys match your search and filters", {
			exact: true,
		}),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Clear search and filters", exact: true })
		.click();
	await expect(
		page.getByText("No organization API keys yet", { exact: true }),
	).toBeVisible();
});

// Covers: WORKSPACES, WORKSPACE_MEMBERSHIPS
test("access.workspaces: organization selection scopes read-only search and the hidden membership placeholder", async ({
	page,
	world,
}) => {
	const organizations = await Promise.all([
		organization(world, "Workspaces A"),
		organization(world, "Workspaces B"),
	]);
	const workspacePath = `${world.productPath}/organizations/${organizations[0].id}/workspaces`;
	const tenantAPI = await world.productAPI();
	const workspaces = await Promise.all(
		["Workspace first", "Workspace second"].map((prefix) =>
			tenantAPI.post<ProductWorkspaceResponse>(workspacePath, {
				name: world.name(prefix),
				description: "Workspace description",
			}),
		),
	);
	await selectProduct(page, world.product);
	await page.goto("/workspaces");
	await expect(
		page.getByText("No Organization Selected", { exact: true }),
	).toBeVisible();
	await selectOrganization(page, organizations[0].name);
	await expect(row(page, workspaces[0].name)).toContainText(
		"Workspace description",
	);
	await page.getByRole("textbox").fill(workspaces[0].name);
	await expect(row(page, workspaces[1].name)).toHaveCount(0);
	await facet(page, "Name", workspaces[0].name);
	await expect(row(page, workspaces[0].name)).toBeVisible();
	await expect(page.getByRole("checkbox")).toHaveCount(0);
	await selectOrganization(page, organizations[1].name);
	await expect(
		page.getByText("No workspaces match your search and filters", {
			exact: true,
		}),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Clear search and filters", exact: true })
		.click();
	await expect(
		page.getByText("No workspaces yet", { exact: true }),
	).toBeVisible();
	await page.goto("/workspace-memberships");
	await expect(
		page.getByRole("heading", { name: "Workspace Memberships", exact: true }),
	).toBeVisible();
});
