import { ProductTopBar } from "@/components/layout/ProductTopBar";
import { AppSidebar } from "@/components/sidebar/app-sidebar";
import { SidebarProvider } from "@/components/ui/sidebar";
import { ProductProvider } from "@/context/product/ProductContext";
import type { ReactNode } from "react";

interface AuthenticatedLayoutProps {
	children: ReactNode;
}

import { getCurrentUserOptions } from "@/client/@tanstack/react-query.gen";
import { useQuery } from "@tanstack/react-query";

export function AuthenticatedLayout({ children }: AuthenticatedLayoutProps) {
	const { data: user, isLoading } = useQuery({
		...getCurrentUserOptions(),
	});

	if (isLoading) {
		return <div>Loading...</div>;
	}
	if (!user) {
		return <div>Unable to load user data.</div>;
	}

	const sidebarUser = {
		name: user.email,
		email: user.email,
		avatar: "/avatars/placeholder.jpg",
	};

	const placeholderTeams = [
		{
			name: "Default Tenant",
			logo: () => <div className="w-4 h-4 bg-gray-300 rounded-sm" />,
			plan: "Active Plan",
		},
	];

	return (
		<ProductProvider>
			<div className="flex h-screen bg-background">
				<SidebarProvider>
					<AppSidebar user={sidebarUser} teams={placeholderTeams} />
					<div className="flex-1 flex min-h-0 flex-col">
						<ProductTopBar />
						<main className="flex-1 min-h-0 overflow-hidden">
							{isLoading ? <div>Loading...</div> : children}
						</main>
					</div>
				</SidebarProvider>
			</div>
		</ProductProvider>
	);
}
