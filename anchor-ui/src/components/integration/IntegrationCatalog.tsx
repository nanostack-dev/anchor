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
import { useProduct } from "@/hooks/useProduct";
import { productIntegrationsRoute } from "@/routes/platform/$productId.integrations";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { CopyIconButton } from "@nanostackorg/design-system/blocks/copy-button";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
import {
	StatCard,
	StatCardLabel,
	StatCardValue,
} from "@nanostackorg/design-system/blocks/stat-card";
import {
	Alert,
	AlertDescription,
} from "@nanostackorg/design-system/components/alert";
import {
	Button,
	ButtonLink,
} from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardFooter,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { ArrowRight, Fingerprint, Mail, Plug } from "lucide-react";
import { useEffect, useMemo } from "react";

function getWebhookUrl(productId: string, providerType: string): string {
	const baseUrl = import.meta.env.VITE_API_BASE_URL || "";
	return `${baseUrl}/v1/products/${productId}/integrations/webhooks/${providerType}`;
}

function getLiveState(instance: IntegrationInstanceResponse | null): {
	label: string;
	tone: StatusTone;
} {
	if (!instance) {
		return {
			label: "Not configured",
			tone: "neutral",
		};
	}

	if (!instance.is_enabled) {
		return {
			label: "Disabled",
			tone: "neutral",
		};
	}

	switch (instance.status) {
		case IntegrationInstanceStatus.ACTIVE:
			return {
				label: "Live",
				tone: "success",
			};
		case IntegrationInstanceStatus.ERROR:
			return {
				label: "Needs attention",
				tone: "destructive",
			};
		case IntegrationInstanceStatus.CONFIGURING:
			return {
				label: "Configuring",
				tone: "warning",
			};
		default:
			return {
				label: "Paused",
				tone: "warning",
			};
	}
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
	const configuredCount = [clerkInstance, smtpInstance].filter(Boolean).length;
	const liveCount = [clerkInstance, smtpInstance].filter(
		(i) => i?.is_enabled && i?.status === IntegrationInstanceStatus.ACTIVE,
	).length;
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
				<EmptyState
					title="Product not found"
					description="Product not found or unavailable."
				/>
			</Page>
		);
	}

	return (
		<Page
			title="Integration Hub"
			description={`Manage provider connections for ${activeProduct?.name ?? productId}`}
			breadCrumbLabels={
				activeProduct ? { [productId]: activeProduct.name } : undefined
			}
		>
			<Stack space="lg">
				<Text tone="muted">
					Pick a provider card to connect and monitor status. Detailed
					configuration lives on the provider detail page.
				</Text>
				<Box className="grid grid-cols-2 gap-3 md:max-w-md">
					<StatCard variant="outline" size="sm">
						<StatCardLabel>Configured</StatCardLabel>
						<StatCardValue>{configuredCount}</StatCardValue>
					</StatCard>
					<StatCard variant="outline" size="sm">
						<StatCardLabel>Live</StatCardLabel>
						<StatCardValue>{liveCount}</StatCardValue>
					</StatCard>
				</Box>

				{error ? (
					<Alert tone="critical">
						<AlertDescription>
							Failed to load integration instances.
						</AlertDescription>
					</Alert>
				) : null}

				<Box className="grid gap-4 md:grid-cols-2">
					<Card variant="outline">
						<CardHeader>
							<Spread alignY="start">
								<Box className="inline-flex size-10 items-center justify-center rounded-xl bg-muted text-foreground">
									<Fingerprint className="size-5" />
								</Box>
								<StatusBadge tone={clerkState.tone}>
									{clerkState.label}
								</StatusBadge>
							</Spread>
							<CardTitle>Clerk</CardTitle>
							<CardDescription>
								Identity lifecycle sync for product users.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								{isLoading ? (
									<Inline>
										<Spinner />
										<Text tone="muted">Loading status...</Text>
									</Inline>
								) : clerkInstance ? (
									<Stack space="sm">
										<Spread>
											<Text size="sm" tone="muted">
												Instance
											</Text>
											<Inline space="xs">
												<Text size="xs" font="mono" truncate>
													{clerkInstance.id}
												</Text>
												<CopyIconButton
													value={clerkInstance.id}
													label="Copy Clerk instance ID"
													size="sm"
												/>
											</Inline>
										</Spread>
										<Spread>
											<Text size="sm" tone="muted">
												Webhook URL
											</Text>
											<CopyIconButton
												label="Copy webhook URL"
												size="sm"
												value={getWebhookUrl(
													clerkInstance.product_id,
													clerkInstance.provider_type,
												)}
											/>
										</Spread>
										{clerkInstance.last_error ? (
											<Alert tone="warning">
												<AlertDescription>
													{clerkInstance.last_error}
												</AlertDescription>
											</Alert>
										) : null}
									</Stack>
								) : (
									<Text tone="muted">
										Not connected yet. Create the instance to activate webhook
										ingress.
									</Text>
								)}

								{createMutation.isError ? (
									<Text size="xs" tone="critical">
										Failed to create integration instance.
									</Text>
								) : null}
							</Stack>
						</CardContent>
						<CardFooter>
							{clerkInstance ? (
								<ButtonLink
									variant="solid"
									tone="brand"
									href={ROUTE_PATHS.PRODUCT_INTEGRATION_CLERK.replace(
										"$productId",
										productId,
									)}
								>
									Open details
									<ArrowRight className="size-3.5" />
								</ButtonLink>
							) : (
								<Button
									variant="solid"
									tone="brand"
									onClick={handleConnectClerk}
									disabled={createMutation.isPending}
								>
									{createMutation.isPending ? (
										<Spinner />
									) : (
										<Plug className="mr-1 size-3.5" />
									)}
									Create and configure
								</Button>
							)}
						</CardFooter>
					</Card>

					<Card variant="outline">
						<CardHeader>
							<Spread alignY="start">
								<Box className="inline-flex size-10 items-center justify-center rounded-xl bg-muted text-foreground">
									<Mail className="size-5" />
								</Box>
								<StatusBadge tone={smtpState.tone}>
									{smtpState.label}
								</StatusBadge>
							</Spread>
							<CardTitle>SMTP Email</CardTitle>
							<CardDescription>
								Outbound transactional email via any SMTP provider.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								{isLoading ? (
									<Inline>
										<Spinner />
										<Text tone="muted">Loading status...</Text>
									</Inline>
								) : smtpInstance ? (
									<Stack space="sm">
										<Spread>
											<Text size="sm" tone="muted">
												Instance
											</Text>
											<Text size="xs" font="mono" truncate>
												{smtpInstance.id}
											</Text>
										</Spread>
										{smtpInstance.last_error ? (
											<Alert tone="warning">
												<AlertDescription>
													{smtpInstance.last_error}
												</AlertDescription>
											</Alert>
										) : null}
									</Stack>
								) : (
									<Text tone="muted">
										Not connected yet. Configure SMTP credentials to enable
										email.
									</Text>
								)}
							</Stack>
						</CardContent>
						<CardFooter>
							<ButtonLink
								variant="solid"
								tone="brand"
								href={ROUTE_PATHS.PRODUCT_INTEGRATION_SMTP.replace(
									"$productId",
									productId,
								)}
							>
								{smtpInstance ? "Open details" : "Configure"}
								<ArrowRight className="size-3.5" />
							</ButtonLink>
						</CardFooter>
					</Card>
				</Box>
			</Stack>
		</Page>
	);
}
