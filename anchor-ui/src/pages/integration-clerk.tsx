import {
	type ClerkIntegrationConfig,
	type CreateIntegrationInstanceData,
	type DeleteIntegrationInstanceData,
	type IntegrationAuditLogEntryResponse,
	IntegrationAuditLogSeverity,
	type IntegrationInstanceResponse,
	IntegrationProviderType,
	type ListIntegrationAuditLogsData,
	type ListIntegrationInstancesData,
	type Options,
	type ProductResponse,
	type UpdateIntegrationInstanceData,
} from "@/client";
import {
	createIntegrationInstanceMutation,
	deleteIntegrationInstanceMutation,
	listIntegrationAuditLogsOptions,
	listIntegrationAuditLogsQueryKey,
	listIntegrationInstancesOptions,
	listIntegrationInstancesQueryKey,
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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { useProduct } from "@/hooks/useProduct";
import { productIntegrationClerkRoute } from "@/routes/platform/$productId.integration-clerk";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import dayjs from "dayjs";
import {
	Check,
	Copy,
	Info,
	Plug,
	Shield,
	Trash2,
	TriangleAlert,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

interface IntegrationFormState {
	webhookSecret: string;
	apiKey: string;
	enabled: boolean;
}

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

	const auditQueryOptions: Options<ListIntegrationAuditLogsData> = {
		path: {
			product_id: productId,
			integration_instance_id: clerkInstance?.id ?? "",
		},
	};

	const {
		data: auditData,
		isLoading: isAuditLoading,
		error: auditError,
	} = useQuery({
		...listIntegrationAuditLogsOptions(auditQueryOptions),
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
				queryKey: listIntegrationAuditLogsQueryKey(auditQueryOptions),
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
		return (auditData?.items ?? []).map(mapAuditEntry);
	}, [auditData]);

	if (!activeProduct && !isProductsLoading) {
		return (
			<Page
				title="Clerk Integration"
				description="Configure identity integration instances for a specific product"
			>
				<Card className="border-dashed">
					<CardContent className="flex min-h-52 items-center justify-center text-center">
						<p className="max-w-sm text-sm text-muted-foreground">
							Product not found or unavailable.
						</p>
					</CardContent>
				</Card>
			</Page>
		);
	}

	return (
		<IntegrationDetailPage
			title="Clerk Authentication"
			description="Configure webhook security, sync controls, and runtime behavior for Clerk identity events."
			backLink={
				<div className="inline-flex flex-wrap items-center gap-2">
					<Badge variant="outline" className="font-normal">
						Product {activeProduct?.name ?? productId}
					</Badge>
					<Badge variant="secondary" className="font-normal">
						Provider Clerk
					</Badge>
					{clerkInstance?.id ? (
						<Badge variant="outline" className="font-mono text-[10px]">
							{clerkInstance.id}
						</Badge>
					) : null}
				</div>
			}
			summary={[
				{
					label: "Connection",
					value: clerkInstance
						? clerkInstance.is_enabled
							? "Live"
							: "Paused"
						: "NOT_CONFIGURED",
				},
				{
					label: "Webhook",
					value: clerkInstance ? "Ready" : "Not set",
				},
				{
					label: "Activity",
					value: auditData?.count ?? 0,
				},
				{
					label: "Last updated",
					value: clerkInstance
						? dayjs(clerkInstance.updated_at).format("D MMMM YYYY H:mm")
						: "-",
				},
			]}
			auditEntries={auditEntries}
			auditIsLoading={Boolean(clerkInstance) && isAuditLoading}
			auditErrorMessage={
				auditError ? "Failed to load integration audit activity." : null
			}
			auditTitle="Integration audit"
		>
			{clerkInstance?.last_error ? (
				<Alert variant="destructive">
					<TriangleAlert className="size-4" />
					<AlertTitle>Clerk reported a recent failure</AlertTitle>
					<AlertDescription>{clerkInstance.last_error}</AlertDescription>
				</Alert>
			) : null}

			{isLoading ? (
				<Card>
					<CardHeader>
						<Skeleton className="h-6 w-44" />
						<Skeleton className="h-4 w-72" />
					</CardHeader>
					<CardContent className="flex flex-col gap-3">
						<Skeleton className="h-10 w-full" />
						<Skeleton className="h-10 w-full" />
						<Skeleton className="h-10 w-full" />
					</CardContent>
				</Card>
			) : null}

			{!isLoading && !clerkInstance ? (
				<Card className="border-dashed">
					<CardContent className="flex min-h-64 flex-col items-center justify-center text-center">
						<div className="rounded-full border bg-muted/40 p-3">
							<Plug className="size-6 text-muted-foreground" />
						</div>
						<p className="mt-4 text-base font-semibold">
							No Clerk instance configured
						</p>
						<p className="mt-2 max-w-md text-sm text-muted-foreground">
							Create an instance to activate Clerk webhook ingestion and user
							sync.
						</p>
						<Button
							className="mt-5"
							onClick={handleCreate}
							disabled={createMutation.isPending}
						>
							{createMutation.isPending ? (
								<Spinner className="mr-2 size-4 text-current" />
							) : (
								<Plug className="mr-1 size-3.5" />
							)}
							Create Clerk Instance
						</Button>
						{createMutation.isError ? (
							<p className="mt-3 text-xs text-destructive" role="alert">
								Failed to create instance. It may already exist.
							</p>
						) : null}
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
						<CardContent className="flex flex-col gap-3 text-sm">
							<div className="flex items-center justify-between gap-3 rounded-lg border bg-muted/20 p-3">
								<span className="text-muted-foreground">Connection</span>
								<Badge variant={form.enabled ? "default" : "secondary"}>
									{form.enabled ? "Live" : "Paused"}
								</Badge>
							</div>
							<div className="flex items-center justify-between gap-2 rounded-lg border p-3">
								<span className="text-muted-foreground">Instance ID</span>
								<div className="flex items-center gap-1">
									<span className="max-w-[200px] truncate font-mono text-xs">
										{clerkInstance.id}
									</span>
									<CopyButton value={clerkInstance.id} />
								</div>
							</div>
							<div className="rounded-lg border p-3">
								<div className="mb-2 flex items-center justify-between">
									<span className="text-muted-foreground">Webhook URL</span>
									<CopyButton
										value={getWebhookUrl(
											clerkInstance.product_id,
											clerkInstance.provider_type,
										)}
									/>
								</div>
								<p className="truncate font-mono text-xs text-muted-foreground">
									{getWebhookUrl(
										clerkInstance.product_id,
										clerkInstance.provider_type,
									)}
								</p>
							</div>
						</CardContent>
					</Card>

					<Card>
						<CardHeader>
							<CardTitle>Configuration</CardTitle>
							<CardDescription>
								Update runtime settings and credentials for Clerk.
							</CardDescription>
						</CardHeader>
						<CardContent className="flex flex-col gap-5">
							<div className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground">
								<div className="inline-flex items-center gap-1">
									<Shield className="size-3.5" />
									Credential visibility
								</div>
								<p className="mt-2 text-[11px] leading-5">
									Secrets are write-only once persisted. Leave secret fields
									blank to keep existing values unchanged.
								</p>
							</div>

							<div className="flex items-center justify-between rounded-lg border bg-muted/20 p-3">
								<div>
									<Label htmlFor="integration-enabled">Enabled</Label>
									<p
										className="text-xs text-muted-foreground"
										id="enabled-description"
									>
										Turn off to pause webhook processing without deleting setup.
									</p>
								</div>
								<Switch
									id="integration-enabled"
									aria-describedby="enabled-description"
									checked={form.enabled}
									onCheckedChange={(checked) =>
										setForm((prev) => ({ ...prev, enabled: checked }))
									}
								/>
							</div>

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
								<Button onClick={handleSave} disabled={isMutating || !isDirty}>
									{isMutating ? (
										<Spinner className="mr-2 size-4 text-current" />
									) : null}
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
								<Alert variant="destructive">
									<Info className="size-4" />
									<AlertTitle>Update failed</AlertTitle>
									<AlertDescription>
										Failed to update Clerk integration settings.
									</AlertDescription>
								</Alert>
							) : null}
						</CardContent>
					</Card>

					<Card className="border-destructive/30">
						<CardHeader>
							<CardTitle className="text-destructive">Danger Zone</CardTitle>
							<CardDescription>
								Delete this integration instance and stop Clerk ingestion.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<AlertDialog>
								<AlertDialogTrigger asChild>
									<Button
										variant="outlineDestructive"
										disabled={deleteMutation.isPending}
									>
										{deleteMutation.isPending ? (
											<Spinner className="mr-2 size-4 text-current" />
										) : (
											<Trash2 className="mr-2 size-4" />
										)}
										Delete Instance
									</Button>
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
										<AlertDialogAction onClick={handleDelete}>
											Delete instance
										</AlertDialogAction>
									</AlertDialogFooter>
								</AlertDialogContent>
							</AlertDialog>
						</CardContent>
					</Card>
				</>
			) : null}
		</IntegrationDetailPage>
	);
}
