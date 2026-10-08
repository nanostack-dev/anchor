import {
	Alert,
	AlertDescription,
	AlertTitle,
	Badge,
	Box,
	Button,
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	Field,
	FieldDescription,
	FieldError,
	FieldLabel,
	Heading,
	Inline,
	Input,
	Stack,
	Switch,
	Text,
} from "@nanostackorg/design-system";
import { useQuery } from "@tanstack/react-query";
import { useEffect, useId, useRef, useState } from "react";
import { type BillingAPI, billingError } from "./billing-api";
import { BillingSelect } from "./billing-fields";
import {
	FraudRefundCurrency,
	type FraudRefundPolicy,
	zFraudRefundPolicy,
} from "./billing-types";
import {
	FraudRefundActivity,
	formatFraudAmount,
} from "./fraud-refund-activity";

const policySchema = zFraudRefundPolicy.superRefine((policy, context) => {
	if (policy.enabled && policy.max_amount === 0)
		context.addIssue({
			code: "custom",
			path: ["max_amount"],
			message:
				"Enter a whole amount greater than zero before enabling refunds.",
		});
});

function policyForm(policy: FraudRefundPolicy) {
	return {
		enabled: policy.enabled,
		currency: policy.currency,
		maximum: String(policy.max_amount),
	};
}

export function FraudRefundSettings({
	api,
	productId,
	connectionAvailable,
	disabled = false,
	onSaved,
}: {
	api: BillingAPI;
	productId: string;
	connectionAvailable: boolean;
	disabled?: boolean;
	onSaved?: () => void;
}) {
	const [form, setForm] = useState(() =>
		policyForm({
			enabled: false,
			currency: FraudRefundCurrency.USD,
			max_amount: 0,
		}),
	);
	const [errors, setErrors] = useState<{ maximum?: string; currency?: string }>(
		{},
	);
	const [saving, setSaving] = useState(false);
	const [hydrated, setHydrated] = useState(false);
	const [notice, setNotice] = useState("");
	const [actionError, setActionError] = useState("");
	const dirty = useRef(false);
	const switchId = useId();
	const amountId = useId();
	const query = useQuery({
		queryKey: ["product-stripe-billing", productId],
		queryFn: () => api.load(),
		retry: false,
		refetchInterval: 5000,
		refetchOnWindowFocus: true,
	});
	const state = query.data;
	const scopeMatches = !state || state.product.id === productId;
	const policy = scopeMatches ? state?.settings.fraud_refund_policy : undefined;
	useEffect(() => {
		if (policy && !dirty.current) {
			setForm(policyForm(policy));
			setHydrated(true);
		}
	}, [policy]);
	const writesDisabled =
		disabled ||
		!connectionAvailable ||
		!hydrated ||
		!policy ||
		!scopeMatches ||
		saving ||
		!!query.error;
	function edit(next: Partial<typeof form>) {
		dirty.current = true;
		setNotice("");
		setForm((previous) => ({ ...previous, ...next }));
	}
	async function save() {
		if (writesDisabled) return;
		const next: FraudRefundPolicy = {
			enabled: form.enabled,
			currency: form.currency,
			max_amount: /^\d+$/.test(form.maximum)
				? Number(form.maximum)
				: Number.NaN,
		};
		const parsed = policySchema.safeParse(next);
		if (!parsed.success) {
			const maximumIssue = parsed.error.issues.find(
				(issue) => issue.path[0] === "max_amount",
			);
			setErrors({
				maximum:
					maximumIssue?.code === "custom"
						? maximumIssue.message
						: maximumIssue
							? "Enter a whole amount between 0 and 99,999,999 cents."
							: undefined,
				currency: parsed.error.issues.some(
					(issue) => issue.path[0] === "currency",
				)
					? "Choose USD, CAD or EUR."
					: undefined,
			});
			return;
		}
		setErrors({});
		setNotice("");
		setActionError("");
		setSaving(true);
		try {
			await api.updateSettings({ fraud_refund_policy: next });
			const readBack = await query.refetch();
			if (readBack.error || readBack.data?.product.id !== productId)
				throw new Error(
					"The policy was saved, but refreshed billing state could not be loaded. Reload to verify it.",
				);
			dirty.current = false;
			setForm(policyForm(readBack.data.settings.fraud_refund_policy));
			setNotice("Refund policy saved.");
			onSaved?.();
		} catch (error) {
			setActionError(billingError(error));
		} finally {
			setSaving(false);
		}
	}
	return (
		<Stack space="lg">
			<Card variant="outline">
				<CardHeader>
					<Heading level={2}>Fraud warning refunds</Heading>
					<CardDescription>
						Opt in to automatic refunds when Stripe sends an early fraud
						warning. Off by default.
					</CardDescription>
				</CardHeader>
				<CardContent>
					<Stack space="lg">
						<Alert tone="info">
							<AlertTitle>Refunds have timing and fee limits</AlertTitle>
							<AlertDescription>
								Early warnings can arrive after payment or a dispute. Refunds
								may not prevent a dispute or its fees, and original processing
								fees may not be returned. Only eligible payments within this
								policy are refunded, for their full remaining captured amount.
								Subscriptions, customers and Anchor licenses stay unchanged.
							</AlertDescription>
						</Alert>
						{query.isPending && (
							<Text tone="muted">Loading refund policy…</Text>
						)}
						{!connectionAvailable && (
							<Text tone="muted">
								Enable an active Stripe connection before changing the refund
								policy.
							</Text>
						)}
						{(query.error || actionError || !scopeMatches) && (
							<Alert tone="critical">
								<AlertTitle>Refund policy needs attention</AlertTitle>
								<AlertDescription>
									{!scopeMatches
										? "Billing state does not match this product."
										: actionError || billingError(query.error)}
								</AlertDescription>
							</Alert>
						)}
						{notice && (
							<Alert tone="success">
								<AlertTitle>Saved</AlertTitle>
								<AlertDescription>{notice}</AlertDescription>
							</Alert>
						)}
						<Box
							as="form"
							noValidate
							onSubmit={(event) => {
								event.preventDefault();
								void save();
							}}
						>
							<Stack space="lg">
								<Field>
									<Box className="flex items-start justify-between gap-4">
										<Stack space="xs">
											<FieldLabel htmlFor={switchId}>
												Automatically refund fraud warnings
											</FieldLabel>
											<FieldDescription>
												Applies to new early fraud warnings. Existing issued
												refunds continue to be monitored after you turn this
												off.
											</FieldDescription>
										</Stack>
										<Switch
											id={switchId}
											checked={form.enabled}
											disabled={writesDisabled}
											onCheckedChange={(enabled) => edit({ enabled })}
										/>
									</Box>
								</Field>
								<BillingSelect
									label="Refund currency"
									value={form.currency}
									disabled={writesDisabled}
									error={errors.currency}
									onChange={(value) => {
										const parsed = Object.values(FraudRefundCurrency).find(
											(currency) => currency === value,
										);
										if (parsed) edit({ currency: parsed });
									}}
									options={Object.values(FraudRefundCurrency).map((value) => ({
										value,
										label: value.toUpperCase(),
									}))}
									description="Only payments in this currency qualify. USD, CAD and EUR use cents as minor units."
								/>
								<Field invalid={!!errors.maximum}>
									<FieldLabel htmlFor={amountId}>
										Maximum payment amount (minor units)
									</FieldLabel>
									<Input
										id={amountId}
										inputMode="numeric"
										value={form.maximum}
										disabled={writesDisabled}
										aria-invalid={!!errors.maximum}
										onChange={(event) => edit({ maximum: event.target.value })}
									/>
									<FieldDescription>
										The cap applies to the original payment amount, even after
										partial refunds. Enter a whole number of cents; for example,
										5000 means {formatFraudAmount(5000, form.currency)}.
									</FieldDescription>
									<FieldError>{errors.maximum}</FieldError>
								</Field>
								<Inline>
									<Button
										type="submit"
										variant="solid"
										tone="brand"
										disabled={writesDisabled}
										loading={saving}
									>
										Save refund policy
									</Button>
									<Button
										disabled={saving || query.isFetching}
										onClick={() => void query.refetch()}
									>
										Reload refund policy
									</Button>
									{policy && (
										<Badge tone={policy.enabled ? "warning" : "neutral"}>
											{policy.enabled
												? "Automatic refunds on"
												: "Automatic refunds off"}
										</Badge>
									)}
								</Inline>
							</Stack>
						</Box>
					</Stack>
				</CardContent>
			</Card>
			{state && scopeMatches && <FraudRefundActivity state={state} />}
		</Stack>
	);
}
