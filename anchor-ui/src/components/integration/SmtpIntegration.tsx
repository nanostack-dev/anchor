import {
	type CreateIntegrationInstanceData,
	IntegrationProviderType,
	type ListIntegrationInstancesData,
	type Options,
	type ProductResponse,
	type SmtpIntegrationConfig,
	type UpdateIntegrationInstanceData,
	zSmtpIntegrationConfig,
} from "@/client";
import {
	createIntegrationInstanceMutation,
	deleteIntegrationInstanceMutation,
	listIntegrationInstancesOptions,
	listIntegrationInstancesQueryKey,
	updateIntegrationInstanceMutation,
} from "@/client/@tanstack/react-query.gen";
import { IntegrationDetailPage } from "@/components/integration/IntegrationDetailPage";
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
import { Stack } from "@nanostackorg/design-system/layout/stack";

import { Label } from "@nanostackorg/design-system/components/label";

import { useProduct } from "@/hooks/useProduct";
import { productIntegrationSmtpRoute } from "@/routes/platform/$productId.integration-smtp";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Switch } from "@nanostackorg/design-system/components/switch";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import dayjs from "dayjs";
import { Plug, Shield, Trash2 } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { z } from "zod";

import { SmtpConfigForm, type SmtpFormState } from "./SmtpConfigForm";

// Field name mapping: OpenAPI snake_case → form camelCase keys
const API_TO_FORM_KEY: Partial<Record<string, keyof SmtpFormState>> = {
	host: "host",
	port: "port",
	username: "username",
	password: "password",
	from_address: "fromAddress",
	from_name: "fromName",
	reply_to: "replyTo",
};

// Augment the generated schema with required-field rules (all fields are optional in the spec).
const smtpConfigSchema = zSmtpIntegrationConfig.superRefine((val, ctx) => {
	if (!val.host?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Host is required",
			path: ["host"],
		});
	}
	if (!val.port || val.port < 1 || val.port > 65535) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Port must be 1–65535",
			path: ["port"],
		});
	}
	if (!val.username?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Username is required",
			path: ["username"],
		});
	}
	if (!val.from_address?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "From address is required",
			path: ["from_address"],
		});
	}
});

const smtpConfigSchemaCreate = smtpConfigSchema.superRefine((val, ctx) => {
	if (!val.password?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Password is required for new instances",
			path: ["password"],
		});
	}
});

const EMPTY_FORM: SmtpFormState = {
	host: "",
	port: "587",
	encryption: "STARTTLS",
	authMethod: "PLAIN",
	username: "",
	password: "",
	fromAddress: "",
	fromName: "",
	replyTo: "",
	enabled: true,
};

export default function SmtpIntegrationPage() {
	const { productId } = productIntegrationSmtpRoute.useParams();
	const {
		currentProduct,
		products,
		selectProduct,
		isLoading: isProductsLoading,
	} = useProduct();
	const queryClient = useQueryClient();

	const routeProduct = useMemo(
		() => products.find((p) => p.id === productId) ?? null,
		[products, productId],
	);
	const activeProduct: ProductResponse | null =
		routeProduct ?? (currentProduct?.id === productId ? currentProduct : null);

	useEffect(() => {
		if (routeProduct && currentProduct?.id !== routeProduct.id) {
			selectProduct(routeProduct);
		}
	}, [routeProduct, currentProduct, selectProduct]);

	const [form, setForm] = useState<SmtpFormState>(EMPTY_FORM);
	const [formErrors, setFormErrors] = useState<
		Partial<Record<keyof SmtpFormState, string>>
	>({});
	// Track whether the user has touched config fields — if so, include config in the update payload.
	const configDirty = useRef(false);
	// Guard: populate form from server only once — never reset after saves/invalidations.
	const enabledInitialized = useRef(false);

	const listQueryOptions: Options<ListIntegrationInstancesData> = {
		path: { product_id: productId },
	};

	const { data: listData, isLoading } = useQuery({
		...listIntegrationInstancesOptions(listQueryOptions),
		enabled: !!productId,
	});

	const smtpInstance = useMemo(
		() =>
			(listData?.items ?? []).find(
				(i) => i.provider_type === IntegrationProviderType.SMTP,
			) ?? null,
		[listData],
	);

	// Populate form on first load from server state. Password is never returned.
	useEffect(() => {
		if (smtpInstance && !enabledInitialized.current) {
			const pc = smtpInstance.public_config;
			setForm((prev) => ({
				...prev,
				enabled: smtpInstance.is_enabled,
				...(pc
					? {
							host: pc.host ?? prev.host,
							port: pc.port != null ? String(pc.port) : prev.port,
							encryption: pc.encryption ?? prev.encryption,
							authMethod: pc.auth_method ?? prev.authMethod,
							username: pc.username ?? prev.username,
							fromAddress: pc.from_address ?? prev.fromAddress,
							fromName: pc.from_name ?? prev.fromName,
							replyTo: pc.reply_to ?? prev.replyTo,
						}
					: {}),
			}));
			enabledInitialized.current = true;
		}
		if (!smtpInstance) {
			setForm(EMPTY_FORM);
			setFormErrors({});
			enabledInitialized.current = false;
			configDirty.current = false;
		}
	}, [smtpInstance]);

	const invalidateList = () => {
		queryClient.invalidateQueries({
			queryKey: listIntegrationInstancesQueryKey(listQueryOptions),
		});
	};

	const createMutation = useMutation({
		...createIntegrationInstanceMutation(),
		onSuccess: () => {
			invalidateList();
			// Reset password only after successful create so it's not re-submitted.
			setForm((prev) => ({ ...prev, password: "" }));
			configDirty.current = false;
		},
	});

	const updateMutation = useMutation({
		...updateIntegrationInstanceMutation(),
		onSuccess: () => {
			invalidateList();
			// Reset password after save — never keep it in memory longer than needed.
			setForm((prev) => ({ ...prev, password: "" }));
			configDirty.current = false;
		},
	});

	const deleteMutation = useMutation({
		...deleteIntegrationInstanceMutation(),
		onSuccess: () => {
			invalidateList();
			setForm(EMPTY_FORM);
			enabledInitialized.current = false;
			configDirty.current = false;
		},
	});

	const isMutating =
		createMutation.isPending ||
		updateMutation.isPending ||
		deleteMutation.isPending;

	function markConfigDirty() {
		configDirty.current = true;
	}

	function setFormField<K extends keyof SmtpFormState>(
		key: K,
		value: SmtpFormState[K],
	) {
		if (key !== "enabled") markConfigDirty();
		setForm((prev) => ({ ...prev, [key]: value }));
	}

	function validate(): boolean {
		const schema = smtpInstance ? smtpConfigSchema : smtpConfigSchemaCreate;
		const result = schema.safeParse(buildConfig());
		if (result.success) {
			setFormErrors({});
			return true;
		}
		const errors: Partial<Record<keyof SmtpFormState, string>> = {};
		for (const issue of result.error.issues) {
			const formKey = API_TO_FORM_KEY[issue.path[0] as string];
			if (formKey && !errors[formKey]) errors[formKey] = issue.message;
		}
		setFormErrors(errors);
		return false;
	}

	function buildConfig(): SmtpIntegrationConfig {
		return {
			host: form.host.trim(),
			port: Number.parseInt(form.port, 10) || 587,
			encryption: form.encryption as SmtpIntegrationConfig["encryption"],
			auth_method: form.authMethod as SmtpIntegrationConfig["auth_method"],
			username: form.username.trim(),
			...(form.password.trim() ? { password: form.password.trim() } : {}),
			from_address: form.fromAddress.trim(),
			...(form.fromName.trim() ? { from_name: form.fromName.trim() } : {}),
			...(form.replyTo.trim() ? { reply_to: form.replyTo.trim() } : {}),
		};
	}

	function handleCreate() {
		if (!validate()) return;
		const opts: Options<CreateIntegrationInstanceData> = {
			path: { product_id: productId },
			body: {
				provider_type: IntegrationProviderType.SMTP,
				config: buildConfig(),
			},
		};
		createMutation.mutate(opts);
	}

	function handleUpdate() {
		if (!smtpInstance) return;
		if (!configDirty.current && !validate()) return;
		if (configDirty.current && !validate()) return;
		const body: UpdateIntegrationInstanceData["body"] = {
			is_enabled: form.enabled,
			...(configDirty.current ? { config: buildConfig() } : {}),
		};
		const opts: Options<UpdateIntegrationInstanceData> = {
			path: { product_id: productId, integration_instance_id: smtpInstance.id },
			body,
		};
		updateMutation.mutate(opts);
	}

	function handleDelete() {
		if (!smtpInstance) return;
		deleteMutation.mutate({
			path: { product_id: productId, integration_instance_id: smtpInstance.id },
		});
	}

	if (!activeProduct && !isProductsLoading) {
		return (
			<div className="p-8 text-sm text-muted-foreground">
				Product not found.
			</div>
		);
	}

	return (
		<IntegrationDetailPage
			title="SMTP Email"
			description="Configure outbound email through any SMTP provider — Proton Mail, SendGrid, Mailgun, AWS SES, or self-hosted relays."
			backLink={
				<div className="inline-flex flex-wrap items-center gap-2">
					<Badge variant="outline">
						Product {activeProduct?.name ?? productId}
					</Badge>
					<Badge variant="soft">Provider SMTP</Badge>
					{smtpInstance?.id ? (
						<Badge variant="outline">{smtpInstance.id}</Badge>
					) : null}
				</div>
			}
			summary={[
				{
					label: "Connection",
					value: smtpInstance
						? smtpInstance.is_enabled
							? "Live"
							: "Paused"
						: "NOT_CONFIGURED",
				},
				{
					label: "Status",
					value: smtpInstance?.status ?? "-",
				},
				{
					label: "Last updated",
					value: smtpInstance
						? dayjs(smtpInstance.updated_at).format("D MMMM YYYY H:mm")
						: "-",
				},
			]}
			auditEntries={[]}
			auditIsLoading={false}
			auditErrorMessage={null}
			auditTitle="Integration audit"
		>
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

			{!isLoading && !smtpInstance ? (
				<Card>
					<CardHeader>
						<CardTitle>Configure SMTP</CardTitle>
						<CardDescription>
							Enter your SMTP credentials to enable outbound email for this
							product.
						</CardDescription>
					</CardHeader>
					<CardContent>
						<Stack space="md">
							<SmtpConfigForm
								form={form}
								setField={setFormField}
								errors={formErrors}
								isNew
							/>
							<div className="flex items-center gap-2 border-t pt-4">
								<Button
									variant="solid"
									tone="brand"
									onClick={handleCreate}
									disabled={isMutating}
								>
									{createMutation.isPending ? (
										<Spinner />
									) : (
										<Plug data-icon="inline-start" className="mr-1 size-3.5" />
									)}
									Create SMTP Integration
								</Button>
							</div>
							{createMutation.isError ? (
								<p className="text-xs text-destructive">
									Failed to create integration. Check credentials and try again.
								</p>
							) : null}
						</Stack>
					</CardContent>
				</Card>
			) : null}

			{smtpInstance ? (
				<>
					<Card>
						<CardHeader>
							<CardTitle>Connection</CardTitle>
							<CardDescription>
								Runtime state for the SMTP integration.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								<div className="flex items-center justify-between gap-3 rounded-lg border bg-muted/20 p-3">
									<span className="text-muted-foreground">Status</span>
									<Badge tone={smtpInstance.is_enabled ? "success" : "neutral"}>
										{smtpInstance.is_enabled ? "Live" : "Paused"}
									</Badge>
								</div>
								<div className="flex items-center justify-between gap-2 rounded-lg border p-3">
									<span className="text-muted-foreground">Instance ID</span>
									<span className="font-mono text-xs">{smtpInstance.id}</span>
								</div>
								{smtpInstance.last_error ? (
									<div className="rounded-lg border border-destructive/30 bg-destructive/10 p-3 text-xs text-destructive">
										{smtpInstance.last_error}
									</div>
								) : null}
							</Stack>
						</CardContent>
					</Card>

					<Card>
						<CardHeader>
							<CardTitle>Configuration</CardTitle>
							<CardDescription>
								Update SMTP credentials and settings.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								<div className="rounded-lg border bg-muted/40 p-3 text-xs text-muted-foreground">
									<div className="inline-flex items-center gap-1">
										<Shield className="size-3.5" />
										Password is write-only once saved. Leave blank to keep the
										existing value.
									</div>
								</div>

								<div className="flex items-center justify-between rounded-lg border bg-muted/20 p-3">
									<div>
										<Label htmlFor="smtp-enabled">Enabled</Label>
										<p className="text-xs text-muted-foreground">
											Pause without deleting configuration.
										</p>
									</div>
									<Switch
										id="smtp-enabled"
										checked={form.enabled}
										onCheckedChange={(checked) =>
											setForm((prev) => ({ ...prev, enabled: checked }))
										}
									/>
								</div>

								<SmtpConfigForm
									form={form}
									setField={setFormField}
									errors={formErrors}
									isNew={false}
								/>

								<div className="flex items-center gap-2 border-t pt-4">
									<Button
										variant="solid"
										tone="brand"
										onClick={handleUpdate}
										disabled={isMutating}
									>
										{updateMutation.isPending ? <Spinner /> : null}
										Update Configuration
									</Button>
								</div>
								{updateMutation.isError ? (
									<p className="text-xs text-destructive">Update failed.</p>
								) : null}
								{updateMutation.isSuccess ? (
									<p className="text-xs text-success">Configuration updated.</p>
								) : null}
							</Stack>
						</CardContent>
					</Card>

					<Card variant="outline">
						<CardHeader>
							<CardTitle>Danger Zone</CardTitle>
							<CardDescription>Delete this SMTP integration.</CardDescription>
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
											<Spinner data-icon="inline-start" />
										) : (
											<Trash2
												data-icon="inline-start"
												className="mr-2 size-4"
											/>
										)}
										Delete Integration
									</AlertDialogTrigger>
									<AlertDialogContent>
										<AlertDialogHeader>
											<AlertDialogTitle>
												Delete SMTP integration?
											</AlertDialogTitle>
											<AlertDialogDescription>
												This removes the SMTP configuration. Emails cannot be
												sent until a new integration is created.
											</AlertDialogDescription>
										</AlertDialogHeader>
										<AlertDialogFooter>
											<AlertDialogCancel>Cancel</AlertDialogCancel>
											<AlertDialogAction tone="critical" onClick={handleDelete}>
												Delete
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
