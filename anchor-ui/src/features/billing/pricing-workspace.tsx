import {
	Badge,
	Button,
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
	Field,
	FieldDescription,
	FieldError,
	FieldLabel,
	Heading,
	Input,
	Text,
} from "@nanostackorg/design-system";
import {
	Box,
	Column,
	Columns,
	Inline,
	Stack,
} from "@nanostackorg/design-system";
import { useEffect, useId, useState } from "react";
import type { BillingAPI } from "./billing-api";
import { BillingSelect } from "./billing-fields";
import {
	type Price,
	type State,
	zCreatePriceRequest,
	zUpdateSettingsRequest,
} from "./billing-types";

export type RunBillingAction = (
	action: () => Promise<unknown>,
	message: string,
) => Promise<boolean>;

export function formatPrice(price: Price): string {
	return `${new Intl.NumberFormat("en", {
		style: "currency",
		currency: price.currency,
		currencyDisplay: "code",
	}).format(price.amount / 100)} / ${price.interval}`;
}

function CreatePriceForm({
	state,
	api,
	run,
	disabled,
	loading,
}: {
	state: State;
	api: BillingAPI;
	run: RunBillingAction;
	disabled: boolean;
	loading: boolean;
}) {
	const id = useId();
	const templates = state.templates.filter((template) => !template.archived);
	const [name, setName] = useState("");
	const [amount, setAmount] = useState("");
	const [templateId, setTemplateId] = useState(templates[0]?.id ?? "");
	const [currency, setCurrency] = useState("usd");
	const [interval, setInterval] = useState("month");
	const [errors, setErrors] = useState<Record<string, string>>({});

	return (
		<Card variant="outline">
			<CardHeader>
				<CardTitle>Create a price</CardTitle>
				<CardDescription>
					Connect a recurring Stripe price to an Anchor license template.
				</CardDescription>
			</CardHeader>
			<CardContent>
				<Box
					as="form"
					noValidate
					onSubmit={async (event) => {
						event.preventDefault();
						const result = zCreatePriceRequest.safeParse({
							name: name.trim(),
							amount: Number(amount),
							template_id: templateId,
							currency,
							interval,
						});
						if (!result.success) {
							setErrors(
								Object.fromEntries(
									result.error.issues.map((issue) => [
										String(issue.path[0]),
										issue.path[0] === "amount"
											? "Enter a whole amount between 1 and 99,999,999 cents."
											: issue.path[0] === "name"
												? "Enter a price name of 1 to 120 characters."
												: issue.message,
									]),
								),
							);
							return;
						}
						setErrors({});
						if (
							await run(() => api.createPrice(result.data), "Price created.")
						) {
							setName("");
							setAmount("");
						}
					}}
				>
					<Stack space="lg">
						<Field invalid={!!errors.name}>
							<FieldLabel htmlFor={`${id}-name`}>Price name</FieldLabel>
							<Input
								id={`${id}-name`}
								value={name}
								onChange={(event) => setName(event.target.value)}
								placeholder="Pro monthly"
								aria-invalid={!!errors.name}
								disabled={disabled}
							/>
							<FieldError>{errors.name}</FieldError>
						</Field>
						<BillingSelect
							label="License template"
							value={templateId}
							onChange={setTemplateId}
							options={templates.map((template) => ({
								value: template.id,
								label: template.name,
							}))}
							error={errors.template_id}
							disabled={disabled || templates.length === 0}
						/>
						<Columns collapseBelow="sm">
							<Column>
								<Field invalid={!!errors.amount}>
									<FieldLabel htmlFor={`${id}-amount`}>
										Amount in cents
									</FieldLabel>
									<Input
										id={`${id}-amount`}
										type="number"
										inputMode="numeric"
										step={1}
										min={1}
										value={amount}
										onChange={(event) => setAmount(event.target.value)}
										placeholder="2900"
										aria-invalid={!!errors.amount}
										disabled={disabled}
									/>
									<FieldDescription>2900 = 29.00</FieldDescription>
									<FieldError>{errors.amount}</FieldError>
								</Field>
							</Column>
							<Column>
								<BillingSelect
									label="Currency"
									value={currency}
									onChange={setCurrency}
									options={[
										{ value: "usd", label: "USD" },
										{ value: "cad", label: "CAD" },
										{ value: "eur", label: "EUR" },
									]}
									disabled={disabled}
								/>
							</Column>
						</Columns>
						<BillingSelect
							label="Billing interval"
							value={interval}
							onChange={setInterval}
							options={[
								{ value: "month", label: "Monthly" },
								{ value: "year", label: "Yearly" },
							]}
							disabled={disabled}
						/>
						<Button
							type="submit"
							variant="solid"
							tone="brand"
							loading={loading}
							disabled={disabled || templates.length === 0}
						>
							Create price
						</Button>
					</Stack>
				</Box>
			</CardContent>
		</Card>
	);
}

function FallbackSettings({
	state,
	api,
	run,
	disabled,
}: {
	state: State;
	api: BillingAPI;
	run: RunBillingAction;
	disabled: boolean;
}) {
	const [fallback, setFallback] = useState(state.settings.fallback_template_id);
	const [dirty, setDirty] = useState(false);
	useEffect(() => {
		if (!dirty) setFallback(state.settings.fallback_template_id);
	}, [state.settings.fallback_template_id, dirty]);
	return (
		<Card variant="outline">
			<CardHeader>
				<CardTitle>After a subscription ends</CardTitle>
				<CardDescription>
					Active and trialing subscriptions grant their price’s template. Ended,
					unpaid, paused, and expired incomplete subscriptions use this
					fallback. Payment failures keep the current license while unresolved.
					Cancellation keeps the paid license until the period ends.
				</CardDescription>
			</CardHeader>
			<CardContent>
				<Stack>
					<BillingSelect
						label="Fallback license template"
						value={fallback}
						onChange={(value) => {
							setFallback(value);
							setDirty(true);
						}}
						options={state.templates
							.filter((template) => !template.archived)
							.map((template) => ({
								value: template.id,
								label: template.name,
							}))}
						disabled={disabled}
					/>
					<Button
						disabled={disabled || !dirty || !fallback}
						onClick={async () => {
							const body = zUpdateSettingsRequest.parse({
								fallback_template_id: fallback,
							});
							if (
								await run(
									() =>
										api.updateSettings({
											fallback_template_id: body.fallback_template_id,
										}),
									"Fallback template saved.",
								)
							)
								setDirty(false);
						}}
					>
						Save fallback
					</Button>
				</Stack>
			</CardContent>
		</Card>
	);
}

export function PricingWorkspace({
	state,
	api,
	run,
	disabled,
	loading,
}: {
	state: State;
	api: BillingAPI;
	run: RunBillingAction;
	disabled: boolean;
	loading: boolean;
}) {
	return (
		<Stack space="xl">
			<Columns collapseBelow="lg" space="lg">
				<Column width="2/3">
					<Stack space="lg">
						<Text tone="muted">
							Stripe amounts, currencies, and intervals are immutable. Create a
							new price for a change; archive the old price to stop new sales.
							Existing subscriptions keep their price.
						</Text>
						{state.prices.length === 0 ? (
							<Empty>
								<EmptyHeader>
									<EmptyTitle>No prices yet</EmptyTitle>
									<EmptyDescription>
										Create a recurring price to start a checkout.
									</EmptyDescription>
								</EmptyHeader>
							</Empty>
						) : (
							<Stack as="ul" space="md" aria-label="Pricing catalog">
								{state.prices.map((price) => (
									<Box as="li" key={price.id} className="list-none">
										<Card variant="outline">
											<CardHeader>
												<Box className="flex min-w-0 flex-wrap items-start justify-between gap-3">
													<Stack space="xs">
														<Box className="min-w-0 break-words [overflow-wrap:anywhere]">
															<Heading level={2}>{price.name}</Heading>
														</Box>
														<Text weight="semibold">{formatPrice(price)}</Text>
													</Stack>
													<Badge tone={price.active ? "success" : "neutral"}>
														{price.active ? "Active" : "Archived"}
													</Badge>
												</Box>
											</CardHeader>
											<CardContent>
												<Stack space="md">
													<Box className="break-words [overflow-wrap:anywhere]">
														<Text tone="muted">
															Grants{" "}
															{state.templates.find(
																(template) => template.id === price.template_id,
															)?.name ?? price.template_id}
														</Text>
														<Text size="xs" font="mono" tone="muted">
															{price.stripe_price_id}
														</Text>
													</Box>
													{price.active && (
														<Inline>
															<Button
																size="sm"
																disabled={disabled}
																aria-label={`Archive ${price.name}`}
																onClick={() =>
																	void run(
																		() => api.archivePrice(price.id),
																		"Price archived. Existing subscriptions keep their price.",
																	)
																}
															>
																Archive
															</Button>
														</Inline>
													)}
												</Stack>
											</CardContent>
										</Card>
									</Box>
								))}
							</Stack>
						)}
						<FallbackSettings
							state={state}
							api={api}
							run={run}
							disabled={disabled}
						/>
					</Stack>
				</Column>
				<Column width="1/3">
					<CreatePriceForm
						state={state}
						api={api}
						run={run}
						disabled={disabled}
						loading={loading}
					/>
				</Column>
			</Columns>
		</Stack>
	);
}
