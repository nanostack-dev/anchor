import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	BadgeCheck,
	BookOpen,
	Building2,
	Home,
	Key,
	LayoutDashboard,
	LayoutTemplate,
	ListChecks,
	Lock,
	type LucideIcon,
	Mail,
	PanelsTopLeft,
	Plug,
	ScrollText,
	Send,
	Shield,
	UserPlus,
	Users,
	Users2,
	Webhook,
} from "lucide-react";

export interface SubMenuItem {
	title: string;
	path: string;
	icon?: LucideIcon;
	external?: boolean;
}

export interface MenuItem {
	title: string;
	path?: string;
	icon?: LucideIcon;
	submenu?: boolean;
	subItems?: SubMenuItem[];
	external?: boolean;
}

export interface SidebarGroup {
	id: string;
	title?: string;
	items: MenuItem[];
	type?: "platform" | "product";
}

const baseURL = import.meta.env.VITE_API_BASE_URL || "";

export const sidebarConfig: SidebarGroup[] = [
	{
		id: "platform",
		title: "Platform",
		type: "platform",
		items: [
			{
				title: "Dashboard",
				path: ROUTE_PATHS.INDEX,
				icon: Home,
			},
			{
				title: "Admin Users",
				icon: Users,
				submenu: true,
				subItems: [
					{
						title: "Users",
						path: ROUTE_PATHS.PLATFORM_USERS,
						icon: Users2,
					},
					{
						title: "Invitations",
						path: ROUTE_PATHS.PLATFORM_INVITATIONS,
						icon: UserPlus,
					},
				],
			},
			{
				title: "Products",
				path: ROUTE_PATHS.PRODUCTS,
				icon: LayoutDashboard,
			},
			{
				title: "Integrations",
				path: ROUTE_PATHS.PLATFORM_INTEGRATIONS,
				icon: Plug,
			},
		],
	},
	{
		id: "product_management",
		title: "Product Management",
		type: "product",
		items: [
			{
				title: "Product API Keys",
				path: ROUTE_PATHS.PRODUCT_API_KEYS,
				icon: Key,
			},
			{
				title: "Anchor Permissions",
				path: ROUTE_PATHS.PRODUCT_PERMISSIONS,
				icon: ListChecks,
			},
		],
	},
	{
		id: "product_resources",
		title: "Product Resources",
		type: "product",
		items: [
			{
				title: "Roles & Permissions",
				icon: Shield,
				submenu: true,
				subItems: [
					{
						title: "Roles",
						path: ROUTE_PATHS.PRODUCT_ROLES,
						icon: Lock,
					},
					{
						title: "Permissions",
						path: ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS,
						icon: ListChecks,
					},
				],
			},
			{
				title: "Organizations",
				icon: Building2,
				submenu: true,
				subItems: [
					{
						title: "Overview",
						path: ROUTE_PATHS.ORGANIZATIONS,
						icon: Building2,
					},
					{
						title: "Members",
						path: ROUTE_PATHS.ORGANIZATION_MEMBERSHIPS,
						icon: Users2,
					},
					{
						title: "API Keys",
						path: ROUTE_PATHS.ORGANIZATIONS_APIS_KEYS,
						icon: Key,
					},
					{
						title: "Workspaces",
						path: ROUTE_PATHS.WORKSPACES,
						icon: PanelsTopLeft,
					},
					{
						title: "License",
						path: ROUTE_PATHS.ORGANIZATION_LICENSE,
						icon: BadgeCheck,
					},
				],
			},
			{
				title: "Users",
				path: ROUTE_PATHS.PRODUCT_USERS,
				icon: Users,
			},
			{
				title: "Events",
				path: ROUTE_PATHS.PRODUCT_EVENTS,
				icon: Webhook,
			},
		],
	},
	{
		id: "licensing",
		title: "Licensing",
		type: "product",
		items: [
			{
				title: "Schema",
				path: ROUTE_PATHS.PRODUCT_LICENSE_SCHEMA,
				icon: ScrollText,
			},
			{
				title: "Templates",
				path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATES,
				icon: LayoutTemplate,
			},
		],
	},
	{
		id: "email",
		title: "Email",
		type: "product",
		items: [
			{
				title: "Templates",
				path: ROUTE_PATHS.EMAIL_TEMPLATES,
				icon: Mail,
			},
			{
				title: "Send History",
				path: ROUTE_PATHS.EMAIL_SENDS,
				icon: Send,
			},
		],
	},
	{
		id: "developers",
		title: "Developers",
		items: [
			{
				title: "API Documentation",
				path: `${baseURL}/docs`,
				icon: BookOpen,
				external: true,
			},
		],
	},
];
