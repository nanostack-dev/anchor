import { indexRoute } from "@/routes";
import { type RouteContext, rootRoute } from "@/routes/__root";
import { organizationApiKeysRoute } from "@/routes/organizations/organization-api-keys";
import { organizationMembershipsRoute } from "@/routes/organizations/organization-memberships";
import { organizationsRoute } from "@/routes/organizations/organizations";
import { workspaceMembershipsRoute } from "@/routes/organizations/workspace-memberships";
import { workspacesRoute } from "@/routes/organizations/workspaces";
import { productIntegrationClerkRoute } from "@/routes/platform/$productId.integration-clerk";
import { productIntegrationSmtpRoute } from "@/routes/platform/$productId.integration-smtp";
import { productIntegrationsRoute } from "@/routes/platform/$productId.integrations";
import { initRoute } from "@/routes/platform/init";
import { integrationClerkRoute } from "@/routes/platform/integration-clerk";
import { platformIntegrationsRoute } from "@/routes/platform/integrations";
import { platformInvitationsRoute } from "@/routes/platform/invitations";
import { loginRoute } from "@/routes/platform/login";
import { platformUsersRoute } from "@/routes/platform/platform-users";
import { registerRoute } from "@/routes/platform/register";
import { productEditRoute } from "@/routes/products/$productId.edit";
import { emailSendsRoute } from "@/routes/products/email-sends";
import { emailTemplateBuilderRoute } from "@/routes/products/email-template-builder";
import { emailTemplatesRoute } from "@/routes/products/email-templates";
import { productPermissionsRoute } from "@/routes/products/permissions";
import { productApiKeysRoute } from "@/routes/products/product-api-keys";
import { productResourcePermissionsRoute } from "@/routes/products/product-resource-permissions";
import { productRolesRoute } from "@/routes/products/product-roles-route";
import { productUsersRoute } from "@/routes/products/product-users";
import { productsRoute } from "@/routes/products/products";
import { settingsAppRoute } from "@/routes/settings/app";
import { settingsUserRoute } from "@/routes/settings/user";
import { createRouter } from "@tanstack/react-router";

const routeTree = rootRoute.addChildren([
	indexRoute,
	loginRoute,
	registerRoute,
	initRoute,
	productsRoute,
	productEditRoute,
	productApiKeysRoute,
	productUsersRoute,
	productRolesRoute,
	productResourcePermissionsRoute,
	productPermissionsRoute,
	organizationsRoute,
	organizationApiKeysRoute,
	organizationMembershipsRoute,
	workspacesRoute,
	workspaceMembershipsRoute,
	settingsUserRoute,
	settingsAppRoute,
	platformUsersRoute,
	platformInvitationsRoute,
	platformIntegrationsRoute,
	productIntegrationsRoute,
	integrationClerkRoute,
	productIntegrationClerkRoute,
	productIntegrationSmtpRoute,
	emailTemplatesRoute,
	emailTemplateBuilderRoute,
	emailSendsRoute,
]);

const routerContext = {
	queryClient: undefined as unknown as RouteContext["queryClient"],
	auth: undefined as unknown as RouteContext["auth"],
} satisfies RouteContext;

export const router = createRouter({
	routeTree,
	context: routerContext,
});
