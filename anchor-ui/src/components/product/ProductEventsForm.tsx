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
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
import { useForm } from "@tanstack/react-form";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, Copy } from "lucide-react";
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

	React.useEffect(() => {
		form.reset({
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
			events:
				product.config.events?.events ??
				catalogQuery.data?.items?.map((item) => item.type) ??
				[],
		});
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

	return (
		<form
			onSubmit={(e) => {
				e.preventDefault();
				e.stopPropagation();
				form.handleSubmit();
			}}
			className="flex w-full flex-col gap-6"
		>
			<FieldGroup>
				{revealedSecret ? (
					<Alert>
						<AlertTitle>Signing secret</AlertTitle>
						<AlertDescription>
							Store this secret now. Later reads return only the obfuscated
							marker.
							<div className="mt-2 flex items-center gap-2">
								<code className="truncate font-mono text-xs">
									{revealedSecret}
								</code>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									className="size-6 shrink-0 p-0"
									onClick={() => {
										void navigator.clipboard.writeText(revealedSecret);
										setSecretCopied(true);
										window.setTimeout(() => setSecretCopied(false), 1500);
									}}
								>
									{secretCopied ? (
										<Check className="size-3 text-success" />
									) : (
										<Copy className="size-3 text-muted-foreground" />
									)}
								</Button>
							</div>
						</AlertDescription>
					</Alert>
				) : null}
				<form.Field name="eventsEndpointUrl">
					{(field) => (
						<Field
							data-disabled={updateMutation.isPending}
							data-invalid={field.state.meta.errors.length > 0}
						>
							<FieldLabel htmlFor="events-endpoint-url">
								Event endpoint URL
							</FieldLabel>
							<Input
								id="events-endpoint-url"
								placeholder="https://example.com/anchor/events"
								value={field.state.value}
								onChange={(e) => field.handleChange(e.target.value)}
								onBlur={field.handleBlur}
								disabled={updateMutation.isPending}
								aria-invalid={field.state.meta.errors.length > 0}
							/>
							<FieldDescription>
								Anchor POSTs signed product events here. Leave empty to clear
								the endpoint. Production requires HTTPS. Anchor mints the
								signing secret on first save.
								{product.config.events?.signing_secret_obfuscated
									? ` Stored secret: ${product.config.events.signing_secret_obfuscated}.`
									: ""}
							</FieldDescription>
							<FormValidationError field={field} />
						</Field>
					)}
				</form.Field>
			</FieldGroup>

			<form.Field name="events">
				{(field) => {
					const selectedEvents = field.state.value ?? [];
					const isAllSelected =
						totalCatalogEvents > 0 &&
						totalCatalogEvents === selectedEvents.length;

					const toggleEvent = (eventType: string) => {
						const next = selectedEvents.includes(eventType)
							? selectedEvents.filter((t) => t !== eventType)
							: [...selectedEvents, eventType];
						field.handleChange(next);
					};

					const toggleGroup = (group: EventGroup) => {
						const groupTypes = group.events.map((e) => e.type);
						const allInGroupSelected = groupTypes.every((t) =>
							selectedEvents.includes(t),
						);
						const next = allInGroupSelected
							? selectedEvents.filter((t) => !groupTypes.includes(t))
							: Array.from(new Set([...selectedEvents, ...groupTypes]));
						field.handleChange(next);
					};

					const selectAll = () => {
						const allTypes =
							catalogQuery.data?.items?.map((item) => item.type) ?? [];
						field.handleChange(allTypes);
					};

					const deselectAll = () => {
						field.handleChange([]);
					};

					return (
						<div className="flex flex-col gap-4">
							<div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between border-b pb-3">
								<div>
									<h3 className="text-base font-medium text-foreground">
										Subscribed Events
									</h3>
									<p className="text-sm text-muted-foreground">
										Select which events to receive at this endpoint. Events are
										grouped by domain theme and webhook integrations.
									</p>
								</div>
								<div className="flex items-center gap-2">
									<Badge variant="secondary" className="font-mono text-xs">
										{selectedEvents.length}/{totalCatalogEvents} selected
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
								</div>
							</div>

							{catalogQuery.isLoading ? (
								<div className="flex items-center justify-center py-8">
									<Spinner />
									<span className="ml-2 text-sm text-muted-foreground">
										Loading event catalog...
									</span>
								</div>
							) : groups.length === 0 ? (
								<p className="text-sm text-muted-foreground py-4">
									No events available in the catalog.
								</p>
							) : (
								<div className="grid grid-cols-1 gap-4">
									{groups.map((group) => {
										const groupTypes = group.events.map((e) => e.type);
										const selectedInGroup = groupTypes.filter((t) =>
											selectedEvents.includes(t),
										);
										const allGroupSelected =
											selectedInGroup.length === groupTypes.length &&
											groupTypes.length > 0;

										return (
											<Card key={`${group.type}:${group.name}`}>
												<CardHeader className="py-3 px-4 flex flex-row items-center justify-between space-y-0 bg-muted/40 border-b">
													<div className="flex items-center gap-2">
														<CardTitle className="text-sm font-semibold">
															{group.name}
														</CardTitle>
														<Badge
															variant={
																group.type === "integration"
																	? "default"
																	: "secondary"
															}
															className="text-[10px] uppercase tracking-wider"
														>
															{group.type === "integration"
																? "Integration"
																: "Domain Theme"}
														</Badge>
													</div>
													<div className="flex items-center gap-2">
														<span className="text-xs text-muted-foreground font-mono">
															{selectedInGroup.length}/{group.events.length}
														</span>
														<Button
															type="button"
															variant="ghost"
															size="sm"
															className="h-7 text-xs px-2"
															onClick={() => toggleGroup(group)}
														>
															{allGroupSelected ? "Deselect" : "Select all"}
														</Button>
													</div>
												</CardHeader>
												<CardContent className="p-0 divide-y">
													{group.events.map((event) => {
														const isSelected = selectedEvents.includes(
															event.type,
														);
														return (
															<label
																key={event.type}
																htmlFor={`event-checkbox-${event.type}`}
																className="flex items-start gap-3 p-3 hover:bg-muted/30 cursor-pointer transition-colors"
															>
																<Checkbox
																	id={`event-checkbox-${event.type}`}
																	checked={isSelected}
																	onCheckedChange={() =>
																		toggleEvent(event.type)
																	}
																	className="mt-1"
																/>
																<div className="flex-1 min-w-0">
																	<div className="flex flex-wrap items-center gap-2">
																		<span className="text-sm font-medium text-foreground">
																			{event.name}
																		</span>
																		<code className="text-xs bg-muted text-muted-foreground px-1.5 py-0.5 rounded font-mono">
																			{event.type}
																		</code>
																	</div>
																	<p className="text-xs text-muted-foreground mt-0.5">
																		{event.description}
																	</p>
																</div>
															</label>
														);
													})}
												</CardContent>
											</Card>
										);
									})}
								</div>
							)}
						</div>
					);
				}}
			</form.Field>

			<div className="flex justify-end">
				<form.Subscribe
					selector={(state) => [
						state.canSubmit,
						state.isSubmitting,
						state.isDirty,
						state.isValidating,
						state.isValid,
					]}
				>
					{([canSubmit, isSubmitting, isDirty, isValidating, isValid]) => (
						<Button
							type="submit"
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
		</form>
	);
}
