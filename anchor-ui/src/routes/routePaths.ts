// Route path constants to avoid circular dependencies
export const ROUTE_PATHS = {
	// Authentication routes
	INIT: "/init",
	LOGIN: "/login",
	REGISTER: "/register",

	// Platform routes
	INDEX: "/",
	PLATFORM_USERS: "/platform/users",
	PLATFORM_INVITATIONS: "/platform/users/invitations",
	PLATFORM_INTEGRATIONS: "/platform/integrations",
	INTEGRATION_CLERK: "/platform/integration-clerk",
	PRODUCT_INTEGRATIONS: "/platform/$productId/integrations",
	PRODUCT_INTEGRATION_CLERK: "/platform/$productId/integration-clerk",
	PRODUCT_INTEGRATION_SMTP: "/platform/$productId/integration-smtp",
	PRODUCTS: "/products",

	// Product routes
	PRODUCT_API_KEYS: "/products/product-api-keys",
	PRODUCT_API_KEY_NEW: "/products/product-api-keys/new",
	PRODUCT_API_KEY_EDIT: "/products/product-api-keys/$apiKeyId/edit",
	PRODUCT_USERS: "/products/users",
	PRODUCT_ROLES: "/products/resources/roles",
	PRODUCT_PERMISSIONS: "/products/permissions",
	PRODUCT_RESOURCES_PERMISSIONS: "/products/resources/permissions",
	PRODUCT_EDIT: "/products/$productId/edit",
	PRODUCT_EVENTS: "/products/events",

	// Licensing routes
	PRODUCT_LICENSE_SCHEMA: "/products/licensing/schema",
	PRODUCT_LICENSE_TEMPLATES: "/products/licensing/templates",
	ORGANIZATION_LICENSE: "/organizations/license",
	ORGANIZATION_LICENSE_DETAIL: "/organizations/license/$organizationId",

	// Organization routes
	ORGANIZATIONS: "/organizations",
	ORGANIZATIONS_APIS_KEYS: "/organizations/api-keys",
	ORGANIZATION_MEMBERSHIPS: "/organization-memberships",
	WORKSPACES: "/workspaces",
	WORKSPACE_MEMBERSHIPS: "/workspace-memberships",

	// Email routes
	EMAIL_TEMPLATES: "/products/email/templates",
	EMAIL_TEMPLATE_BUILDER: "/products/email/templates/$templateId",
	EMAIL_SENDS: "/products/email/sends",

	// Settings routes
	SETTINGS_USER: "/settings/user",
	SETTINGS_APP: "/settings/app",
} as const;
