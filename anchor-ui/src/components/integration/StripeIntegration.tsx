import {
	IntegrationProviderType,
	StripeIntegrationAuthMethod,
	type StripeIntegrationConfigWritable,
	zStripeIntegrationConfigWritable,
	zStripeIntegrationPublicConfig,
} from "@/client";
import {
	createIntegrationInstanceMutation,
	deleteIntegrationInstanceMutation,
	listIntegrationAuditLogsOptions,
	listIntegrationInstancesOptions,
	updateIntegrationInstanceMutation,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { IntegrationDetailPage } from "@/components/integration/IntegrationDetailPage";
import { billingError } from "@/features/billing/billing-api";
import { useProduct } from "@/hooks/useProduct";
import { productIntegrationStripeRoute } from "@/routes/platform/$productId.integration-stripe";
import { ROUTE_PATHS } from "@/routes/routePaths";
import {
	Alert,
	AlertDescription,
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
	AlertTitle,
	Badge,
	Box,
	Button,
	ButtonLink,
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
	Heading,
	Inline,
	Skeleton,
	Stack,
	Text,
} from "@nanostackorg/design-system";
import { CopyIconButton } from "@nanostackorg/design-system/blocks/copy-button";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { StripeConfigForm, type StripeFormState } from "./StripeConfigForm";

const fieldNames: Record<string, keyof StripeFormState> = {
	auth_method: "authMethod",
	account_id: "accountId",
	api_key: "apiKey",
	webhook_secret: "webhookSecret",
	return_url: "returnUrl",
};
const emptyForm = (): StripeFormState => ({
	authMethod: StripeIntegrationAuthMethod.API_KEY,
	accountId: "",
	apiKey: "",
	webhookSecret: "",
	returnUrl: window.location.origin,
	enabled: true,
});

export default function StripeIntegration() {
	const { productId } = productIntegrationStripeRoute.useParams();
	return <StripeConnection key={productId} />;
}

function StripeConnection() {
	const { productId } = productIntegrationStripeRoute.useParams();
	const {
		currentProduct,
		products,
		selectProduct,
		isLoading: isProductsLoading,
	} = useProduct();
	const product =
		products.find((item) => item.id === productId) ??
		(currentProduct?.id === productId ? currentProduct : null);
	const queryClient = useQueryClient();
	const [form, setForm] = useState(emptyForm);
	const [errors, setErrors] = useState<
		Partial<Record<keyof StripeFormState, string>>
	>({});
	const [notice, setNotice] = useState("");
	const [actionError, setActionError] = useState("");
	const [saving, setSaving] = useState(false);
	const dirty = useRef(false);
	const listQuery = useQuery({
		...listIntegrationInstancesOptions({ path: { product_id: productId } }),
		enabled: !!product,
	});
	const instance = listQuery.data?.items.find(
		(item) => item.provider_type === IntegrationProviderType.STRIPE,
	);
	const parsedConfig = zStripeIntegrationPublicConfig.safeParse(
		instance?.public_config,
	);
	const publicConfig = parsedConfig.success ? parsedConfig.data : undefined;
	const auditQuery = useQuery({
		...listIntegrationAuditLogsOptions({
			path: {
				product_id: productId,
				integration_instance_id: instance?.id ?? "",
			},
		}),
		enabled: !!instance,
	});
	const createMutation = useMutation(createIntegrationInstanceMutation());
	const updateMutation = useMutation(updateIntegrationInstanceMutation());
	const deleteMutation = useMutation(deleteIntegrationInstanceMutation());
	useEffect(() => {
		if (product && currentProduct?.id !== product.id) selectProduct(product);
	}, [product, currentProduct?.id, selectProduct]);
	useEffect(() => {
		if (dirty.current) return;
		setForm({
			authMethod:
				publicConfig?.auth_method === "LOCAL_CLI"
					? StripeIntegrationAuthMethod.LOCAL_CLI
					: StripeIntegrationAuthMethod.API_KEY,
			accountId: publicConfig?.account_id ?? "",
			apiKey: "",
			webhookSecret: "",
			returnUrl: publicConfig?.return_url ?? window.location.origin,
			enabled: instance?.is_enabled ?? true,
		});
	}, [
		instance?.is_enabled,
		publicConfig?.auth_method,
		publicConfig?.account_id,
		publicConfig?.return_url,
	]);
	function setField<K extends keyof StripeFormState>(
		key: K,
		value: StripeFormState[K],
	) {
		dirty.current = true;
		setNotice("");
		setForm((previous) => ({ ...previous, [key]: value }));
	}
	async function readBack(message: string) {
		dirty.current = false;
		setForm((previous) => ({ ...previous, apiKey: "", webhookSecret: "" }));
		const result = await listQuery.refetch();
		await queryClient.invalidateQueries({
			queryKey: ["product-stripe-billing", productId],
		});
		void auditQuery.refetch();
		if (result.error)
			throw new Error(
				"The change completed, but connection status could not be refreshed. Reload to verify it.",
			);
		setNotice(message);
	}
	async function save() {
		const config: StripeIntegrationConfigWritable = {
			auth_method: form.authMethod,
			account_id: form.accountId.trim(),
			return_url: form.returnUrl.trim(),
			...(form.apiKey.trim() ? { api_key: form.apiKey.trim() } : {}),
			...(form.webhookSecret.trim()
				? { webhook_secret: form.webhookSecret.trim() }
				: {}),
		};
		const result = zStripeIntegrationConfigWritable.safeParse(config);
		const nextErrors: Partial<Record<keyof StripeFormState, string>> = {};
		if (!result.success)
			for (const issue of result.error.issues) {
				const key = fieldNames[String(issue.path[0])];
				if (key) nextErrors[key] = issue.message;
			}
		if (!config.account_id)
			nextErrors.accountId = "Enter the Stripe sandbox account ID.";
		if (!config.return_url)
			nextErrors.returnUrl = "Enter Anchor’s return address.";
		if (
			form.authMethod === StripeIntegrationAuthMethod.API_KEY &&
			!config.api_key &&
			!publicConfig?.api_key_configured
		)
			nextErrors.apiKey = "Enter a Stripe test API key.";
		if (!config.webhook_secret && !publicConfig?.webhook_secret_configured)
			nextErrors.webhookSecret = "Enter the webhook signing secret.";
		setErrors(nextErrors);
		if (Object.keys(nextErrors).length) return;
		setSaving(true);
		setActionError("");
		setNotice("");
		try {
			if (instance)
				await updateMutation.mutateAsync({
					path: { product_id: productId, integration_instance_id: instance.id },
					body: { config, is_enabled: form.enabled },
				});
			else
				await createMutation.mutateAsync({
					path: { product_id: productId },
					body: { provider_type: IntegrationProviderType.STRIPE, config },
				});
			await readBack("Stripe connection saved.");
		} catch (error) {
			setActionError(billingError(error));
		} finally {
			setSaving(false);
		}
	}
	async function remove() {
		if (!instance) return;
		setSaving(true);
		setActionError("");
		setNotice("");
		try {
			await deleteMutation.mutateAsync({
				path: { product_id: productId, integration_instance_id: instance.id },
			});
			dirty.current = false;
			setForm(emptyForm());
			await readBack("Stripe connection removed.");
		} catch (error) {
			setActionError(billingError(error));
		} finally {
			setSaving(false);
		}
	}
	const webhookUrl = `${import.meta.env.VITE_API_BASE_URL || window.location.origin}/v1/products/${productId}/billing/stripe/webhook`;
	if (!product && !isProductsLoading)
		return (
			<Page
				title="Stripe"
				description="Connect billing for a selected product."
			>
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Product unavailable</EmptyTitle>
						<EmptyDescription>
							Select a product you can manage from the top bar.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			</Page>
		);
	return (
		<IntegrationDetailPage
			title="Stripe"
			description="Connect subscriptions and payments to this product’s organization licenses."
			backLink={
				<Inline>
					<ButtonLink
						href={ROUTE_PATHS.PRODUCT_INTEGRATIONS.replace(
							"$productId",
							productId,
						)}
					>
						Integration Hub
					</ButtonLink>
					<Badge tone="warning">Stripe sandbox</Badge>
					<Text tone="muted">{product?.name ?? productId}</Text>
					<Button
						disabled={saving || listQuery.isFetching}
						onClick={() => void listQuery.refetch()}
					>
						Reload connection
					</Button>
				</Inline>
			}
			summary={[
				{
					label: "Connection",
					value: instance
						? instance.is_enabled
							? "Enabled"
							: "Paused"
						: "Not configured",
				},
				{ label: "Status", value: instance?.status ?? "Not configured" },
				{ label: "Account", value: publicConfig?.account_id ?? "—" },
				{
					label: "Last updated",
					value: instance
						? new Date(instance.updated_at).toLocaleDateString()
						: "—",
				},
			]}
			auditEntries={(auditQuery.data?.items ?? []).map((entry) => ({
				id: entry.id,
				title: entry.action.replaceAll("_", " "),
				description: entry.message,
				timestamp: entry.created_at,
				severity:
					entry.severity === "ERROR"
						? "error"
						: entry.severity === "WARNING"
							? "warning"
							: entry.severity === "SUCCESS"
								? "success"
								: "info",
			}))}
			auditIsLoading={!!instance && auditQuery.isPending}
			auditErrorMessage={
				auditQuery.error ? billingError(auditQuery.error) : null
			}
		>
			{listQuery.isPending ? (
				<Skeleton height="xxl" />
			) : (
				<Stack space="lg">
					{(listQuery.error || actionError || instance?.last_error) && (
						<Alert tone="critical">
							<AlertTitle>Stripe connection needs attention</AlertTitle>
							<AlertDescription>
								{actionError ||
									instance?.last_error ||
									billingError(listQuery.error)}
							</AlertDescription>
						</Alert>
					)}
					{notice && (
						<Alert tone="success">
							<AlertTitle>Saved</AlertTitle>
							<AlertDescription>{notice}</AlertDescription>
						</Alert>
					)}
					<Card variant="outline">
						<CardHeader>
							<Heading level={2}>
								{instance ? "Connection settings" : "Connect Stripe"}
							</Heading>
							<CardDescription>
								Credentials are encrypted and are never returned to this page.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Box
								as="form"
								noValidate
								onSubmit={(event) => {
									event.preventDefault();
									void save();
								}}
							>
								<Stack space="lg">
									<StripeConfigForm
										form={form}
										setField={setField}
										errors={errors}
										disabled={saving || !!listQuery.error}
										existing={!!instance}
										apiKeyConfigured={publicConfig?.api_key_configured ?? false}
										webhookSecretConfigured={
											publicConfig?.webhook_secret_configured ?? false
										}
									/>
									<Inline>
										<Button
											type="submit"
											variant="solid"
											tone="brand"
											loading={saving}
											disabled={saving || !!listQuery.error}
										>
											{instance ? "Save Stripe settings" : "Connect Stripe"}
										</Button>
										{instance && (
											<ButtonLink href={ROUTE_PATHS.PRODUCT_PRICING}>
												Manage pricing
											</ButtonLink>
										)}
									</Inline>
								</Stack>
							</Box>
						</CardContent>
					</Card>
					{instance && (
						<Card variant="outline">
							<CardHeader>
								<Heading level={2}>Webhook connection</Heading>
								<CardDescription>
									Configure Stripe to send checkout, subscription and invoice
									events to this address.
								</CardDescription>
							</CardHeader>
							<CardContent>
								<Stack>
									<Box className="min-w-0 break-words [overflow-wrap:anywhere]">
										<Text font="mono" size="sm">
											{webhookUrl}
										</Text>
									</Box>
									<Inline>
										<Badge
											tone={
												publicConfig?.webhook_secret_configured
													? "success"
													: "warning"
											}
										>
											{publicConfig?.webhook_secret_configured
												? "Signing secret saved"
												: "Signing secret required"}
										</Badge>
										<CopyIconButton
											value={webhookUrl}
											label="Copy Stripe webhook URL"
										/>
									</Inline>
								</Stack>
							</CardContent>
						</Card>
					)}
					{instance && (
						<Card variant="outline">
							<CardHeader>
								<Heading level={2}>Remove connection</Heading>
								<CardDescription>
									Disconnect Stripe from this product. Existing Stripe
									subscriptions remain in Stripe.
								</CardDescription>
							</CardHeader>
							<CardContent>
								<AlertDialog>
									<AlertDialogTrigger
										render={
											<Button
												tone="critical"
												variant="outline"
												disabled={saving}
											/>
										}
									>
										Remove Stripe connection
									</AlertDialogTrigger>
									<AlertDialogContent>
										<AlertDialogHeader>
											<AlertDialogTitle>
												Remove Stripe connection?
											</AlertDialogTitle>
											<AlertDialogDescription>
												Anchor will stop managing billing and processing Stripe
												events for this product until a connection is configured
												again.
											</AlertDialogDescription>
										</AlertDialogHeader>
										<AlertDialogFooter>
											<AlertDialogCancel>Keep connection</AlertDialogCancel>
											<AlertDialogAction
												tone="critical"
												onClick={() => void remove()}
											>
												Remove connection
											</AlertDialogAction>
										</AlertDialogFooter>
									</AlertDialogContent>
								</AlertDialog>
							</CardContent>
						</Card>
					)}
				</Stack>
			)}
		</IntegrationDetailPage>
	);
}
