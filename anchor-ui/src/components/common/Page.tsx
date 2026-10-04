import { cn } from "@/lib/utils";
import {
	PageHeader,
	PageHeaderActions,
	PageHeaderContent,
	PageHeaderDescription,
	PageHeaderTitle,
} from "@nanostackorg/design-system/blocks/page-header";
import {
	Breadcrumb,
	BreadcrumbItem,
	BreadcrumbLink,
	BreadcrumbList,
	BreadcrumbPage,
	BreadcrumbSeparator,
} from "@nanostackorg/design-system/components/breadcrumb";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { useLocation } from "@tanstack/react-router";
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

type PageVariant = "full" | "wide" | "narrow" | "default";

type PageProps = {
	children: React.ReactNode;
	title?: string;
	description?: string;
	actions?: React.ReactNode;
	breadCrumbs?: boolean;
	/**
	 * Renames crumbs, keyed by the path segment they were derived from. A
	 * detail route's identifier segment reads `Org_3Hy...`, which names
	 * nothing — pass the record's own name against that identifier instead.
	 */
	breadCrumbLabels?: Record<string, string>;
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
	breadCrumbLabels,
	variant = "full",
	pageInfo,
}: PageProps) {
	const location = useLocation();
	const crumbs = useMemo(() => {
		const trail = getBreadcrumbs(location.pathname);
		if (!breadCrumbLabels) return trail;
		return trail.map((crumb) => {
			const segment = crumb.path.split("/").pop() ?? "";
			const label = breadCrumbLabels[segment];
			return label ? { ...crumb, name: label } : crumb;
		});
	}, [location.pathname, breadCrumbLabels]);

	return (
		<Box
			as="section"
			data-slot="page"
			data-testid="page-root"
			className={cn(
				"flex min-h-full flex-col gap-6 p-4 lg:p-6",
				variantWidth[variant],
			)}
		>
			{breadCrumbs && (
				<Breadcrumb aria-label="Breadcrumb" data-testid="breadcrumb-nav">
					<BreadcrumbList>
						{crumbs.map((crumb, idx) => (
							<React.Fragment key={crumb.path}>
								<BreadcrumbItem>
									{idx === crumbs.length - 1 && idx !== 0 ? (
										<BreadcrumbPage>{crumb.name}</BreadcrumbPage>
									) : (
										<BreadcrumbLink
											href={crumb.path}
											aria-label={idx === 0 ? "Dashboard" : undefined}
										>
											<Inline space="xs">
												{idx === 0 && (
													<HomeIcon aria-hidden className="size-4" />
												)}
												{crumb.name}
											</Inline>
										</BreadcrumbLink>
									)}
								</BreadcrumbItem>
								{idx < crumbs.length - 1 && <BreadcrumbSeparator />}
							</React.Fragment>
						))}
					</BreadcrumbList>
				</Breadcrumb>
			)}
			{(title || actions) && (
				<PageHeader>
					<PageHeaderContent>
						{title && (
							<PageHeaderTitle data-testid="page-title">
								{title}
							</PageHeaderTitle>
						)}
						{description && (
							<PageHeaderDescription data-testid="page-description">
								{description}
							</PageHeaderDescription>
						)}
					</PageHeaderContent>
					{actions && <PageHeaderActions>{actions}</PageHeaderActions>}
				</PageHeader>
			)}
			{pageInfo && <PageInfo {...pageInfo} />}
			<Box className="min-h-0 flex-1" data-testid="page-content">
				{children}
			</Box>
		</Box>
	);
}
