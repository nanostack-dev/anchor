import { AppShellBrand, AppShellSidebar } from "@/components/layout/app-shell";
import { NavUser } from "@/components/sidebar/nav-user";
import { useProduct } from "@/hooks/useProduct";
import { sidebarConfig } from "@/lib/sidebar-config";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
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
} from "@nanostackorg/design-system/components/sidebar";
import { useLocation } from "@tanstack/react-router";
import type { LucideIcon } from "lucide-react";
import type { ElementType } from "react";

interface AppSidebarProps {
	user: { name: string; email: string; avatar: string };
	teams: { name: string; logo: ElementType | LucideIcon; plan: string }[];
}

export function AppSidebar({ user }: AppSidebarProps) {
	const { currentProduct } = useProduct();
	const location = useLocation();
	const isActivePath = (path?: string) => !!path && location.pathname === path;
	const renderIcon = (Icon?: LucideIcon) =>
		Icon ? <Icon aria-hidden /> : null;
	return (
		<AppShellSidebar material="frosted">
			<SidebarHeader>
				<AppShellBrand
					name="Anchor"
					description="Organization-as-a-Service"
					href={ROUTE_PATHS.PRODUCT_PERMISSIONS}
					logo={
						<img
							src="/logo.svg"
							alt=""
							className="size-6 brightness-0 invert"
						/>
					}
				/>
			</SidebarHeader>
			<SidebarContent>
				{sidebarConfig.map((group) => {
					const disabled = group.type === "product" && !currentProduct;
					return (
						<SidebarGroup key={group.id}>
							{group.title && (
								<SidebarGroupLabel>{group.title}</SidebarGroupLabel>
							)}
							<SidebarMenu>
								{group.items.map((item) => (
									<SidebarMenuItem key={item.title}>
										<SidebarMenuButton
											href={
												item.path && !item.submenu && !disabled
													? item.path
													: undefined
											}
											disabled={disabled}
											isActive={
												!disabled &&
												(isActivePath(item.path) ||
													item.subItems?.some((child) =>
														isActivePath(child.path),
													))
											}
											aria-current={
												!disabled && isActivePath(item.path)
													? "page"
													: undefined
											}
											tooltip={item.title}
											{...(item.external
												? {
														render: (
															<a
																aria-label={item.title}
																href={disabled ? undefined : item.path}
																target="_blank"
																rel="noopener noreferrer"
															>
																{item.title}
															</a>
														),
													}
												: {})}
										>
											{renderIcon(item.icon)}
											<span>{item.title}</span>
										</SidebarMenuButton>
										{item.submenu && item.subItems && (
											<SidebarMenuSub>
												{item.subItems.map((child) => (
													<SidebarMenuSubItem key={child.title}>
														<SidebarMenuSubButton
															href={child.path}
															isActive={!disabled && isActivePath(child.path)}
															aria-current={
																!disabled && isActivePath(child.path)
																	? "page"
																	: undefined
															}
															aria-disabled={disabled || undefined}
															tabIndex={disabled ? -1 : undefined}
															onClick={
																disabled
																	? (event) => event.preventDefault()
																	: undefined
															}
															target={child.external ? "_blank" : undefined}
															rel={
																child.external
																	? "noopener noreferrer"
																	: undefined
															}
														>
															{renderIcon(child.icon)}
															<span>{child.title}</span>
														</SidebarMenuSubButton>
													</SidebarMenuSubItem>
												))}
											</SidebarMenuSub>
										)}
									</SidebarMenuItem>
								))}
							</SidebarMenu>
						</SidebarGroup>
					);
				})}
			</SidebarContent>
			<SidebarFooter>
				<NavUser user={user} />
			</SidebarFooter>
		</AppShellSidebar>
	);
}
