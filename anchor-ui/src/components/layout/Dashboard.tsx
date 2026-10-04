import { SortDirection } from "@/client";
import {
	getCurrentUserOptions,
	searchPlatformInvitationsOptions,
	searchPlatformUsersOptions,
	searchProductApiKeysOptions,
	searchProductOrganizationsOptions,
	searchProductUsersOptions,
	searchProductsOptions,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { useProduct } from "@/hooks/useProduct";
import { DashboardHero } from "@/pages/DashboardHero";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	StatCard as SharedStatCard,
	StatCardLabel,
	StatCardValue,
} from "@nanostackorg/design-system/blocks/stat-card";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { TextLink } from "@nanostackorg/design-system/components/text-link";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import dayjs from "dayjs";
import {
	ArrowUpRight,
	Boxes,
	Building2,
	KeyRound,
	type LucideIcon,
	UserPlus,
	Users,
} from "lucide-react";

const ONE = { pagination: { limit: 1, offset: 0 } } as const;
const RECENT = {
	pagination: { limit: 5, offset: 0 },
	sort_by: "created_at",
	sort_direction: SortDirection.DESC,
} as const;

const formatCount = (value?: number) => (value ?? 0).toLocaleString();
const formatDate = (value?: string) =>
	value ? dayjs(value).format("MMM D") : "";
const initial = (value: string) => value.trim().charAt(0).toUpperCase() || "?";

type Stat = {
	to: string;
	label: string;
	value?: number;
	isLoading: boolean;
	icon: LucideIcon;
};

function StatCard({ stat }: { stat: Stat; index: number }) {
	const { to, label, value, isLoading, icon: Icon } = stat;
	return (
		<Link
			to={to}
			aria-label={`${label}: view all`}
			className="block rounded-xl outline-none focus-visible:ring-3 focus-visible:ring-ring/30"
		>
			<SharedStatCard variant="outline">
				<Icon aria-hidden className="size-5 text-muted-foreground" />
				<StatCardValue>
					{isLoading ? (
						<Skeleton height="lg" width="1/2" />
					) : (
						formatCount(value)
					)}
				</StatCardValue>
				<StatCardLabel>{label}</StatCardLabel>
			</SharedStatCard>
		</Link>
	);
}

function SectionLabel({ children }: { children: React.ReactNode }) {
	return (
		<div className="flex items-center gap-3">
			<h2 className="text-xs font-semibold uppercase tracking-[0.18em] text-muted-foreground">
				{children}
			</h2>
			<span aria-hidden className="h-px flex-1 bg-border" />
		</div>
	);
}

function ResourceList({
	title,
	viewAllTo,
	isLoading,
	isEmpty,
	emptyText,
	children,
}: {
	title: string;
	viewAllTo: string;
	isLoading: boolean;
	isEmpty: boolean;
	emptyText: string;
	children: React.ReactNode;
}) {
	return (
		<div className="overflow-hidden rounded-2xl border border-border bg-card shadow-sm">
			<div className="flex items-center justify-between gap-2 border-b border-border px-5 py-3.5">
				<h3 className="text-sm font-semibold text-foreground">{title}</h3>
				<TextLink href={viewAllTo}>
					View all
					<ArrowUpRight aria-hidden className="size-3.5" />
				</TextLink>
			</div>
			{isLoading ? (
				<ul className="divide-y divide-border">
					{Array.from({ length: 4 }).map((_, i) => (
						<li
							// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton rows
							key={i}
							className="flex items-center gap-3 px-5 py-3"
						>
							<Skeleton height="sm" />
							<div className="flex min-w-0 flex-1 flex-col gap-1.5">
								<Skeleton height="sm" width="1/2" />
								<Skeleton height="sm" width="3/4" />
							</div>
						</li>
					))}
				</ul>
			) : isEmpty ? (
				<p className="px-5 py-10 text-center text-sm text-muted-foreground">
					{emptyText}
				</p>
			) : (
				<ul className="divide-y divide-border">{children}</ul>
			)}
		</div>
	);
}

export default function DashboardPage() {
	const { currentProduct } = useProduct();
	const { data: user } = useQuery({ ...getCurrentUserOptions() });

	const products = useQuery({ ...searchProductsOptions({ body: ONE }) });
	const platformUsers = useQuery({
		...searchPlatformUsersOptions({ body: ONE }),
	});
	const invitations = useQuery({
		...searchPlatformInvitationsOptions({ body: ONE }),
	});

	const recentProducts = useQuery({
		...searchProductsOptions({ body: RECENT }),
	});
	const recentUsers = useQuery({
		...searchPlatformUsersOptions({ body: RECENT }),
	});

	const productId = currentProduct?.id ?? "";
	const productUsers = useQuery({
		...searchProductUsersOptions({
			path: { product_id: productId },
			body: ONE,
		}),
		enabled: !!productId,
	});
	const productApiKeys = useQuery({
		...searchProductApiKeysOptions({
			path: { product_id: productId },
			body: ONE,
		}),
		enabled: !!productId,
	});
	const productOrgs = useQuery({
		...searchProductOrganizationsOptions({
			path: { product_id: productId },
			body: ONE,
		}),
		enabled: !!productId,
	});

	const platformStats: Stat[] = [
		{
			to: ROUTE_PATHS.PRODUCTS,
			label: "Products",
			value: products.data?.total,
			isLoading: products.isLoading,
			icon: Boxes,
		},
		{
			to: ROUTE_PATHS.PLATFORM_USERS,
			label: "Platform users",
			value: platformUsers.data?.total,
			isLoading: platformUsers.isLoading,
			icon: Users,
		},
		{
			to: ROUTE_PATHS.PLATFORM_INVITATIONS,
			label: "Invitations",
			value: invitations.data?.total,
			isLoading: invitations.isLoading,
			icon: UserPlus,
		},
	];

	const productStats: Stat[] = [
		{
			to: ROUTE_PATHS.PRODUCT_USERS,
			label: "Users",
			value: productUsers.data?.total,
			isLoading: productUsers.isLoading,
			icon: Users,
		},
		{
			to: ROUTE_PATHS.PRODUCT_API_KEYS,
			label: "API keys",
			value: productApiKeys.data?.total,
			isLoading: productApiKeys.isLoading,
			icon: KeyRound,
		},
		{
			to: ROUTE_PATHS.ORGANIZATIONS,
			label: "Organizations",
			value: productOrgs.data?.total,
			isLoading: productOrgs.isLoading,
			icon: Building2,
		},
	];

	const recentProductItems = recentProducts.data?.items ?? [];
	const recentUserItems = recentUsers.data?.items ?? [];

	return (
		<Page breadCrumbs={false}>
			<div className="flex flex-col gap-8">
				<DashboardHero
					subtitle={
						user
							? `Welcome back, ${user.email}`
							: "An overview of your workspace."
					}
				/>

				<section className="flex flex-col gap-4">
					<SectionLabel>Platform</SectionLabel>
					<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
						{platformStats.map((stat, index) => (
							<StatCard key={stat.label} stat={stat} index={index} />
						))}
					</div>
				</section>

				<section className="flex flex-col gap-4">
					<SectionLabel>
						{currentProduct
							? `Current product · ${currentProduct.name}`
							: "Current product"}
					</SectionLabel>
					{currentProduct ? (
						<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
							{productStats.map((stat, index) => (
								<StatCard key={stat.label} stat={stat} index={index + 3} />
							))}
						</div>
					) : (
						<Empty>
							<EmptyHeader>
								<EmptyTitle>No product selected</EmptyTitle>
								<EmptyDescription>
									Pick a product from the top bar to see its users, API keys,
									and organizations.
								</EmptyDescription>
							</EmptyHeader>
						</Empty>
					)}
				</section>

				<section className="flex flex-col gap-4">
					<SectionLabel>Recent</SectionLabel>
					<div className="grid gap-4 lg:grid-cols-2">
						<ResourceList
							title="Products"
							viewAllTo={ROUTE_PATHS.PRODUCTS}
							isLoading={recentProducts.isLoading}
							isEmpty={recentProductItems.length === 0}
							emptyText="No products yet."
						>
							{recentProductItems.map((product) => (
								<li
									key={product.id}
									className="flex items-center gap-3 px-5 py-3"
								>
									<span className="grid size-8 shrink-0 place-items-center rounded-lg bg-accent text-xs font-semibold text-primary">
										{initial(product.name)}
									</span>
									<span className="flex min-w-0 flex-1 flex-col">
										<span className="truncate text-sm font-medium text-foreground">
											{product.name}
										</span>
										<span className="truncate text-xs text-muted-foreground">
											{product.description || "No description"}
										</span>
									</span>
									<span className="shrink-0 text-xs tabular-nums text-muted-foreground">
										{formatDate(product.created_at)}
									</span>
								</li>
							))}
						</ResourceList>

						<ResourceList
							title="Platform users"
							viewAllTo={ROUTE_PATHS.PLATFORM_USERS}
							isLoading={recentUsers.isLoading}
							isEmpty={recentUserItems.length === 0}
							emptyText="No platform users yet."
						>
							{recentUserItems.map((member) => (
								<li
									key={member.id}
									className="flex items-center gap-3 px-5 py-3"
								>
									<span className="grid size-8 shrink-0 place-items-center rounded-lg bg-accent text-xs font-semibold text-primary">
										{initial(member.email)}
									</span>
									<span className="flex min-w-0 flex-1 flex-col">
										<span className="truncate text-sm font-medium text-foreground">
											{member.email}
										</span>
										<span className="truncate text-xs text-muted-foreground">
											{member.role}
										</span>
									</span>
									<span className="shrink-0 text-xs tabular-nums text-muted-foreground">
										{formatDate(member.created_at)}
									</span>
								</li>
							))}
						</ResourceList>
					</div>
				</section>
			</div>
		</Page>
	);
}
