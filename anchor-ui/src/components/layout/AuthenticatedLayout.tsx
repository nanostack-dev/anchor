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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Skeleton } from "@/components/ui/skeleton";
import { ProductProvider } from "@/context/product/ProductContext";
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
				<Skeleton className="h-8 w-32" />
				<div className="mt-4 flex flex-col gap-2">
					{Array.from({ length: 8 }).map((_, index) => (
						<Skeleton
							// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholders
							key={index}
							className="h-8 w-full"
						/>
					))}
				</div>
			</div>
			<div className="flex min-w-0 flex-1 flex-col">
				<div className="flex h-16 items-center gap-3 border-b px-4">
					<Skeleton className="size-8" />
					<Skeleton className="h-8 w-48" />
				</div>
				<div className="flex flex-1 flex-col gap-4 p-6">
					<Skeleton className="h-9 w-64" />
					<Skeleton className="h-5 w-96" />
					<Skeleton className="mt-4 h-64 w-full" />
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
				<Alert variant="destructive" className="max-w-md">
					<AlertCircleIcon />
					<AlertTitle>Unable to load your account</AlertTitle>
					<AlertDescription>
						We couldn't load your user data. Refresh the page or sign in again.
					</AlertDescription>
				</Alert>
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
