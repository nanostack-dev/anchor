import {
	Alert,
	AlertDescription,
	AlertTitle,
	Badge,
	Button,
	ButtonLink,
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
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
import { ArrowSquareOutIcon } from "@phosphor-icons/react";
import { useId, useState } from "react";
import type { BillingAPI } from "./billing-api";
import { BillingSelect } from "./billing-fields";
import {
	type Organization,
	type State,
	zCheckoutRequest,
	zSubscriptionRequest,
} from "./billing-types";
import { type RunBillingAction, formatPrice } from "./pricing-workspace";

export function billingStatus(status: string): string {
	return status === "none" || !status
		? "No subscription"
		: status.replaceAll("_", " ");
}

export function formatBillingDate(value: string | null): string {
	return value
		? new Intl.DateTimeFormat("en", {
				dateStyle: "medium",
				timeStyle: "short",
			}).format(new Date(value))
		: "—";
}

function SubscriptionBadge({ organization }: { organization: Organization }) {
	return (
		<Badge
			tone={
				["active", "trialing"].includes(organization.status)
					? "success"
					: ["past_due", "unpaid", "incomplete"].includes(organization.status)
						? "warning"
						: "neutral"
			}
		>
			{billingStatus(organization.status)}
		</Badge>
	);
}

export function OrganizationBilling({
	state,
	organization,
	api,
	run,
	disabled,
}: {
	state: State;
	organization: Organization;
	api: BillingAPI;
	run: RunBillingAction;
	disabled: boolean;
}) {
	const id = useId();
	const activePrices = state.prices.filter((price) => price.active);
	const [priceId, setPriceId] = useState(
		activePrices.find((price) => price.id === organization.price_id)?.id ??
			activePrices[0]?.id ??
			"",
	);
	const [trialDays, setTrialDays] = useState("0");
	const [validationError, setValidationError] = useState("");
	const [hostedPage, setHostedPage] = useState<{
		url: string;
		label: string;
	} | null>(null);
	const template = state.templates.find(
		(item) => item.id === organization.template_id,
	);
	const currentPrice = state.prices.find(
		(item) => item.id === organization.price_id,
	);
	const hasSubscription =
		!!organization.subscription_id &&
		!["canceled", "incomplete_expired"].includes(organization.status);
	return (
		<Stack space="lg">
			{organization.sync_error && (
				<Alert tone="critical">
					<AlertTitle>License synchronization needs attention</AlertTitle>
					<AlertDescription>{organization.sync_error}</AlertDescription>
				</Alert>
			)}
			{organization.pending_update && (
				<Alert tone="warning">
					<AlertTitle>Price change is waiting for payment</AlertTitle>
					<AlertDescription>
						Stripe has not applied the new price yet. The current license stays
						in place until the update completes.
					</AlertDescription>
				</Alert>
			)}
			<Columns collapseBelow="lg" space="lg">
				<Column>
					<Card variant="outline">
						<CardHeader>
							<CardTitle>
								<Heading level={2}>Stripe subscription</Heading>
							</CardTitle>
							<CardDescription>
								Payment and subscription state from Stripe.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="lg">
								<Inline>
									<SubscriptionBadge organization={organization} />
									{organization.cancel_at_period_end && (
										<Badge tone="warning">Cancellation scheduled</Badge>
									)}
								</Inline>
								<Box className="break-words [overflow-wrap:anywhere]">
									<Stack space="sm">
										<Text weight="medium">
											{currentPrice
												? `${currentPrice.name} · ${formatPrice(currentPrice)}`
												: "No recurring price"}
										</Text>
										<Text tone="muted">
											{organization.cancel_at_period_end
												? "Access until"
												: "Current period ends"}
											: {formatBillingDate(organization.current_period_end)}
										</Text>
										<Text size="xs" font="mono" tone="muted">
											Customer: {organization.customer_id || "Not created"}
										</Text>
										<Text size="xs" font="mono" tone="muted">
											Subscription: {organization.subscription_id || "None"}
										</Text>
									</Stack>
								</Box>
								<Inline>
									<Button
										disabled={disabled}
										onClick={() =>
											void run(
												() => api.sync(organization.id),
												"Stripe and Anchor state reconciled.",
											)
										}
									>
										Reconcile
									</Button>
									<Button
										disabled={disabled || !organization.customer_id}
										onClick={() =>
											void run(
												async () =>
													setHostedPage({
														url: await api.portal(organization.id),
														label: "customer portal",
													}),
												"Customer portal ready.",
											)
										}
									>
										Customer portal
									</Button>
								</Inline>
								{hasSubscription && (
									<Inline>
										{organization.cancel_at_period_end ? (
											<Button
												disabled={disabled}
												onClick={() =>
													void run(
														() => api.resume(organization.id),
														"Subscription resumed.",
													)
												}
											>
												Resume subscription
											</Button>
										) : (
											<Button
												tone="critical"
												disabled={disabled}
												onClick={() =>
													void run(
														() => api.cancel(organization.id),
														"Cancellation scheduled for the period end.",
													)
												}
											>
												Cancel at period end
											</Button>
										)}
									</Inline>
								)}
							</Stack>
						</CardContent>
					</Card>
				</Column>
				<Column>
					<Card variant="outline">
						<CardHeader>
							<CardTitle>
								<Heading level={2}>Anchor license</Heading>
							</CardTitle>
							<CardDescription>
								What this organization can use right now.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Stack space="md">
								<Box className="break-words [overflow-wrap:anywhere]">
									<Heading level={3}>
										{template?.name ??
											(organization.template_id || "No license")}
									</Heading>
								</Box>
								<Inline>
									<Badge
										tone={
											organization.sync_state === "synced"
												? "success"
												: organization.sync_state === "error"
													? "critical"
													: "warning"
										}
									>
										{organization.sync_state || "Awaiting sync"}
									</Badge>
								</Inline>
								<Text size="xs" tone="muted">
									Last synchronized:{" "}
									{formatBillingDate(organization.last_synced_at)}
								</Text>
								{Object.keys(organization.license_values).length === 0 ? (
									<Text tone="muted">No license values granted yet.</Text>
								) : (
									<Stack space="sm" aria-label="Granted license values">
										{Object.entries(organization.license_values).map(
											([name, value]) => (
												<Box
													key={name}
													className="flex min-w-0 flex-wrap justify-between gap-2 border-b border-border pb-2 break-words [overflow-wrap:anywhere]"
												>
													<Text tone="muted">{name}</Text>
													<Text font="mono">
														{typeof value === "string"
															? value
															: JSON.stringify(value)}
													</Text>
												</Box>
											),
										)}
									</Stack>
								)}
							</Stack>
						</CardContent>
					</Card>
				</Column>
			</Columns>
			{hostedPage && (
				<Alert tone="info">
					<AlertTitle>Stripe {hostedPage.label} is ready</AlertTitle>
					<AlertDescription>
						<Stack space="sm">
							<Text>
								Continue to Stripe, then return and reconcile the organization.
							</Text>
							<Inline>
								<ButtonLink href={hostedPage.url} icon={ArrowSquareOutIcon}>
									Continue to {hostedPage.label}
								</ButtonLink>
							</Inline>
						</Stack>
					</AlertDescription>
				</Alert>
			)}
			<Card variant="outline">
				<CardHeader>
					<CardTitle>
						{hasSubscription
							? "Change recurring price"
							: "Start a subscription"}
					</CardTitle>
					<CardDescription>
						{hasSubscription
							? "Stripe calculates proration. The license follows the applied subscription price."
							: "Checkout collects a payment method on Stripe’s hosted page."}
					</CardDescription>
				</CardHeader>
				<CardContent>
					<Stack space="lg">
						{activePrices.length === 0 && (
							<Text tone="muted">Create an active price in Pricing first.</Text>
						)}
						<BillingSelect
							label="Recurring price"
							value={priceId}
							onChange={(value) => {
								setPriceId(value);
								setValidationError("");
								setHostedPage(null);
							}}
							options={activePrices.map((price) => ({
								value: price.id,
								label: `${price.name} · ${formatPrice(price)}`,
							}))}
							disabled={disabled || activePrices.length === 0}
						/>
						{!hasSubscription && (
							<Field invalid={!!validationError}>
								<FieldLabel htmlFor={`${id}-trial`}>Trial days</FieldLabel>
								<Input
									id={`${id}-trial`}
									type="number"
									min={0}
									max={30}
									step={1}
									value={trialDays}
									onChange={(event) => setTrialDays(event.target.value)}
									disabled={disabled}
								/>
								<FieldDescription>
									0 to 30 days. Leave 0 for immediate payment.
								</FieldDescription>
							</Field>
						)}
						<FieldError>{validationError}</FieldError>
						<Inline>
							<Button
								variant="solid"
								tone="brand"
								disabled={
									disabled ||
									!activePrices.some((price) => price.id === priceId) ||
									(hasSubscription && priceId === organization.price_id)
								}
								onClick={() => {
									if (hasSubscription) {
										const body = zSubscriptionRequest.safeParse({
											price_id: priceId,
										});
										if (!body.success) {
											setValidationError("Choose an active price.");
											return;
										}
										setValidationError("");
										void run(
											() => api.changeSubscription(organization.id, body.data),
											"Subscription price updated.",
										);
									} else {
										const body = zCheckoutRequest.safeParse({
											price_id: priceId,
											trial_days: Number(trialDays),
										});
										if (!body.success) {
											setValidationError(
												"Choose a price and a whole trial duration of 0 to 30 days.",
											);
											return;
										}
										setValidationError("");
										void run(
											async () =>
												setHostedPage({
													url: await api.checkout(organization.id, body.data),
													label: "Checkout",
												}),
											"Checkout ready.",
										);
									}
								}}
							>
								{hasSubscription ? "Change price" : "Create Checkout"}
							</Button>
						</Inline>
					</Stack>
				</CardContent>
			</Card>
		</Stack>
	);
}
