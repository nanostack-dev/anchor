import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { cn } from "@/lib/utils";
import { Link, useLocation } from "@tanstack/react-router";
import { HomeIcon } from "lucide-react";
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

/**
 * A breadcrumb trail must carry exactly one `aria-current="page"`, on the final
 * crumb. TanStack Router marks a `<Link>` active on a prefix match by default
 * (`exact: false`), so every ancestor crumb would also claim to be the current
 * page — three claims at /products/checkout-api/api-keys, all announced by a
 * screen reader. Ancestors are by definition not the current page, so the crumb
 * links opt into exact matching and leave the marker to `BreadcrumbPage`.
 */
const crumbActiveOptions = { exact: true } as const;

type PageVariant = "full" | "wide" | "narrow" | "default";

type PageProps = {
	children: React.ReactNode;
	title?: string;
	description?: string;
	actions?: React.ReactNode;
	breadCrumbs?: boolean;
	/**
	 * Replaces the text of the last crumb. A detail route's final path segment
	 * is an identifier, and a breadcrumb reading `Org_3Hy...` names nothing —
	 * pass the record's own name instead.
	 */
	breadCrumbLabel?: string;
	variant?: PageVariant;
	pageInfo?: PageInfoProps;
};

const variantWidth: Record<PageVariant, string> = {
	full: "",
	wide: "mx-auto w-full max-w-[1200px]",
	narrow: "mx-auto w-full max-w-[600px]",
	default: "mx-auto w-full max-w-[900px]",
};

/**
 * Anchor route page shell. Composes the shared Nanostack page pattern
 * (heading/description/actions header + body) while keeping an Anchor-local,
 * route-aware breadcrumb adapter derived from the TanStack Router location.
 */
export function Page({
	children,
	title,
	description,
	actions,
	breadCrumbs = true,
	breadCrumbLabel,
	variant = "full",
	pageInfo,
}: PageProps) {
	const location = useLocation();
	const crumbs = useMemo(() => {
		const trail = getBreadcrumbs(location.pathname);
		if (breadCrumbLabel && trail.length > 0) {
			trail[trail.length - 1] = {
				...trail[trail.length - 1],
				name: breadCrumbLabel,
			};
		}
		return trail;
	}, [location.pathname, breadCrumbLabel]);

	return (
		<section
			data-slot="page"
			data-testid="page-root"
			className={cn(
				"flex min-h-full flex-col gap-6 p-4 lg:p-6",
				variantWidth[variant],
			)}
		>
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
												activeOptions={crumbActiveOptions}
												className="inline-flex items-center gap-1 transition-colors hover:text-foreground focus-visible:underline"
												aria-label="Dashboard"
											>
												<HomeIcon className="size-4" />
												<span
													className="max-w-[120px] truncate"
													title={crumb.name}
												>
													{crumb.name}
												</span>
											</Link>
										) : idx === crumbs.length - 1 ? (
											<BreadcrumbPage aria-current="page">
												<span
													className="max-w-[120px] truncate"
													title={crumb.name}
												>
													{crumb.name}
												</span>
											</BreadcrumbPage>
										) : (
											<Link
												to={crumb.path}
												activeOptions={crumbActiveOptions}
												className="transition-colors hover:text-foreground focus-visible:underline"
											>
												<span
													className="max-w-[120px] truncate"
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

			{(title || actions) && (
				<header
					data-slot="page-header"
					className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between"
				>
					<div className="max-w-3xl min-w-0 space-y-2">
						{title && (
							<h1
								data-testid="page-title"
								className="text-3xl font-semibold tracking-tight text-balance text-foreground"
							>
								{title}
							</h1>
						)}
						{description && (
							<p
								data-testid="page-description"
								className="text-sm leading-6 text-muted-foreground"
							>
								{description}
							</p>
						)}
					</div>
					{actions && (
						<div className="flex flex-wrap items-center gap-3 lg:justify-end">
							{actions}
						</div>
					)}
				</header>
			)}

			{pageInfo && <PageInfo {...pageInfo} />}

			<div className="min-h-0 flex-1" data-testid="page-content">
				{children}
			</div>
		</section>
	);
}
