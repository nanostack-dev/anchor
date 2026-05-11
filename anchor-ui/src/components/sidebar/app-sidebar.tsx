import { NavUser } from "@/components/sidebar/nav-user";
import { useProduct } from "@/hooks/useProduct";
import {
	type MenuItem as ConfigMenuItem,
	type SubMenuItem as ConfigSubMenuItem,
	type SidebarGroup as SidebarConfigGroup,
	sidebarConfig,
} from "@/lib/sidebar-config";
import { Link } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import type * as React from "react";

import {
	Sidebar,
	SidebarContent,
	SidebarFooter,
	SidebarGroup,
	SidebarGroupLabel,
	SidebarHeader,
	SidebarMenu,
	SidebarMenuButton,
	SidebarMenuItem,
	SidebarMenuSub,
	SidebarMenuSubButton,
	SidebarMenuSubItem,
	SidebarRail,
} from "@/components/ui/sidebar";

interface UserData {
	name: string;
	email: string;
	avatar: string;
}

interface TeamData {
	name: string;
	logo: React.ElementType | LucideIcon;
	plan: string;
}

interface AppSidebarProps extends React.ComponentProps<typeof Sidebar> {
	user: UserData;
	teams: TeamData[];
}

import { useSidebar } from "@/components/ui/sidebar";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useLocation } from "@tanstack/react-router";

export function AppSidebar({ user, teams, ...props }: AppSidebarProps) {
	const { currentProduct } = useProduct();
	const hasProductSelected = !!currentProduct;

	const renderIcon = (IconComponent?: LucideIcon) => {
		if (IconComponent) {
			return <IconComponent className="size-4 shrink-0" />;
		}
		return null;
	};

	const { state } = useSidebar();
	const collapsed = state === "collapsed";
	const location = useLocation();

	// Helper to check if a path is active
	const isActivePath = (path?: string): boolean =>
		!!path && location.pathname === path;

	const isGroupDisabled = (group: SidebarConfigGroup): boolean =>
		group.type === "product" && !hasProductSelected;

	const getMenuItemClasses = (group: SidebarConfigGroup): string => {
		if (isGroupDisabled(group)) {
			return "opacity-50";
		}
		return "";
	};

	return (
		<Sidebar collapsible="icon" {...props}>
			<SidebarHeader>
				<Link
					to={ROUTE_PATHS.PRODUCT_PERMISSIONS}
					className={`flex items-center gap-2 ${collapsed ? "justify-center" : ""}`}
				>
					<img
						src="/logo.svg"
						alt="App Logo"
						className={
							collapsed
								? "h-6 w-6 transition-all duration-200"
								: "h-8 w-auto transition-all duration-200"
						}
					/>
					{!collapsed && (
						<h4 className="scroll-m-20 text-xl font-semibold tracking-tight transition-all duration-200">
							Anchor
						</h4>
					)}
				</Link>
			</SidebarHeader>
			<SidebarContent>
				{sidebarConfig.map((group: SidebarConfigGroup) => (
					<SidebarGroup key={group.id} className={getMenuItemClasses(group)}>
						{group.title && (
							<SidebarGroupLabel
								className={
									isGroupDisabled(group) ? "text-muted-foreground" : ""
								}
							>
								{group.title}
							</SidebarGroupLabel>
						)}
						<SidebarMenu>
							{group.items.map((item: ConfigMenuItem, itemIndex: number) => (
								<SidebarMenuItem key={`${group.id}-item-${itemIndex}`}>
									{item.path && !item.submenu ? (
										<SidebarMenuButton
											asChild
											isActive={
												!isGroupDisabled(group) && isActivePath(item.path)
											}
										>
											{item.external ? (
												<a
													href={item.path}
													target="_blank"
													rel="noopener noreferrer"
													className={
														isGroupDisabled(group) ? "pointer-events-none" : ""
													}
												>
													{renderIcon(item.icon)}
													<span>{item.title}</span>
												</a>
											) : (
												<Link
													to={item.path}
													className={
														isGroupDisabled(group) ? "pointer-events-none" : ""
													}
												>
													{renderIcon(item.icon)}
													<span>{item.title}</span>
												</Link>
											)}
										</SidebarMenuButton>
									) : (
										<SidebarMenuButton
											isActive={
												!isGroupDisabled(group) &&
												item.subItems?.some((subItem) =>
													isActivePath(subItem.path),
												)
											}
										>
											{renderIcon(item.icon)}
											<span>{item.title}</span>
										</SidebarMenuButton>
									)}
									{item.submenu && item.subItems && (
										<SidebarMenuSub>
											{item.subItems.map(
												(subItem: ConfigSubMenuItem, subItemIndex: number) => (
													<SidebarMenuSubItem
														key={`${group.id}-item-${itemIndex}-sub-${subItemIndex}`}
													>
														<SidebarMenuSubButton
															asChild
															isActive={
																!isGroupDisabled(group) &&
																isActivePath(subItem.path)
															}
														>
															{subItem.external ? (
																<a
																	href={subItem.path}
																	target="_blank"
																	rel="noopener noreferrer"
																	className={
																		isGroupDisabled(group)
																			? "pointer-events-none"
																			: ""
																	}
																>
																	{renderIcon(subItem.icon)}
																	<span>{subItem.title}</span>
																</a>
															) : (
																<Link
																	to={subItem.path}
																	className={
																		isGroupDisabled(group)
																			? "pointer-events-none"
																			: ""
																	}
																>
																	{renderIcon(subItem.icon)}
																	<span>{subItem.title}</span>
																</Link>
															)}
														</SidebarMenuSubButton>
													</SidebarMenuSubItem>
												),
											)}
										</SidebarMenuSub>
									)}
								</SidebarMenuItem>
							))}
						</SidebarMenu>
					</SidebarGroup>
				))}
			</SidebarContent>
			<SidebarFooter>
				<NavUser user={user} />
			</SidebarFooter>
			<SidebarRail />
		</Sidebar>
	);
}
