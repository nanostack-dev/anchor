import {
	type CreateIntegrationInstanceData,
	type IntegrationInstanceResponse,
	IntegrationInstanceStatus,
	IntegrationProviderType,
	type ListIntegrationInstancesData,
	type Options,
	type ProductResponse,
} from "@/client";
import {
	createIntegrationInstanceMutation,
	listIntegrationInstancesOptions,
	listIntegrationInstancesQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { StatusBadge, type StatusTone } from "@/components/common/StatusBadge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { useProduct } from "@/hooks/useProduct";
import { productIntegrationsRoute } from "@/routes/platform/$productId.integrations";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
	ArrowRight,
	Check,
	Copy,
	Fingerprint,
	Mail,
	Plug,
	Sparkles,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

function getWebhookUrl(productId: string, providerType: string): string {
	const baseUrl = import.meta.env.VITE_API_BASE_URL || "";
	return `${baseUrl}/v1/products/${productId}/integrations/webhooks/${providerType}`;
}

function getLiveState(instance: IntegrationInstanceResponse | null): {
	label: string;
	dotClassName: string;
	tone: StatusTone;
} {
	if (!instance) {
		return {
			label: "Not configured",
			dotClassName: "bg-muted-foreground",
			tone: "neutral",
		};
	}

	if (!instance.is_enabled) {
		return {
			label: "Disabled",
			dotClassName: "bg-muted-foreground",
			tone: "neutral",
		};
	}

	switch (instance.status) {
		case IntegrationInstanceStatus.ACTIVE:
			return {
				label: "Live",
				dotClassName: "bg-success",
				tone: "success",
			};
		case IntegrationInstanceStatus.ERROR:
			return {
				label: "Needs attention",
				dotClassName: "bg-destructive",
				tone: "destructive",
			};
		case IntegrationInstanceStatus.CONFIGURING:
			return {
				label: "Configuring",
				dotClassName: "bg-warning",
				tone: "warning",
			};
		default:
			return {
				label: "Paused",
				dotClassName: "bg-warning",
				tone: "warning",
			};
	}
}

function CopyButton({ value }: { value: string }) {
	const [copied, setCopied] = useState(false);

	const handleCopy = useCallback(() => {
		navigator.clipboard.writeText(value).then(() => {
			setCopied(true);
			setTimeout(() => setCopied(false), 1500);
		});
	}, [value]);

	return (
		<Button
			variant="ghost"
			size="sm"
			className="size-6 p-0 shrink-0"
			onClick={handleCopy}
			title="Copy to clipboard"
		>
			{copied ? (
				<Check className="size-3 text-success" />
			) : (
				<Copy className="size-3 text-muted-foreground" />
			)}
		</Button>
	);
}

export default function PlatformIntegrationsPage() {
	const { productId } = productIntegrationsRoute.useParams();
	const {
		currentProduct,
		products,
		selectProduct,
		isLoading: isProductsLoading,
	} = useProduct();
	const queryClient = useQueryClient();
	const navigate = useNavigate();

	const routeProduct = useMemo(
		() => products.find((product) => product.id === productId) ?? null,
		[products, productId],
	);

	const activeProduct: ProductResponse | null =
		routeProduct ?? (currentProduct?.id === productId ? currentProduct : null);

	useEffect(() => {
		if (routeProduct && currentProduct?.id !== routeProduct.id) {
			selectProduct(routeProduct);
		}
	}, [routeProduct, currentProduct, selectProduct]);

	const listQueryOptions: Options<ListIntegrationInstancesData> = {
		path: { product_id: productId },
	};

	const {
		data: listData,
		isLoading,
		error,
	} = useQuery({
		...listIntegrationInstancesOptions(listQueryOptions),
		enabled: !!productId,
	});

	const instances: IntegrationInstanceResponse[] = listData?.items ?? [];
	const clerkInstance =
		instances.find(
			(item) => item.provider_type === IntegrationProviderType.CLERK,
		) ?? null;
	const smtpInstance =
		instances.find(
			(item) => item.provider_type === IntegrationProviderType.SMTP,
		) ?? null;
	const clerkState = getLiveState(clerkInstance);
	const smtpState = getLiveState(smtpInstance);

	const createMutation = useMutation({
		...createIntegrationInstanceMutation(),
		onSuccess: () => {
			queryClient.invalidateQueries({
				queryKey: listIntegrationInstancesQueryKey(listQueryOptions),
			});
		},
	});

	const handleConnectClerk = () => {
		if (!productId) return;
		const createOptions: Options<CreateIntegrationInstanceData> = {
			path: { product_id: productId },
			body: { provider_type: IntegrationProviderType.CLERK },
		};
		createMutation.mutate(createOptions, {
			onSuccess: () => {
				navigate({
					to: ROUTE_PATHS.PRODUCT_INTEGRATION_CLERK,
					params: { productId },
				});
			},
		});
	};

	if (!activeProduct && !isProductsLoading) {
		return (
			<Page
				title="Integrations"
				description="Configure provider integrations for a specific product"
			>
				<div className="flex h-48 items-center justify-center rounded-lg border border-dashed">
					<p className="text-sm text-muted-foreground">
						Product not found or unavailable.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Integration Hub"
			description={`Manage provider connections for ${activeProduct?.name ?? productId}`}
		>
			<div className="space-y-6 pb-6">
				<section className="rounded-2xl border bg-card p-6">
					<div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
						<div className="space-y-2">
							<h2 className="inline-flex items-center gap-2 text-xl font-semibold md:text-2xl">
								<Sparkles className="size-5 text-warning" />
								Integrations
							</h2>
							<p className="max-w-2xl text-sm text-muted-foreground">
								Pick a provider card to connect and monitor status. Detailed
								configuration lives on the provider detail page.
							</p>
						</div>
						<div className="grid grid-cols-2 gap-2 md:w-64">
							<div className="rounded-xl border bg-card p-3">
								<p className="text-xs uppercase tracking-wide text-muted-foreground">
									Configured
								</p>
								<p className="text-2xl font-semibold">
									{[clerkInstance, smtpInstance].filter(Boolean).length}
								</p>
							</div>
							<div className="rounded-xl border bg-card p-3">
								<p className="text-xs uppercase tracking-wide text-muted-foreground">
									Live
								</p>
								<p className="text-2xl font-semibold text-success">
									{
										[clerkInstance, smtpInstance].filter(
											(i) =>
												i?.is_enabled &&
												i?.status === IntegrationInstanceStatus.ACTIVE,
										).length
									}
								</p>
							</div>
						</div>
					</div>
				</section>

				{error ? (
					<div className="rounded-xl border border-destructive/30 bg-destructive/10 px-4 py-3 text-sm text-destructive">
						Failed to load integration instances.
					</div>
				) : null}

				<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
					<Card className="border-primary/20 bg-card">
						<CardHeader>
							<div className="flex items-start justify-between gap-3">
								<div className="inline-flex size-10 items-center justify-center rounded-xl bg-muted text-foreground">
									<Fingerprint className="size-5" />
								</div>
								<div className="flex flex-col items-end gap-2">
									<StatusBadge tone={clerkState.tone}>
										{clerkState.label}
									</StatusBadge>
									<span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
										<span
											className={`size-2 rounded-full ${clerkState.dotClassName}`}
										/>
										Health
									</span>
								</div>
							</div>
							<CardTitle className="mt-4">Clerk</CardTitle>
							<CardDescription>
								Identity lifecycle sync for product users.
							</CardDescription>
						</CardHeader>
						<CardContent className="space-y-3">
							{isLoading ? (
								<div className="flex items-center gap-2 text-sm text-muted-foreground">
									<Spinner className="size-4 text-current" />
									Loading status...
								</div>
							) : clerkInstance ? (
								<div className="space-y-2 rounded-xl border bg-muted/30 p-3 text-xs">
									<div className="flex items-center justify-between gap-2">
										<span className="text-muted-foreground">Instance</span>
										<div className="flex items-center gap-1">
											<span className="max-w-[120px] truncate font-mono text-[11px]">
												{clerkInstance.id}
											</span>
											<CopyButton value={clerkInstance.id} />
										</div>
									</div>
									<div className="flex items-center justify-between gap-2">
										<span className="text-muted-foreground">Webhook URL</span>
										<CopyButton
											value={getWebhookUrl(
												clerkInstance.product_id,
												clerkInstance.provider_type,
											)}
										/>
									</div>
									{clerkInstance.last_error ? (
										<p className="rounded-md border border-warning/30 bg-warning/10 px-2 py-1 text-[11px] text-warning">
											{clerkInstance.last_error}
										</p>
									) : null}
								</div>
							) : (
								<p className="text-sm text-muted-foreground">
									Not connected yet. Create the instance to activate webhook
									ingress.
								</p>
							)}

							<div className="flex flex-wrap gap-2">
								{clerkInstance ? (
									<Link
										to={ROUTE_PATHS.PRODUCT_INTEGRATION_CLERK}
										params={{ productId }}
										className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:opacity-90"
									>
										Open details
										<ArrowRight className="size-3.5" />
									</Link>
								) : (
									<Button
										onClick={handleConnectClerk}
										disabled={createMutation.isPending}
									>
										{createMutation.isPending ? (
											<Spinner className="mr-1 size-3.5 text-current" />
										) : (
											<Plug className="mr-1 size-3.5" />
										)}
										Create and configure
									</Button>
								)}
							</div>

							{createMutation.isError ? (
								<p className="text-xs text-destructive">
									Failed to create integration instance.
								</p>
							) : null}
						</CardContent>
					</Card>

					<Card className="border-primary/20 bg-card">
						<CardHeader>
							<div className="flex items-start justify-between gap-3">
								<div className="inline-flex size-10 items-center justify-center rounded-xl bg-muted text-foreground">
									<Mail className="size-5" />
								</div>
								<div className="flex flex-col items-end gap-2">
									<StatusBadge tone={smtpState.tone}>
										{smtpState.label}
									</StatusBadge>
									<span className="inline-flex items-center gap-1 text-xs text-muted-foreground">
										<span
											className={`size-2 rounded-full ${smtpState.dotClassName}`}
										/>
										Health
									</span>
								</div>
							</div>
							<CardTitle className="mt-4">SMTP Email</CardTitle>
							<CardDescription>
								Outbound transactional email via any SMTP provider.
							</CardDescription>
						</CardHeader>
						<CardContent className="space-y-3">
							{isLoading ? (
								<div className="flex items-center gap-2 text-sm text-muted-foreground">
									<Spinner className="size-4 text-current" />
									Loading status...
								</div>
							) : smtpInstance ? (
								<div className="space-y-2 rounded-xl border bg-muted/30 p-3 text-xs">
									<div className="flex items-center justify-between gap-2">
										<span className="text-muted-foreground">Instance</span>
										<span className="max-w-[120px] truncate font-mono text-[11px]">
											{smtpInstance.id}
										</span>
									</div>
									{smtpInstance.last_error ? (
										<p className="rounded-md border border-warning/30 bg-warning/10 px-2 py-1 text-[11px] text-warning">
											{smtpInstance.last_error}
										</p>
									) : null}
								</div>
							) : (
								<p className="text-sm text-muted-foreground">
									Not connected yet. Configure SMTP credentials to enable email.
								</p>
							)}

							<div className="flex flex-wrap gap-2">
								<Link
									to={ROUTE_PATHS.PRODUCT_INTEGRATION_SMTP}
									params={{ productId }}
									className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground hover:opacity-90"
								>
									{smtpInstance ? "Open details" : "Configure"}
									<ArrowRight className="size-3.5" />
								</Link>
							</div>
						</CardContent>
					</Card>
				</div>
			</div>
		</Page>
	);
}
