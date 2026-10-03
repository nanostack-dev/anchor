import type {
	ProductEventDefinitionResponse,
	ProductRequest,
	ProductResponse,
} from "@/client";
import {
	getProductEventsCatalogOptions,
	getProductQueryKey,
	updateProductMutation,
} from "@/client/@tanstack/react-query.gen";
import { zProductEventsConfigRequest } from "@/client/zod.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { getApiErrorMessage } from "@/lib/api-error";
import { cn } from "@/lib/utils";
import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	Field,
	FieldDescription,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useForm } from "@tanstack/react-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
	Award,
	Building2,
	Check,
	CheckCircle2,
	Copy,
	FolderKanban,
	Globe,
	KeyRound,
	Layers,
	Plug,
	Radio,
	RotateCcw,
	Search,
	ShieldCheck,
	Users,
	Webhook,
} from "lucide-react";
import * as React from "react";
import { z } from "zod";

const eventsFormSchema = z.object({
	eventsEndpointUrl: zProductEventsConfigRequest.shape.endpoint_url.unwrap(),
	events: zProductEventsConfigRequest.shape.events.unwrap(),
});

type EventsFormData = z.infer<typeof eventsFormSchema>;

interface EventGroup {
	type: "internal" | "integration";
	name: string;
	events: ProductEventDefinitionResponse[];
}

interface ProductEventsFormProps {
	product: ProductResponse;
	onSaved?: () => void;
}

function formValues(
	product: ProductResponse,
	items: ProductEventDefinitionResponse[] = [],
): EventsFormData {
	return {
		eventsEndpointUrl: product.config.events?.endpoint_url ?? "",
		events: product.config.events?.events ?? items.map((item) => item.type),
	};
}

function getGroupIcon(name: string, type: "internal" | "integration") {
	if (type === "integration") {
		return <Plug className="size-4 text-primary" />;
	}
	const normalized = name.toLowerCase();
	if (normalized.includes("organization")) {
		return <Building2 className="size-4 text-primary" />;
	}
	if (normalized.includes("workspace")) {
		return <FolderKanban className="size-4 text-primary" />;
	}
	if (normalized.includes("key")) {
		return <KeyRound className="size-4 text-primary" />;
	}
	if (normalized.includes("user")) {
		return <Users className="size-4 text-primary" />;
	}
	if (normalized.includes("role") || normalized.includes("permission")) {
		return <ShieldCheck className="size-4 text-primary" />;
	}
	if (normalized.includes("licens")) {
		return <Award className="size-4 text-primary" />;
	}
	return <Layers className="size-4 text-primary" />;
}

export function ProductEventsForm({
	product,
	onSaved,
}: ProductEventsFormProps) {
	const queryClient = useQueryClient();
	const [revealedSecret, setRevealedSecret] = React.useState<string | null>(
		null,
	);
	const [secretCopied, setSecretCopied] = React.useState(false);
	const [searchQuery, setSearchQuery] = React.useState("");
	const [filterType, setFilterType] = React.useState<
		"all" | "internal" | "integration"
	>("all");
	const prevProductRef = React.useRef(product);
	const eventsInitializedRef = React.useRef(false);

	const hadEvents = Boolean(product.config.events?.endpoint_url);

	const catalogQuery = useQuery({
		...getProductEventsCatalogOptions({
			path: { product_id: product.id },
		}),
	});

	const form = useForm({
		defaultValues: formValues(product, catalogQuery.data?.items),
		onSubmit: async ({ value }) => {
			const result = eventsFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(result.data);
		},
		validators: {
			onChange: eventsFormSchema,
			onSubmit: eventsFormSchema,
		},
	});

	const updateMutation = useMutation({
		...updateProductMutation(),
		onSuccess: (updated) => {
			form.reset(formValues(updated, catalogQuery.data?.items));
			void queryClient.invalidateQueries({
				queryKey: getProductQueryKey({
					path: { product_id: product.id },
				}),
			});
			onSaved?.();
			const generatedSecret = updated.config.events?.signing_secret;
			if (generatedSecret) {
				setRevealedSecret(generatedSecret);
				toast.add({
					type: "success",
					title: "Store the event signing secret now. It is not shown again.",
				});
				return;
			}
			if (updated.config.events?.endpoint_url) {
				toast.add({ type: "success", title: "Event endpoint saved." });
				return;
			}
			toast.add({ type: "success", title: "Event endpoint cleared." });
		},
		onError: (error) => {
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: "Failed to save the event endpoint. Please try again.",
				});
			}
		},
	});

	const onSubmit = async (values: EventsFormData) => {
		const endpointUrl = values.eventsEndpointUrl.trim();
		if (!endpointUrl && !hadEvents) {
			return;
		}
		if (endpointUrl && !catalogQuery.data) {
			toast.add({
				type: "error",
				title: "Load the event catalog before saving this endpoint.",
			});
			return;
		}
		const updateData: ProductRequest = {
			name: product.name,
			config: {
				organization_api_keys: {
					prefix: product.config.organization_api_keys.prefix,
				},
				events: {
					endpoint_url: endpointUrl,
					events: endpointUrl ? values.events : [],
				},
			},
		};

		await updateMutation.mutateAsync({
			path: { product_id: product.id },
			body: updateData,
		});
	};

	React.useEffect(() => {
		const previousProduct = prevProductRef.current;
		prevProductRef.current = product;
		if (!eventsInitializedRef.current && catalogQuery.data?.items) {
			eventsInitializedRef.current = true;
			if (form.state.isDirty && !product.config.events) {
				form.setFieldValue(
					"events",
					catalogQuery.data.items.map((item) => item.type),
				);
			} else if (!form.state.isDirty) {
				form.reset(formValues(product, catalogQuery.data.items));
			}
			return;
		}
		if (previousProduct !== product && !form.state.isDirty) {
			form.reset(formValues(product, catalogQuery.data?.items));
		}
	}, [product, catalogQuery.data?.items, form]);

	const groups = React.useMemo<EventGroup[]>(() => {
		const items = catalogQuery.data?.items ?? [];
		const map = new Map<string, EventGroup>();

		for (const item of items) {
			const key = `${item.group_type}:${item.group_name}`;
			let group = map.get(key);
			if (!group) {
				group = {
					type: item.group_type,
					name: item.group_name,
					events: [],
				};
				map.set(key, group);
			}
			group.events.push(item);
		}

		return Array.from(map.values()).sort((a, b) => {
			if (a.type !== b.type) {
				return a.type === "internal" ? -1 : 1;
			}
			return a.name.localeCompare(b.name);
		});
	}, [catalogQuery.data?.items]);

	const totalCatalogEvents = catalogQuery.data?.items?.length ?? 0;
	const internalEventsCount = React.useMemo(
		() =>
			(catalogQuery.data?.items ?? []).filter(
				(item) => item.group_type === "internal",
			).length,
		[catalogQuery.data?.items],
	);
	const integrationEventsCount = React.useMemo(
		() =>
			(catalogQuery.data?.items ?? []).filter(
				(item) => item.group_type === "integration",
			).length,
		[catalogQuery.data?.items],
	);

	const filteredGroups = React.useMemo(() => {
		const query = searchQuery.trim().toLowerCase();
		return groups
			.filter((group) => {
				if (filterType === "all") return true;
				return group.type === filterType;
			})
			.map((group) => {
				if (!query) return group;
				const matchingEvents = group.events.filter(
					(e) =>
						e.name.toLowerCase().includes(query) ||
						e.type.toLowerCase().includes(query) ||
						e.description.toLowerCase().includes(query),
				);
				return {
					...group,
					events: matchingEvents,
				};
			})
			.filter((group) => group.events.length > 0);
	}, [groups, filterType, searchQuery]);

	const handleReset = () => {
		form.reset(formValues(product, catalogQuery.data?.items));
	};

	const endpointValue = form.state.values.eventsEndpointUrl;
	const isEndpointConfigured = Boolean(endpointValue?.trim());
	const deliveryStatus = product.config.events?.delivery_status;
	const failedCalls = product.config.events?.consecutive_failed_calls ?? 0;

	return (
		<Box
			as="form"
			onSubmit={(e) => {
				e.preventDefault();
				e.stopPropagation();
				form.handleSubmit();
			}}
			className="flex w-full flex-col gap-6 font-sans antialiased"
		>
			<form.Field name="events">
				{(eventsField) => {
					const selectedEvents = eventsField.state.value ?? [];
					const isAllSelected =
						totalCatalogEvents > 0 &&
						totalCatalogEvents === selectedEvents.length;

					const toggleEvent = (eventType: string) => {
						const next = selectedEvents.includes(eventType)
							? selectedEvents.filter((t) => t !== eventType)
							: [...selectedEvents, eventType];
						eventsField.handleChange(next);
					};

					const toggleGroup = (group: EventGroup) => {
						const groupTypes = group.events.map((e) => e.type);
						const allInGroupSelected = groupTypes.every((t) =>
							selectedEvents.includes(t),
						);
						const next = allInGroupSelected
							? selectedEvents.filter((t) => !groupTypes.includes(t))
							: Array.from(new Set([...selectedEvents, ...groupTypes]));
						eventsField.handleChange(next);
					};

					const selectAll = () => {
						const allTypes =
							catalogQuery.data?.items?.map((item) => item.type) ?? [];
						eventsField.handleChange(allTypes);
					};

					const deselectAll = () => {
						eventsField.handleChange([]);
					};

					return (
						<Box className="flex flex-col gap-6">
							<Card>
								<CardHeader>
									<Box className="flex items-center justify-between">
										<Box className="space-y-1">
											<CardTitle>
												<Webhook className="size-4 text-primary" />
												Endpoint Configuration
											</CardTitle>
											<CardDescription>
												Where Anchor delivers signed JSON payloads when product
												events occur.
											</CardDescription>
										</Box>
										<Badge variant={isEndpointConfigured ? "outline" : "soft"}>
											{isEndpointConfigured ? "Configured" : "Unset"}
										</Badge>
									</Box>
								</CardHeader>
								<CardContent>
									{hadEvents && deliveryStatus ? (
										<Alert
											tone={
												deliveryStatus === "failed" ? "critical" : "neutral"
											}
											role={deliveryStatus === "failed" ? "alert" : "status"}
										>
											<AlertTitle>Delivery status</AlertTitle>
											<AlertDescription>
												{deliveryStatus === "failed"
													? `Last call failed · ${failedCalls} consecutive failed ${failedCalls === 1 ? "call" : "calls"}. Anchor attempts each event up to six times.`
													: deliveryStatus === "succeeded"
														? "Last call succeeded."
														: "No delivery attempts yet."}
											</AlertDescription>
										</Alert>
									) : null}
									{revealedSecret ? (
										<Alert tone="warning">
											<KeyRound className="size-4" />
											<AlertTitle>New Signing Secret Minted</AlertTitle>
											<AlertDescription>
												<p>
													Store this secret in your webhook handler. It
													validates payload signatures in the{" "}
													<code className="font-mono font-semibold">
														webhook-signature
													</code>{" "}
													header. For security, it cannot be revealed again.
												</p>
												<Box className="flex items-center gap-2 rounded-xl border border-border bg-background p-2.5 shadow-2xs">
													<code className="flex-1 truncate font-mono text-xs text-foreground">
														{revealedSecret}
													</code>
													<Button
														type="button"
														variant="outline"
														size="sm"
														onClick={() => {
															void navigator.clipboard.writeText(
																revealedSecret,
															);
															setSecretCopied(true);
															window.setTimeout(
																() => setSecretCopied(false),
																1500,
															);
														}}
													>
														{secretCopied ? (
															<>
																<Check className="mr-1 size-3 text-success" />{" "}
																Copied
															</>
														) : (
															<>
																<Copy className="mr-1 size-3" /> Copy Secret
															</>
														)}
													</Button>
												</Box>
											</AlertDescription>
										</Alert>
									) : null}

									<FieldGroup>
										<form.Field name="eventsEndpointUrl">
											{(urlField) => (
												<Field
													data-disabled={updateMutation.isPending}
													data-invalid={urlField.state.meta.errors.length > 0}
												>
													<FieldLabel htmlFor="events-endpoint-url">
														Event endpoint URL
													</FieldLabel>
													<Box className="relative mt-1">
														<Globe className="absolute top-2.5 left-3 size-4 text-muted-foreground/70" />
														<Input
															font="mono"
															id="events-endpoint-url"
															placeholder="https://api.yourdomain.com/webhooks/anchor"
															value={urlField.state.value}
															onChange={(e) =>
																urlField.handleChange(e.target.value)
															}
															onBlur={urlField.handleBlur}
															disabled={updateMutation.isPending}
															aria-invalid={
																urlField.state.meta.errors.length > 0
															}
														/>
													</Box>
													<FieldDescription>
														Anchor POSTs signed catalog events here. Leave empty
														to clear the endpoint. Production requires HTTPS.
														Anchor mints the signing secret on first save.
														{product.config.events?.signing_secret_obfuscated
															? ` Stored secret: ${product.config.events.signing_secret_obfuscated}.`
															: ""}
													</FieldDescription>
													<FormValidationError field={urlField} />
												</Field>
											)}
										</form.Field>
									</FieldGroup>
								</CardContent>
							</Card>
							<Card>
								<CardHeader>
									<Box className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
										<Box className="space-y-1">
											<CardTitle>
												<Layers className="size-4 text-primary" />
												Event Subscriptions
											</CardTitle>
											<CardDescription>
												Select the internal and integration events Anchor
												delivers to your endpoint.
											</CardDescription>
										</Box>
										<Box className="flex items-center gap-2">
											<Badge tone="neutral" variant="soft">
												{selectedEvents.length} selected
											</Badge>
											<Button
												type="button"
												variant="outline"
												size="sm"
												onClick={isAllSelected ? deselectAll : selectAll}
												disabled={totalCatalogEvents === 0}
											>
												{isAllSelected ? "Deselect all" : "Select all"}
											</Button>
										</Box>
									</Box>
								</CardHeader>
								<CardContent>
									<Box className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
										<Box className="relative max-w-sm flex-1">
											<Search className="absolute top-2.5 left-3 size-4 text-muted-foreground/70" />
											<Input
												placeholder="Filter events by name, code, or description..."
												value={searchQuery}
												onChange={(e) => setSearchQuery(e.target.value)}
											/>
										</Box>
										<Box className="inline-flex rounded-xl border border-border/50 bg-muted/50 p-1 shadow-2xs backdrop-blur-xs">
											<button
												type="button"
												onClick={() => setFilterType("all")}
												className={cn(
													"rounded-lg px-3 py-1 text-xs font-medium transition-all active:scale-[0.98]",
													filterType === "all"
														? "bg-background text-foreground shadow-xs font-semibold"
														: "text-muted-foreground hover:text-foreground",
												)}
											>
												All ({totalCatalogEvents})
											</button>
											<button
												type="button"
												onClick={() => setFilterType("internal")}
												className={cn(
													"rounded-lg px-3 py-1 text-xs font-medium transition-all active:scale-[0.98]",
													filterType === "internal"
														? "bg-background text-foreground shadow-xs font-semibold"
														: "text-muted-foreground hover:text-foreground",
												)}
											>
												Internal ({internalEventsCount})
											</button>
											<button
												type="button"
												onClick={() => setFilterType("integration")}
												className={cn(
													"rounded-lg px-3 py-1 text-xs font-medium transition-all active:scale-[0.98]",
													filterType === "integration"
														? "bg-background text-foreground shadow-xs font-semibold"
														: "text-muted-foreground hover:text-foreground",
												)}
											>
												Integrations ({integrationEventsCount})
											</button>
										</Box>
									</Box>

									{catalogQuery.isLoading ? (
										<Box className="flex items-center justify-center py-12">
											<Spinner />
											<Box
												as="span"
												className="ml-2 text-sm text-muted-foreground"
											>
												Loading event catalog...
											</Box>
										</Box>
									) : catalogQuery.isError && !catalogQuery.data ? (
										<Alert tone="critical">
											<AlertTitle>Event catalog unavailable</AlertTitle>
											<AlertDescription>
												<p>Load the catalog before saving an endpoint.</p>
												<Button
													type="button"
													variant="outline"
													onClick={() => void catalogQuery.refetch()}
												>
													Retry
												</Button>
											</AlertDescription>
										</Alert>
									) : filteredGroups.length === 0 ? (
										<Box className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border/60 py-12 text-center text-sm text-muted-foreground">
											<Radio className="mb-2.5 size-7 text-muted-foreground/40" />
											<Box
												as="p"
												className="font-semibold tracking-tight text-foreground"
											>
												No events found
											</Box>
											<Box as="p" className="text-xs text-muted-foreground">
												No catalog events match "{searchQuery}".
											</Box>
										</Box>
									) : (
										<Box className="space-y-4">
											{filteredGroups.map((group) => {
												const groupTypes = group.events.map((e) => e.type);
												const selectedInGroup = groupTypes.filter((t) =>
													selectedEvents.includes(t),
												);
												const allGroupSelected =
													selectedInGroup.length === groupTypes.length &&
													groupTypes.length > 0;

												return (
													<Box
														key={`${group.type}:${group.name}`}
														className="overflow-hidden rounded-2xl border border-border/50 bg-card/60 shadow-2xs transition-all duration-200 hover:border-border/80"
													>
														<Box className="flex items-center justify-between border-b border-border/40 bg-muted/20 px-4 py-2.5">
															<Box className="flex items-center gap-2">
																{getGroupIcon(group.name, group.type)}
																<Box
																	as="span"
																	className="text-sm font-semibold tracking-tight text-foreground"
																>
																	{group.name}
																</Box>
															</Box>
															<Box className="flex items-center gap-3">
																<Box
																	as="span"
																	className="font-mono text-xs text-muted-foreground"
																>
																	{selectedInGroup.length}/{group.events.length}
																</Box>
																<Button
																	type="button"
																	variant="ghost"
																	size="sm"
																	onClick={() => toggleGroup(group)}
																>
																	{allGroupSelected ? "Deselect" : "Select all"}
																</Button>
															</Box>
														</Box>
														<Box className="divide-y divide-border/30">
															{group.events.map((event) => {
																const isSelected = selectedEvents.includes(
																	event.type,
																);
																return (
																	<label
																		key={event.type}
																		htmlFor={`event-checkbox-${event.type}`}
																		className={cn(
																			"flex cursor-pointer items-start gap-3.5 p-3.5 transition-all active:scale-[0.998] hover:bg-muted/30",
																			isSelected && "bg-primary/[0.02]",
																		)}
																	>
																		<Checkbox
																			id={`event-checkbox-${event.type}`}
																			checked={isSelected}
																			onCheckedChange={() =>
																				toggleEvent(event.type)
																			}
																		/>
																		<Box className="min-w-0 flex-1">
																			<Box className="flex flex-wrap items-center gap-2">
																				<Box
																					as="span"
																					className="text-sm font-medium tracking-tight text-foreground"
																				>
																					{event.name}
																				</Box>
																				<code className="rounded-md border border-border/50 bg-muted/60 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
																					{event.type}
																				</code>
																			</Box>
																			<Box
																				as="p"
																				className="mt-1 text-xs leading-normal text-muted-foreground"
																			>
																				{event.description}
																			</Box>
																		</Box>
																	</label>
																);
															})}
														</Box>
													</Box>
												);
											})}
										</Box>
									)}
								</CardContent>
							</Card>
							<Box className="sticky bottom-4 z-20 flex items-center justify-between rounded-2xl border border-border/70 bg-card/85 p-4 shadow-xl backdrop-blur-xl">
								<Box className="text-xs">
									<form.Subscribe selector={(state) => [state.isDirty]}>
										{([isDirty]) =>
											isDirty ? (
												<Box
													as="span"
													className="inline-flex items-center gap-2 font-medium text-warning-on-tint"
												>
													<Box
														as="span"
														className="size-2 animate-pulse rounded-full bg-warning"
													/>
													Unsaved changes
												</Box>
											) : (
												<Box
													as="span"
													className="inline-flex items-center gap-2 text-muted-foreground"
												>
													<CheckCircle2 className="size-3.5 text-success" />
													Endpoint and subscriptions saved
												</Box>
											)
										}
									</form.Subscribe>
								</Box>
								<Box className="flex items-center gap-2.5">
									<form.Subscribe selector={(state) => [state.isDirty]}>
										{([isDirty]) => (
											<Button
												type="button"
												variant="outline"
												size="sm"
												disabled={!isDirty || updateMutation.isPending}
												onClick={handleReset}
											>
												<RotateCcw className="mr-1.5 size-3.5" />
												Discard
											</Button>
										)}
									</form.Subscribe>
									<form.Subscribe
										selector={(state) =>
											[
												state.canSubmit,
												state.isSubmitting,
												state.isDirty,
												state.isValidating,
												state.isValid,
												state.values.eventsEndpointUrl,
											] as const
										}
									>
										{([
											canSubmit,
											isSubmitting,
											isDirty,
											isValidating,
											isValid,
											endpointUrl,
										]) => (
											<Button
												variant="solid"
												tone="brand"
												type="submit"
												size="sm"
												disabled={
													!canSubmit ||
													isSubmitting ||
													!isValid ||
													isValidating ||
													!isDirty ||
													(Boolean(endpointUrl.trim()) && !catalogQuery.data) ||
													updateMutation.isPending
												}
											>
												{updateMutation.isPending || isSubmitting ? (
													<>
														<Spinner data-icon="inline-start" />
														Saving...
													</>
												) : (
													"Save endpoint"
												)}
											</Button>
										)}
									</form.Subscribe>
								</Box>
							</Box>
						</Box>
					);
				}}
			</form.Field>
		</Box>
	);
}
