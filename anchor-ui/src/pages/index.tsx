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
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { useProduct } from "@/hooks/useProduct";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
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

const formatCount = (value?: number) => (value ?? 0).toLocaleString();

type Stat = {
	to: string;
	label: string;
	value?: number;
	isLoading: boolean;
	icon: LucideIcon;
};

function StatCard({ stat, index }: { stat: Stat; index: number }) {
	const { to, label, value, isLoading, icon: Icon } = stat;
	return (
		<Link
			to={to}
			aria-label={`${label}: view all`}
			className="group block rounded-2xl outline-none motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-3 motion-safe:fill-mode-both"
			style={{ animationDelay: `${index * 70}ms`, animationDuration: "500ms" }}
		>
			<div className="relative h-full overflow-hidden rounded-2xl border border-border bg-card p-5 shadow-sm transition-all duration-300 group-hover:-translate-y-0.5 group-hover:border-border-strong group-hover:shadow-md group-focus-visible:ring-2 group-focus-visible:ring-ring">
				<span
					aria-hidden
					className="pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-primary/50 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"
				/>
				<div className="flex items-start justify-between">
					<span className="grid size-9 place-items-center rounded-xl bg-accent-soft text-primary">
						<Icon className="size-[18px]" />
					</span>
					<ArrowUpRight
						aria-hidden
						className="size-4 -translate-x-1 text-muted-foreground/40 opacity-0 transition-all duration-300 group-hover:translate-x-0 group-hover:opacity-100"
					/>
				</div>
				<div className="mt-5 flex flex-col gap-1">
					{isLoading ? (
						<Skeleton className="h-9 w-20" />
					) : (
						<span className="font-heading text-3xl font-semibold tracking-tight tabular-nums text-foreground">
							{formatCount(value)}
						</span>
					)}
					<span className="text-xs font-medium uppercase tracking-wider text-muted-foreground">
						{label}
					</span>
				</div>
			</div>
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

	return (
		<Page breadCrumbs={false}>
			<div className="flex flex-col gap-8">
				<header className="relative isolate overflow-hidden rounded-3xl border border-border bg-card px-6 py-8 shadow-sm motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-2 motion-safe:fill-mode-both motion-safe:duration-500">
					<div
						aria-hidden
						className="pointer-events-none absolute -right-20 -top-24 size-64 rounded-full bg-primary/10 blur-3xl"
					/>
					<div
						aria-hidden
						className="pointer-events-none absolute -bottom-24 left-10 size-48 rounded-full bg-chart-2/10 blur-3xl"
					/>
					<p className="text-xs font-semibold uppercase tracking-[0.22em] text-primary">
						Anchor · Organization-as-a-Service
					</p>
					<h1 className="mt-2 font-heading text-3xl font-semibold tracking-tight text-foreground">
						Dashboard
					</h1>
					<p className="mt-1.5 max-w-prose text-sm text-muted-foreground">
						{user
							? `Welcome back, ${user.email}`
							: "An overview of your workspace."}
					</p>
				</header>

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
			</div>
		</Page>
	);
}
