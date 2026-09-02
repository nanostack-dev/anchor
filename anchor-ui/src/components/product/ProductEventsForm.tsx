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
import { FormValidationError } from "@/components/common/FormValidationError";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
	Field,
	FieldDescription,
	FieldGroup,
	FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { cn } from "@/lib/utils";
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
import { toast } from "sonner";
import { z } from "zod";

const eventsFormSchema = z.object({
	eventsEndpointUrl: z.string(),
	events: z.array(z.string()),
});

type EventsFormData = z.infer<typeof eventsFormSchema>;

interface EventGroup {
	type: "theme" | "integration";
	name: string;
	events: ProductEventDefinitionResponse[];
}

interface ProductEventsFormProps {
	product: ProductResponse;
	productId: string;
	onSaved?: () => void;
}

function getGroupIcon(name: string, type: "theme" | "integration") {
	if (type === "integration") {
		return <Plug className="size-4 text-sky-500" />;
	}
	const normalized = name.toLowerCase();
	if (normalized.includes("organization")) {
		return <Building2 className="size-4 text-blue-500" />;
	}
	if (normalized.includes("workspace")) {
		return <FolderKanban className="size-4 text-indigo-500" />;
	}
	if (normalized.includes("key")) {
		return <KeyRound className="size-4 text-amber-500" />;
	}
	if (normalized.includes("user")) {
		return <Users className="size-4 text-purple-500" />;
	}
	if (normalized.includes("role") || normalized.includes("permission")) {
		return <ShieldCheck className="size-4 text-emerald-500" />;
	}
	if (normalized.includes("licens")) {
		return <Award className="size-4 text-teal-500" />;
	}
	return <Layers className="size-4 text-primary" />;
}

export function ProductEventsForm({
	product,
	productId,
	onSaved,
}: ProductEventsFormProps) {
	const queryClient = useQueryClient();
	const [revealedSecret, setRevealedSecret] = React.useState<string | null>(
		null,
	);
	const [secretCopied, setSecretCopied] = React.useState(false);
	const [searchQuery, setSearchQuery] = React.useState("");
	const [filterType, setFilterType] = React.useState<
		"all" | "theme" | "integration"
	>("all");

	const hadEvents = Boolean(product.config.events?.endpoint_url);

	const catalogQuery = useQuery({
		...getProductEventsCatalogOptions({
			path: { product_id: productId },
		}),
	});

	const form = useForm({
		defaultValues: {
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
			events:
				product.config.events?.events ??
				catalogQuery.data?.items?.map((item) => item.type) ??
				[],
		} as EventsFormData,
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
			void queryClient.invalidateQueries({
				queryKey: getProductQueryKey({
					path: { product_id: productId },
				}),
			});
			onSaved?.();
			const generatedSecret = updated.config.events?.signing_secret;
			if (generatedSecret) {
				setRevealedSecret(generatedSecret);
				toast.success(
					"Store the event signing secret now. It is not shown again.",
				);
				return;
			}
			if (updated.config.events?.endpoint_url) {
				toast.success("Event endpoint saved.");
				return;
			}
			toast.success("Event endpoint cleared.");
		},
		onError: (error) => {
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to save the event endpoint. Please try again.");
			}
		},
	});

	const onSubmit = async (values: EventsFormData) => {
		const endpointUrl = values.eventsEndpointUrl.trim();
		if (!endpointUrl && !hadEvents) {
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
			path: { product_id: productId },
			body: updateData,
		});
	};

	const prevProductRef = React.useRef(product);
	const eventsInitializedRef = React.useRef(false);

	React.useEffect(() => {
		const productChanged = prevProductRef.current !== product;
		if (productChanged) {
			prevProductRef.current = product;
			eventsInitializedRef.current = false;
			form.reset({
				eventsEndpointUrl: product.config.events?.endpoint_url || "",
				events:
					product.config.events?.events ??
					catalogQuery.data?.items?.map((item) => item.type) ??
					[],
			});
			return;
		}

		if (
			!eventsInitializedRef.current &&
			catalogQuery.data?.items &&
			!form.state.isDirty
		) {
			eventsInitializedRef.current = true;
			form.reset({
				eventsEndpointUrl: product.config.events?.endpoint_url || "",
				events:
					product.config.events?.events ??
					catalogQuery.data.items.map((item) => item.type),
			});
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
				return a.type === "theme" ? -1 : 1;
			}
			return a.name.localeCompare(b.name);
		});
	}, [catalogQuery.data?.items]);

	const totalCatalogEvents = catalogQuery.data?.items?.length ?? 0;
	const themeEventsCount = React.useMemo(
		() =>
			(catalogQuery.data?.items ?? []).filter(
				(item) => item.group_type === "theme",
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
		form.reset({
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
			events:
				product.config.events?.events ??
				catalogQuery.data?.items?.map((item) => item.type) ??
				[],
		});
	};

	const endpointValue = form.state.values.eventsEndpointUrl;
	const isEndpointConfigured = Boolean(endpointValue?.trim());
	const endpointHostname = React.useMemo(() => {
		try {
			if (!endpointValue) return "";
			const parsed = new URL(endpointValue);
			return parsed.hostname;
		} catch {
			return endpointValue;
		}
	}, [endpointValue]);

	return (
		<form
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

					const coveragePercent =
						totalCatalogEvents > 0
							? Math.round((selectedEvents.length / totalCatalogEvents) * 100)
							: 0;

					return (
						<div className="flex flex-col gap-6">
							{/* Executive Metric Summary Tiles (macOS Glass / Depth) */}
							<div className="grid grid-cols-1 gap-3.5 sm:grid-cols-3">
								<div className="group rounded-2xl border border-border/60 bg-card/75 p-4.5 shadow-2xs backdrop-blur-md transition-all duration-200 hover:border-border hover:shadow-xs">
									<div className="flex items-center justify-between">
										<span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
											Delivery Endpoint
										</span>
										<StatusBadge
											tone={isEndpointConfigured ? "success" : "neutral"}
										/>
									</div>
									<div className="mt-2.5 truncate text-base font-semibold tracking-tight text-foreground">
										{isEndpointConfigured ? endpointHostname : "Not configured"}
									</div>
									<p className="mt-1 text-xs text-muted-foreground">
										{isEndpointConfigured
											? "Signed HTTP POST webhook delivery"
											: "Provide a destination URL below"}
									</p>
								</div>

								<div className="group rounded-2xl border border-border/60 bg-card/75 p-4.5 shadow-2xs backdrop-blur-md transition-all duration-200 hover:border-border hover:shadow-xs">
									<div className="flex items-center justify-between">
										<span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
											Subscribed Events
										</span>
										<Badge
											variant="secondary"
											className="font-mono text-[11px] font-semibold"
										>
											{selectedEvents.length}/{totalCatalogEvents}
										</Badge>
									</div>
									<div className="mt-2.5 text-base font-semibold tracking-tight text-foreground">
										{selectedEvents.length} Active Events
									</div>
									<p className="mt-1 text-xs text-muted-foreground">
										{coveragePercent}% catalog coverage
									</p>
								</div>

								<div className="group rounded-2xl border border-border/60 bg-card/75 p-4.5 shadow-2xs backdrop-blur-md transition-all duration-200 hover:border-border hover:shadow-xs">
									<div className="flex items-center justify-between">
										<span className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground">
											Signing Security
										</span>
										<StatusBadge
											tone={
												product.config.events?.signing_secret_obfuscated ||
												revealedSecret
													? "success"
													: "warning"
											}
										/>
									</div>
									<div className="mt-2.5 text-base font-semibold tracking-tight text-foreground">
										HMAC SHA-256
									</div>
									<p className="mt-1 truncate text-xs text-muted-foreground">
										{product.config.events?.signing_secret_obfuscated
											? `Key: ${product.config.events.signing_secret_obfuscated}`
											: "Mints automatically on first save"}
									</p>
								</div>
							</div>

							{/* Endpoint Configuration Card */}
							<Card className="rounded-2xl border-border/60 bg-card/75 pt-0 shadow-2xs backdrop-blur-md transition-all">
								<CardHeader className="rounded-t-2xl border-b border-border/40 bg-muted/25 px-5 py-4">
									<div className="flex items-center justify-between">
										<div className="space-y-1">
											<CardTitle className="inline-flex items-center gap-2 text-base font-semibold tracking-tight text-foreground">
												<Webhook className="size-4 text-primary" />
												Endpoint Configuration
											</CardTitle>
											<CardDescription className="text-xs text-muted-foreground">
												Where Anchor delivers signed JSON payloads when product
												events occur.
											</CardDescription>
										</div>
										<Badge
											variant={isEndpointConfigured ? "outline" : "secondary"}
											className="rounded-lg text-xs"
										>
											{isEndpointConfigured ? "Configured" : "Unset"}
										</Badge>
									</div>
								</CardHeader>
								<CardContent className="space-y-5 p-5">
									{revealedSecret ? (
										<Alert className="rounded-xl border-amber-500/40 bg-amber-500/10 text-foreground">
											<KeyRound className="size-4 text-amber-600 dark:text-amber-400" />
											<AlertTitle className="font-semibold tracking-tight text-amber-900 dark:text-amber-200">
												New Signing Secret Minted
											</AlertTitle>
											<AlertDescription className="mt-1 space-y-2 text-xs text-amber-800 dark:text-amber-300">
												<p>
													Store this secret in your webhook handler. It
													validates payload signatures in the{" "}
													<code className="font-mono font-semibold">
														anchor-signature
													</code>{" "}
													header. For security, it cannot be revealed again.
												</p>
												<div className="flex items-center gap-2 rounded-xl border border-amber-500/30 bg-background/90 p-2.5 shadow-2xs">
													<code className="flex-1 truncate font-mono text-xs text-foreground">
														{revealedSecret}
													</code>
													<Button
														type="button"
														variant="outline"
														size="sm"
														className="h-7 shrink-0 rounded-lg px-2.5 text-xs transition-transform active:scale-95"
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
												</div>
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
													<FieldLabel
														htmlFor="events-endpoint-url"
														className="text-xs font-semibold tracking-tight text-foreground uppercase"
													>
														Event endpoint URL
													</FieldLabel>
													<div className="relative mt-1">
														<Globe className="absolute top-2.5 left-3 size-4 text-muted-foreground/70" />
														<Input
															id="events-endpoint-url"
															className="h-9.5 rounded-xl border-border/70 pl-9 font-mono text-xs shadow-2xs transition-all focus-visible:ring-2 focus-visible:ring-primary/20"
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
													</div>
													<FieldDescription className="mt-1.5 text-xs leading-relaxed text-muted-foreground">
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

							{/* Event Subscriptions Catalog Card */}
							<Card className="rounded-2xl border-border/60 bg-card/75 pt-0 shadow-2xs backdrop-blur-md transition-all">
								<CardHeader className="rounded-t-2xl border-b border-border/40 bg-muted/25 px-5 py-4">
									<div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
										<div className="space-y-1">
											<CardTitle className="inline-flex items-center gap-2 text-base font-semibold tracking-tight text-foreground">
												<Layers className="size-4 text-primary" />
												Event Subscriptions
											</CardTitle>
											<CardDescription className="text-xs text-muted-foreground">
												Select the domain and integration events Anchor delivers
												to your endpoint.
											</CardDescription>
										</div>
										<div className="flex items-center gap-2">
											<Badge
												variant="secondary"
												className="rounded-lg font-mono text-[11px]"
											>
												{selectedEvents.length} selected
											</Badge>
											<Button
												type="button"
												variant="outline"
												size="sm"
												className="h-8 rounded-lg text-xs transition-transform active:scale-95"
												onClick={isAllSelected ? deselectAll : selectAll}
												disabled={totalCatalogEvents === 0}
											>
												{isAllSelected ? "Deselect all" : "Select all"}
											</Button>
										</div>
									</div>
								</CardHeader>
								<CardContent className="space-y-4 p-5">
									{/* Apple-style search field & segmented filter buttons */}
									<div className="flex flex-col gap-2.5 sm:flex-row sm:items-center sm:justify-between">
										<div className="relative max-w-sm flex-1">
											<Search className="absolute top-2.5 left-3 size-4 text-muted-foreground/70" />
											<Input
												placeholder="Filter events by name, code, or description..."
												value={searchQuery}
												onChange={(e) => setSearchQuery(e.target.value)}
												className="h-9 rounded-xl border-border/70 pl-9 text-xs shadow-2xs transition-all focus-visible:ring-2 focus-visible:ring-primary/20"
											/>
										</div>
										{/* Segmented control bar */}
										<div className="inline-flex rounded-xl border border-border/50 bg-muted/50 p-1 shadow-2xs backdrop-blur-xs">
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
												onClick={() => setFilterType("theme")}
												className={cn(
													"rounded-lg px-3 py-1 text-xs font-medium transition-all active:scale-[0.98]",
													filterType === "theme"
														? "bg-background text-foreground shadow-xs font-semibold"
														: "text-muted-foreground hover:text-foreground",
												)}
											>
												Domains ({themeEventsCount})
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
										</div>
									</div>

									{catalogQuery.isLoading ? (
										<div className="flex items-center justify-center py-12">
											<Spinner />
											<span className="ml-2 text-sm text-muted-foreground">
												Loading event catalog...
											</span>
										</div>
									) : filteredGroups.length === 0 ? (
										<div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border/60 py-12 text-center text-sm text-muted-foreground">
											<Radio className="mb-2.5 size-7 text-muted-foreground/40" />
											<p className="font-semibold tracking-tight text-foreground">
												No events found
											</p>
											<p className="text-xs text-muted-foreground">
												No catalog events match "{searchQuery}".
											</p>
										</div>
									) : (
										<div className="space-y-4">
											{filteredGroups.map((group) => {
												const groupTypes = group.events.map((e) => e.type);
												const selectedInGroup = groupTypes.filter((t) =>
													selectedEvents.includes(t),
												);
												const allGroupSelected =
													selectedInGroup.length === groupTypes.length &&
													groupTypes.length > 0;

												return (
													<div
														key={`${group.type}:${group.name}`}
														className="overflow-hidden rounded-2xl border border-border/50 bg-card/60 shadow-2xs transition-all duration-200 hover:border-border/80"
													>
														<div className="flex items-center justify-between border-b border-border/40 bg-muted/20 px-4 py-2.5">
															<div className="flex items-center gap-2">
																{getGroupIcon(group.name, group.type)}
																<span className="text-sm font-semibold tracking-tight text-foreground">
																	{group.name}
																</span>
															</div>
															<div className="flex items-center gap-3">
																<span className="font-mono text-xs text-muted-foreground">
																	{selectedInGroup.length}/{group.events.length}
																</span>
																<Button
																	type="button"
																	variant="ghost"
																	size="sm"
																	className="h-7 rounded-lg px-2 text-xs transition-transform active:scale-95"
																	onClick={() => toggleGroup(group)}
																>
																	{allGroupSelected ? "Deselect" : "Select all"}
																</Button>
															</div>
														</div>
														<div className="divide-y divide-border/30">
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
																			className="mt-0.5 rounded-md transition-transform active:scale-90"
																		/>
																		<div className="min-w-0 flex-1">
																			<div className="flex flex-wrap items-center gap-2">
																				<span className="text-sm font-medium tracking-tight text-foreground">
																					{event.name}
																				</span>
																				<code className="rounded-md border border-border/50 bg-muted/60 px-1.5 py-0.5 font-mono text-[11px] text-muted-foreground">
																					{event.type}
																				</code>
																				{isSelected ? (
																					<span className="inline-flex items-center gap-1 rounded-full bg-success/15 px-2 py-0.5 text-[10px] font-medium text-success">
																						<span className="size-1.5 rounded-full bg-success" />
																						Subscribed
																					</span>
																				) : null}
																			</div>
																			<p className="mt-1 text-xs leading-normal text-muted-foreground">
																				{event.description}
																			</p>
																		</div>
																	</label>
																);
															})}
														</div>
													</div>
												);
											})}
										</div>
									)}
								</CardContent>
							</Card>

							{/* Apple Floating Glass Action Bar */}
							<div className="sticky bottom-4 z-20 flex items-center justify-between rounded-2xl border border-border/70 bg-card/85 p-4 shadow-xl backdrop-blur-xl border-t border-white/20 dark:border-white/10">
								<div className="text-xs">
									<form.Subscribe selector={(state) => [state.isDirty]}>
										{([isDirty]) =>
											isDirty ? (
												<span className="inline-flex items-center gap-2 font-medium text-warning">
													<span className="size-2 animate-pulse rounded-full bg-warning" />
													Unsaved changes
												</span>
											) : (
												<span className="inline-flex items-center gap-2 text-muted-foreground">
													<CheckCircle2 className="size-3.5 text-success" />
													Endpoint and subscriptions saved
												</span>
											)
										}
									</form.Subscribe>
								</div>
								<div className="flex items-center gap-2.5">
									<form.Subscribe selector={(state) => [state.isDirty]}>
										{([isDirty]) => (
											<Button
												type="button"
												variant="outline"
												size="sm"
												className="rounded-xl transition-transform active:scale-95"
												disabled={!isDirty || updateMutation.isPending}
												onClick={handleReset}
											>
												<RotateCcw className="mr-1.5 size-3.5" />
												Discard
											</Button>
										)}
									</form.Subscribe>
									<form.Subscribe
										selector={(state) => [
											state.canSubmit,
											state.isSubmitting,
											state.isDirty,
											state.isValidating,
											state.isValid,
										]}
									>
										{([
											canSubmit,
											isSubmitting,
											isDirty,
											isValidating,
											isValid,
										]) => (
											<Button
												type="submit"
												size="sm"
												className="rounded-xl shadow-xs transition-transform active:scale-95"
												disabled={
													!canSubmit ||
													isSubmitting ||
													!isValid ||
													isValidating ||
													!isDirty ||
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
								</div>
							</div>
						</div>
					);
				}}
			</form.Field>
		</form>
	);
}
