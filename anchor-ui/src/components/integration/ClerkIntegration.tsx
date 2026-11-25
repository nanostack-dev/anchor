import {
	type ClerkIntegrationConfig,
	type CreateIntegrationInstanceData,
	type DeleteIntegrationInstanceData,
	type IntegrationAuditLogEntryResponse,
	type IntegrationAuditLogSearchResponse,
	IntegrationAuditLogSeverity,
	type IntegrationInstanceResponse,
	IntegrationProviderType,
	type ListIntegrationInstancesData,
	type Options,
	type ProductResponse,
	SortDirection,
	type UpdateIntegrationInstanceData,
	searchIntegrationAuditLogs,
} from "@/client";
import {
	createIntegrationInstanceMutation,
	deleteIntegrationInstanceMutation,
	listIntegrationInstancesOptions,
	listIntegrationInstancesQueryKey,
	searchIntegrationAuditLogsQueryKey,
	updateIntegrationInstanceMutation,
} from "@/client/@tanstack/react-query.gen";
import {
	zClerkIntegrationConfig,
	zIntegrationInstanceUpdateRequest,
} from "@/client/zod.gen";
import { Page } from "@/components/common/Page";
import {
	type IntegrationAuditEntry,
	IntegrationDetailPage,
} from "@/components/integration/IntegrationDetailPage";
import { useProduct } from "@/hooks/useProduct";
import { productIntegrationClerkRoute } from "@/routes/platform/$productId.integration-clerk";
import { CopyIconButton } from "@nanostackorg/design-system/blocks/copy-button";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "@nanostackorg/design-system/components/alert-dialog";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { Text } from "@nanostackorg/design-system/components/text";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { PlugsIcon } from "@phosphor-icons/react";
import {
	useInfiniteQuery,
	useMutation,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import dayjs from "dayjs";
import { Info, Plug, Shield, Trash2, TriangleAlert } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

interface IntegrationFormState {
	webhookSecret: string;
	apiKey: string;
	enabled: boolean;
}

const AUDIT_PAGE_SIZE = 50;

const EMPTY_FORM: IntegrationFormState = {
	webhookSecret: "",
	apiKey: "",
	enabled: true,
};

function mapAuditSeverity(
	severity: IntegrationAuditLogSeverity,
): IntegrationAuditEntry["severity"] {
	switch (severity) {
		case IntegrationAuditLogSeverity.SUCCESS:
			return "success";
		case IntegrationAuditLogSeverity.WARNING:
			return "warning";
		case IntegrationAuditLogSeverity.ERROR:
			return "error";
		default:
			return "info";
	}
}

function mapAuditEntry(
	entry: IntegrationAuditLogEntryResponse,
): IntegrationAuditEntry {
	let description = entry.message;
	if (entry.metadata && Object.keys(entry.metadata).length > 0) {
		description = `${entry.message} - ${Object.entries(entry.metadata)
			.map(([key, value]) => `${key}: ${String(value)}`)
			.join(" | ")}`;
	}

	return {
		id: entry.id,
		title: entry.action.replaceAll("_", " "),
		description,
		timestamp: entry.created_at,
		severity: mapAuditSeverity(entry.severity),
	};
}

function getWebhookUrl(productId: string, providerType: string): string {
	const baseUrl = import.meta.env.VITE_API_BASE_URL || "";
	return `${baseUrl}/v1/products/${productId}/integrations/webhooks/${providerType}`;
}

export default function ClerkIntegrationPage() {
	const { productId } = productIntegrationClerkRoute.useParams();
	const {
		currentProduct,
		products,
		selectProduct,
		isLoading: isProductsLoading,
	} = useProduct();
	const queryClient = useQueryClient();

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

	const [form, setForm] = useState<IntegrationFormState>(EMPTY_FORM);
	const [isEditingWebhookSecret, setIsEditingWebhookSecret] = useState(false);
	const [formErrors, setFormErrors] = useState<
		Partial<Record<keyof IntegrationFormState, string>>
	>({});

	const listQueryOptions: Options<ListIntegrationInstancesData> = {
		path: { product_id: productId },
	};

	const { data: listData, isLoading } = useQuery({
		...listIntegrationInstancesOptions(listQueryOptions),
		enabled: !!productId,
	});

	const clerkInstance: IntegrationInstanceResponse | null = useMemo(() => {
		return (
			(listData?.items ?? []).find(
				(item) => item.provider_type === IntegrationProviderType.CLERK,
			) ?? null
		);
	}, [listData]);

	const auditQueryKey = searchIntegrationAuditLogsQueryKey({
		path: {
			product_id: productId,
			integration_instance_id: clerkInstance?.id ?? "",
		},
		body: {
			pagination: { limit: AUDIT_PAGE_SIZE },
			sort_by: "created_at",
			sort_direction: SortDirection.DESC,
		},
	});

	const {
		data: auditData,
		isLoading: isAuditLoading,
		error: auditError,
		hasNextPage: hasOlderAudit,
		fetchNextPage: fetchOlderAudit,
		isFetchingNextPage: isFetchingOlderAudit,
		isFetchNextPageError: isOlderAuditError,
	} = useInfiniteQuery({
		queryKey: auditQueryKey,
		initialPageParam: undefined as string | undefined,
		getNextPageParam: (
			lastPage: IntegrationAuditLogSearchResponse,
		): string | undefined => lastPage.next_cursor,
		queryFn: async ({ pageParam, signal }) => {
			const { data } = await searchIntegrationAuditLogs({
				path: {
					product_id: productId,
					integration_instance_id: clerkInstance?.id ?? "",
				},
				body: {
					pagination: { limit: AUDIT_PAGE_SIZE, cursor: pageParam },
					sort_by: "created_at",
					sort_direction: SortDirection.DESC,
				},
				signal,
				throwOnError: true,
			});
			return data;
		},
		enabled: Boolean(clerkInstance),
	});

	useEffect(() => {
		if (!clerkInstance) {
			setForm(EMPTY_FORM);
			setFormErrors({});
			return;
		}

		setForm({
			webhookSecret: "",
			apiKey: "",
			enabled: clerkInstance.is_enabled,
		});
		setIsEditingWebhookSecret(false);
		setFormErrors({});
	}, [clerkInstance]);

	const isDirty = useMemo(() => {
		if (!clerkInstance) {
			return false;
		}

		return (
			form.enabled !== clerkInstance.is_enabled ||
			form.webhookSecret.trim().length > 0 ||
			form.apiKey.trim().length > 0
		);
	}, [clerkInstance, form]);

	const invalidateList = () => {
		queryClient.invalidateQueries({
			queryKey: listIntegrationInstancesQueryKey(listQueryOptions),
		});
		if (clerkInstance) {
			queryClient.invalidateQueries({
				queryKey: auditQueryKey,
			});
		}
	};

	const createMutation = useMutation({
		...createIntegrationInstanceMutation(),
		onSuccess: () => {
			invalidateList();
		},
	});

	const updateMutation = useMutation({
		...updateIntegrationInstanceMutation(),
		onSuccess: () => {
			invalidateList();
		},
	});

	const deleteMutation = useMutation({
		...deleteIntegrationInstanceMutation(),
		onSuccess: () => {
			invalidateList();
			setForm(EMPTY_FORM);
		},
	});

	const isMutating =
		createMutation.isPending ||
		updateMutation.isPending ||
		deleteMutation.isPending;

	const handleCreate = () => {
		if (!productId) return;
		const createOptions: Options<CreateIntegrationInstanceData> = {
			path: { product_id: productId },
			body: { provider_type: IntegrationProviderType.CLERK },
		};
		createMutation.mutate(createOptions);
	};

	const handleSave = () => {
		if (!productId || !clerkInstance) return;

		setFormErrors({});

		const nextConfig: ClerkIntegrationConfig = {};
		const trimmedApiKey = form.apiKey.trim();

		if (trimmedApiKey.length > 0) {
			nextConfig.api_key = trimmedApiKey;
		}

		if (Object.keys(nextConfig).length > 0) {
			const parsedConfig = zClerkIntegrationConfig.safeParse(nextConfig);
			if (!parsedConfig.success) {
				setFormErrors((prev) => ({
					...prev,
					apiKey:
						parsedConfig.error.issues[0]?.message ??
						"Invalid Clerk configuration.",
				}));
				return;
			}
		}

		const updateBody: UpdateIntegrationInstanceData["body"] = {
			is_enabled: form.enabled,
			...(Object.keys(nextConfig).length > 0 ? { config: nextConfig } : {}),
			...(form.webhookSecret.length > 0
				? { webhook_secret: form.webhookSecret }
				: {}),
		};

		const parsedUpdate =
			zIntegrationInstanceUpdateRequest.safeParse(updateBody);
		if (!parsedUpdate.success) {
			const nextErrors: Partial<Record<keyof IntegrationFormState, string>> =
				{};
			for (const issue of parsedUpdate.error.issues) {
				const field = issue.path[0];
				if (field === "webhook_secret" && !nextErrors.webhookSecret) {
					nextErrors.webhookSecret = issue.message;
				}
				if (field === "is_enabled" && !nextErrors.enabled) {
					nextErrors.enabled = issue.message;
				}
				if (field === "config" && !nextErrors.apiKey) {
					nextErrors.apiKey = issue.message;
				}
			}
			setFormErrors(nextErrors);
			return;
		}

		const updateOptions: Options<UpdateIntegrationInstanceData> = {
			path: {
				product_id: productId,
				integration_instance_id: clerkInstance.id,
			},
			body: updateBody,
		};
		updateMutation.mutate(updateOptions);
	};

	const handleReset = useCallback(() => {
		if (!clerkInstance) {
			return;
		}

		setForm({
			webhookSecret: "",
			apiKey: "",
			enabled: clerkInstance.is_enabled,
		});
		setIsEditingWebhookSecret(false);
		setFormErrors({});
	}, [clerkInstance]);

	const handleDelete = () => {
		if (!productId || !clerkInstance) return;
		const deleteOptions: Options<DeleteIntegrationInstanceData> = {
			path: {
				product_id: productId,
				integration_instance_id: clerkInstance.id,
			},
		};
		deleteMutation.mutate(deleteOptions);
	};

	const auditEntries: IntegrationAuditEntry[] = useMemo(() => {
		return (auditData?.pages ?? [])
			.flatMap((page) => page.items)
			.map(mapAuditEntry);
	}, [auditData]);

	if (!activeProduct && !isProductsLoading) {
		return (
			<Page
				title="Clerk Integration"
				description="Configure identity integration instances for a specific product"
			>
				<Card variant="outline">
					<CardContent>
						<Stack space="md">
							<p className="max-w-sm text-sm text-muted-foreground">
								Product not found or unavailable.
							</p>
						</Stack>
					</CardContent>
				</Card>
			</Page>
		);
	}

	return (
		<IntegrationDetailPage
			breadCrumbLabels={{
				...(activeProduct ? { [productId]: activeProduct.name } : {}),
				"integration-clerk": "Clerk",
			}}
			title="Clerk Authentication"
			description="Configure webhook security, sync controls, and runtime behavior for Clerk identity events."
			backLink={
				<Inline>
					<Badge variant="soft">
						Product {activeProduct?.name ?? productId}
					</Badge>
					<Badge variant="soft">Provider Clerk</Badge>
					{clerkInstance?.id ? (
						<Badge variant="soft">{clerkInstance.id}</Badge>
					) : null}
				</Inline>
			}
			summary={[
				{
					label: "Connection",
					value: clerkInstance
						? clerkInstance.is_enabled
							? "Live"
							: "Paused"
						: "Not configured",
				},
				{
					label: "Webhook",
					value: clerkInstance ? "Ready" : "Not set",
				},
				{
					label: "Activity",
					value: auditData?.pages[0]?.total ?? 0,
				},
				{
					label: "Last updated",
					value: clerkInstance
						? dayjs(clerkInstance.updated_at).format("D MMMM YYYY H:mm")
						: "Never",
				},
			]}
			auditEntries={auditEntries}
			auditIsLoading={Boolean(clerkInstance) && isAuditLoading}
			auditErrorMessage={
				auditError && !auditData
					? "Failed to load integration audit activity."
					: null
			}
			auditTitle="Integration audit"
			onLoadMoreAudit={hasOlderAudit ? () => void fetchOlderAudit() : undefined}
			auditIsLoadingMore={isFetchingOlderAudit}
			auditLoadMoreErrorMessage={
				isOlderAuditError ? "Couldn’t load older activity. Try again." : null
			}
			auditTotal={auditData?.pages[0]?.total}
		>
			{clerkInstance?.last_error ? (
				<Alert tone="critical">
					<TriangleAlert className="size-4" />
					<AlertTitle>Clerk reported a recent failure</AlertTitle>
					<AlertDescription>{clerkInstance.last_error}</AlertDescription>
				</Alert>
			) : null}

			{isLoading ? (
				<Card>
					<CardHeader>
						<Skeleton height="md" />
						<Skeleton height="sm" />
					</CardHeader>
					<CardContent>
						<Stack space="md">
							<Skeleton height="lg" />
							<Skeleton height="lg" />
							<Skeleton height="lg" />
						</Stack>
					</CardContent>
				</Card>
			) : null}

			{!isLoading && !clerkInstance ? (
				<Card variant="outline">
					<CardContent>
						<EmptyState
							icon={PlugsIcon}
							title="No Clerk instance configured"
							description="Create an instance to activate Clerk webhook ingestion and user sync."
						>
							<Button
								variant="solid"
								tone="brand"
								onClick={handleCreate}
								disabled={createMutation.isPending}
							>
								{createMutation.isPending ? (
									<Spinner />
								) : (
									<Plug className="mr-1 size-3.5" />
								)}
								Create Clerk Instance
							</Button>
							{createMutation.isError ? (
								<Text size="xs" tone="critical" role="alert">
									Failed to create instance. It may already exist.
								</Text>
							) : null}
						</EmptyState>
					</CardContent>
				</Card>
			) : null}

			{clerkInstance ? (
				<>
					<Card>
						<CardHeader>
							<CardTitle>Instance Details</CardTitle>
							<CardDescription>
								Identity sync endpoint and runtime state.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								<Spread>
									<Text tone="muted">Connection</Text>
									<Badge tone={form.enabled ? "success" : "neutral"}>
										{form.enabled ? "Live" : "Paused"}
									</Badge>
								</Spread>
								<Spread>
									<Text tone="muted">Instance ID</Text>
									<Inline space="xs" wrap={false}>
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
								<Stack space="xs">
									<Spread>
										<Text tone="muted">Webhook URL</Text>
										<CopyIconButton
											label="Copy webhook URL"
											size="sm"
											value={getWebhookUrl(
												clerkInstance.product_id,
												clerkInstance.provider_type,
											)}
										/>
									</Spread>
									<Text size="xs" tone="muted" font="mono" truncate>
										{getWebhookUrl(
											clerkInstance.product_id,
											clerkInstance.provider_type,
										)}
									</Text>
								</Stack>
							</Stack>
						</CardContent>
					</Card>

					<Card>
						<CardHeader>
							<CardTitle>Configuration</CardTitle>
							<CardDescription>
								Update runtime settings and credentials for Clerk.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								<Stack space="xs">
									<Inline space="xs" wrap={false}>
										<Shield className="size-3.5 shrink-0 text-muted-foreground" />
										<Text size="sm" weight="medium">
											Credential visibility
										</Text>
									</Inline>
									<Text size="xs" tone="muted">
										Secrets are write-only once persisted. Leave secret fields
										blank to keep existing values unchanged.
									</Text>
								</Stack>

								<Spread>
									<Stack space="xs">
										<Label htmlFor="integration-enabled">Enabled</Label>
										<Text size="xs" tone="muted" id="enabled-description">
											Turn off to pause webhook processing without deleting
											setup.
										</Text>
									</Stack>
									<Switch
										id="integration-enabled"
										aria-describedby="enabled-description"
										checked={form.enabled}
										onCheckedChange={(checked) =>
											setForm((prev) => ({ ...prev, enabled: checked }))
										}
									/>
								</Spread>

								<div className="flex flex-col gap-2">
									<Label htmlFor="webhook-secret">Webhook Secret</Label>
									<Input
										id="webhook-secret"
										type={isEditingWebhookSecret ? "password" : "text"}
										placeholder="whsec_..."
										value={
											isEditingWebhookSecret
												? form.webhookSecret
												: (clerkInstance.webhook_secret_obfuscated ?? "")
										}
										onChange={(event) =>
											setForm((prev) => ({
												...prev,
												webhookSecret: event.target.value,
											}))
										}
										onFocus={() => setIsEditingWebhookSecret(true)}
										onBlur={() => {
											if (form.webhookSecret.trim().length === 0) {
												setIsEditingWebhookSecret(false);
											}
										}}
										aria-invalid={Boolean(formErrors.webhookSecret)}
									/>
									<p className="text-[11px] text-muted-foreground">
										From Clerk dashboard webhook settings. Leave blank to keep
										unchanged.
									</p>
									{formErrors.webhookSecret ? (
										<p className="text-xs text-destructive" role="alert">
											{formErrors.webhookSecret}
										</p>
									) : null}
								</div>

								<div className="flex flex-col gap-2">
									<Label htmlFor="api-key">Clerk API Key</Label>
									<Input
										id="api-key"
										type="password"
										placeholder="sk_live_..."
										value={form.apiKey}
										onChange={(event) =>
											setForm((prev) => ({
												...prev,
												apiKey: event.target.value,
											}))
										}
										aria-invalid={Boolean(formErrors.apiKey)}
									/>
									<p className="text-[11px] text-muted-foreground">
										Optional. Used for Clerk reconciliation and backfill flows.
									</p>
									{formErrors.apiKey ? (
										<p className="text-xs text-destructive" role="alert">
											{formErrors.apiKey}
										</p>
									) : null}
								</div>

								<div className="flex flex-wrap items-center gap-2 border-t pt-4">
									<Button
										variant="solid"
										tone="brand"
										onClick={handleSave}
										disabled={isMutating || !isDirty}
									>
										{isMutating ? <Spinner /> : null}
										Update Configuration
									</Button>
									<Button
										variant="ghost"
										onClick={handleReset}
										disabled={isMutating || !isDirty}
									>
										Reset
									</Button>
								</div>

								{updateMutation.isError ? (
									<Alert tone="critical">
										<Info className="size-4" />
										<AlertTitle>Update failed</AlertTitle>
										<AlertDescription>
											Failed to update Clerk integration settings.
										</AlertDescription>
									</Alert>
								) : null}
							</Stack>
						</CardContent>
					</Card>

					<Card variant="outline">
						<CardHeader>
							<CardTitle>Danger Zone</CardTitle>
							<CardDescription>
								Delete this integration instance and stop Clerk ingestion.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								<AlertDialog>
									<AlertDialogTrigger
										render={
											<Button
												variant="outline"
												tone="critical"
												disabled={deleteMutation.isPending}
											/>
										}
									>
										{deleteMutation.isPending ? (
											<Spinner />
										) : (
											<Trash2 className="mr-2 size-4" />
										)}
										Delete Instance
									</AlertDialogTrigger>
									<AlertDialogContent>
										<AlertDialogHeader>
											<AlertDialogTitle>
												Delete Clerk integration instance?
											</AlertDialogTitle>
											<AlertDialogDescription>
												This action removes provider configuration and stops
												webhook processing for Clerk.
											</AlertDialogDescription>
										</AlertDialogHeader>
										<AlertDialogFooter>
											<AlertDialogCancel>Cancel</AlertDialogCancel>
											<AlertDialogAction tone="critical" onClick={handleDelete}>
												Delete instance
											</AlertDialogAction>
										</AlertDialogFooter>
									</AlertDialogContent>
								</AlertDialog>
							</Stack>
						</CardContent>
					</Card>
				</>
			) : null}
		</IntegrationDetailPage>
	);
}
