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
import { Card } from "@/components/ui/card";
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
	Boxes,
	Building2,
	KeyRound,
	type LucideIcon,
	UserPlus,
	Users,
} from "lucide-react";

const ONE = { pagination: { limit: 1, offset: 0 } } as const;

type StatCardProps = {
	to: string;
	label: string;
	value?: number;
	isLoading: boolean;
	icon: LucideIcon;
};

function StatCard({ to, label, value, isLoading, icon: Icon }: StatCardProps) {
	return (
		<Link
			to={to}
			className="group block rounded-xl focus-visible:outline-none"
			aria-label={`${label}: view all`}
		>
			<Card className="gap-3 p-5 transition-colors group-hover:border-border-strong group-hover:bg-accent-soft group-focus-visible:ring-2 group-focus-visible:ring-ring">
				<div className="flex items-center justify-between gap-2">
					<span className="text-sm text-muted-foreground">{label}</span>
					<Icon className="size-4 text-muted-foreground" />
				</div>
				{isLoading ? (
					<Skeleton className="h-9 w-16" />
				) : (
					<span className="text-3xl font-semibold tracking-tight tabular-nums">
						{value ?? 0}
					</span>
				)}
			</Card>
		</Link>
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

	return (
		<Page
			title="Dashboard"
			description={
				user
					? `Welcome back, ${user.email}`
					: "Overview of your Anchor workspace"
			}
		>
			<div className="flex flex-col gap-8">
				<section className="flex flex-col gap-3">
					<h2 className="text-sm font-medium text-muted-foreground">
						Platform
					</h2>
					<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
						<StatCard
							to={ROUTE_PATHS.PRODUCTS}
							label="Products"
							value={products.data?.total}
							isLoading={products.isLoading}
							icon={Boxes}
						/>
						<StatCard
							to={ROUTE_PATHS.PLATFORM_USERS}
							label="Platform users"
							value={platformUsers.data?.total}
							isLoading={platformUsers.isLoading}
							icon={Users}
						/>
						<StatCard
							to={ROUTE_PATHS.PLATFORM_INVITATIONS}
							label="Invitations"
							value={invitations.data?.total}
							isLoading={invitations.isLoading}
							icon={UserPlus}
						/>
					</div>
				</section>

				<section className="flex flex-col gap-3">
					<h2 className="text-sm font-medium text-muted-foreground">
						{currentProduct
							? `Current product · ${currentProduct.name}`
							: "Current product"}
					</h2>
					{currentProduct ? (
						<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
							<StatCard
								to={ROUTE_PATHS.PRODUCT_USERS}
								label="Users"
								value={productUsers.data?.total}
								isLoading={productUsers.isLoading}
								icon={Users}
							/>
							<StatCard
								to={ROUTE_PATHS.PRODUCT_API_KEYS}
								label="API keys"
								value={productApiKeys.data?.total}
								isLoading={productApiKeys.isLoading}
								icon={KeyRound}
							/>
							<StatCard
								to={ROUTE_PATHS.ORGANIZATIONS}
								label="Organizations"
								value={productOrgs.data?.total}
								isLoading={productOrgs.isLoading}
								icon={Building2}
							/>
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
