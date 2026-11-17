import { getCurrentUserOptions } from "@/client/@tanstack/react-query.gen";
import { AnchorLogo } from "@/components/brand/AnchorLogo";
import { ProductTopBar } from "@/components/layout/ProductTopBar";
import {
	AppShell,
	AppShellBrand,
	AppShellContent,
	AppShellHeader,
	AppShellInset,
	AppShellSidebar,
} from "@/components/layout/app-shell";
import { AppSidebar } from "@/components/sidebar/app-sidebar";
import { ProductProvider } from "@/context/product/ProductContext";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import {
	SidebarContent,
	SidebarGroup,
	SidebarHeader,
	SidebarMenu,
	SidebarMenuItem,
	SidebarMenuSkeleton,
} from "@nanostackorg/design-system/components/sidebar";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { useQuery } from "@tanstack/react-query";
import { AlertCircleIcon } from "lucide-react";
import type { ReactNode } from "react";

interface AuthenticatedLayoutProps {
	children: ReactNode;
}

// The loading state draws the same glass shell as the loaded page, so the
// layout does not jump when the current user arrives.
function ShellLoadingState() {
	return (
		<AppShell>
			<AppShellSidebar>
				<SidebarHeader>
					<AppShellBrand
						name="Anchor"
						description="Organization-as-a-Service"
						href={ROUTE_PATHS.PRODUCT_PERMISSIONS}
						mark={<AnchorLogo size="lg" />}
					/>
				</SidebarHeader>
				<SidebarContent>
					<SidebarGroup>
						<SidebarMenu>
							{Array.from({ length: 8 }).map((_, index) => (
								<SidebarMenuItem
									// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
									key={index}
								>
									<SidebarMenuSkeleton showIcon />
								</SidebarMenuItem>
							))}
						</SidebarMenu>
					</SidebarGroup>
				</SidebarContent>
			</AppShellSidebar>
			<AppShellInset>
				<AppShellHeader>
					<div className="flex h-10 w-full items-center gap-3 px-2">
						<Skeleton height="lg" width="1/4" />
					</div>
				</AppShellHeader>
				<AppShellContent>
					<div className="flex flex-col gap-4" aria-busy="true">
						<Skeleton height="lg" width="1/2" />
						<Skeleton height="md" width="3/4" />
						<Skeleton height="xxl" />
					</div>
				</AppShellContent>
			</AppShellInset>
		</AppShell>
	);
}

export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
	const { data: user, isLoading } = useQuery({
		...getCurrentUserOptions(),
	});

	if (isLoading) {
		return <ShellLoadingState />;
	}
	if (!user) {
		return (
			<div className="flex min-h-svh items-center justify-center p-6">
				<div className="w-full max-w-md">
					<Alert tone="critical">
						<AlertCircleIcon />
						<AlertTitle>Unable to load your account</AlertTitle>
						<AlertDescription>
							We couldn't load your user data. Refresh the page or sign in
							again.
						</AlertDescription>
					</Alert>
				</div>
			</div>
		);
	}

	const sidebarUser = {
		name: user.email,
		email: user.email,
		avatar: "/avatars/placeholder.jpg",
	};

	const placeholderTeams = [
		{
			name: "Default Tenant",
			logo: () => <div className="size-4 rounded-sm bg-muted" />,
			plan: "Active Plan",
		},
	];

	return (
		<ProductProvider>
			<AppShell>
				<AppSidebar user={sidebarUser} teams={placeholderTeams} />
				<AppShellInset>
					<AppShellHeader>
						<ProductTopBar />
					</AppShellHeader>
					<AppShellContent>{children}</AppShellContent>
				</AppShellInset>
			</AppShell>
		</ProductProvider>
	);
}
