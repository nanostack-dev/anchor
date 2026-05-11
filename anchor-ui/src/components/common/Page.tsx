import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Link, useLocation } from "@tanstack/react-router";
import { Home as HomeIcon } from "lucide-react";
import * as React from "react";
import { useMemo } from "react";
import { PageInfo, type PageInfoProps } from "./PageInfo";

function getBreadcrumbs(pathname: string) {
	const segments = pathname.split("/").filter(Boolean);
	const crumbs = [{ name: "Dashboard", path: "/" }];
	let currentPath = "";
	for (const segment of segments) {
		currentPath += `/${segment}`;
		crumbs.push({
			name: segment.replace(/-/g, " ").replace(/\b\w/g, (l) => l.toUpperCase()),
			path: currentPath,
		});
	}
	return crumbs;
}

type PageVariant = "full" | "wide" | "narrow" | "default";

type PageProps = {
	children: React.ReactNode;
	title?: string;
	description?: string;
	breadCrumbs?: boolean;
	variant?: PageVariant;
	pageInfo?: PageInfoProps;
};

export function Page({
	children,
	title,
	description,
	breadCrumbs = true,
	variant = "full",
	pageInfo,
}: PageProps) {
	const location = useLocation();
	const crumbs = useMemo(
		() => getBreadcrumbs(location.pathname),
		[location.pathname],
	);

	// Variant-based container styles
	let variantClass = "h-full min-h-0 w-full flex flex-col gap-6 p-6";
	if (variant === "full") {
		// No max-width or centering for full variant
	} else if (variant === "wide") {
		variantClass += " max-w-[1200px] mx-auto";
	} else if (variant === "narrow") {
		variantClass += " max-w-[600px] mx-auto";
	} else {
		variantClass += " max-w-[900px] mx-auto";
	}

	return (
		<div className={variantClass} data-testid="page-root">
			{title && (
				<div className="mb-2">
					<h1 className="text-2xl font-bold" data-testid="page-title">
						{title}
					</h1>
					{description && (
						<p
							className="text-muted-foreground text-base mt-1"
							data-testid="page-description"
						>
							{description}
						</p>
					)}
				</div>
			)}
			{breadCrumbs && (
				<nav aria-label="Breadcrumb" data-testid="breadcrumb-nav">
					<Breadcrumb>
						<BreadcrumbList>
							{crumbs.map((crumb, idx) => (
								<React.Fragment key={crumb.path}>
									<BreadcrumbItem>
										{idx === 0 ? (
											<Link
												to={crumb.path}
												className="inline-flex items-center gap-1 hover:text-primary transition-colors focus:underline"
												tabIndex={0}
												aria-label="Dashboard"
											>
												<HomeIcon size={16} className="mr-1" />
												<span
													className="truncate max-w-[120px]"
													title={crumb.name}
												>
													{crumb.name}
												</span>
											</Link>
										) : idx === crumbs.length - 1 ? (
											<BreadcrumbPage aria-current="page">
												<span
													className="truncate max-w-[120px]"
													title={crumb.name}
												>
													{crumb.name}
												</span>
											</BreadcrumbPage>
										) : (
											<Link
												to={crumb.path}
												className="hover:text-primary transition-colors focus:underline"
												tabIndex={0}
											>
												<span
													className="truncate max-w-[120px]"
													title={crumb.name}
												>
													{crumb.name}
												</span>
											</Link>
										)}
									</BreadcrumbItem>
									{idx < crumbs.length - 1 && <BreadcrumbSeparator />}
								</React.Fragment>
							))}
						</BreadcrumbList>
					</Breadcrumb>
				</nav>
			)}
			{pageInfo && <PageInfo {...pageInfo} />}
			<ScrollArea className="min-h-0 flex-1" data-testid="page-content">
				{children}
			</ScrollArea>
		</div>
	);
}
