import { getCurrentUserOptions } from "@/client/@tanstack/react-query.gen";
import { ProductTopBar } from "@/components/layout/ProductTopBar";
import {
	AppShell,
	AppShellContent,
	AppShellInset,
	AppShellSidebarTrigger,
	AppShellTopbar,
	AppShellTopbarContent,
} from "@/components/layout/app-shell";
import { AppSidebar } from "@/components/sidebar/app-sidebar";
import { ProductProvider } from "@/context/product/ProductContext";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { useQuery } from "@tanstack/react-query";
import { AlertCircleIcon } from "lucide-react";
import type { ReactNode } from "react";

interface AuthenticatedLayoutProps {
	children: ReactNode;
}

function ShellLoadingState() {
	return (
		<div className="flex min-h-svh w-full" aria-busy="true">
			<div className="hidden w-61 shrink-0 flex-col gap-4 border-r bg-sidebar p-4 md:flex">
				<Skeleton height="lg" />
				<div className="mt-4 flex flex-col gap-2">
					{Array.from({ length: 8 }).map((_, index) => (
						<Skeleton
							// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
							key={index}
							height="lg"
						/>
					))}
				</div>
			</div>
			<div className="flex min-w-0 flex-1 flex-col">
				<div className="flex h-16 items-center gap-3 border-b px-4">
					<Skeleton height="sm" />
					<Skeleton height="lg" />
				</div>
				<div className="flex flex-1 flex-col gap-4 p-6">
					<Skeleton height="lg" />
					<Skeleton height="md" />
					<Skeleton height="xxl" />
				</div>
			</div>
		</div>
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
					<AppShellTopbar>
						<AppShellSidebarTrigger />
						<AppShellTopbarContent>
							<ProductTopBar />
						</AppShellTopbarContent>
					</AppShellTopbar>
					<AppShellContent>{children}</AppShellContent>
				</AppShellInset>
			</AppShell>
		</ProductProvider>
	);
}
