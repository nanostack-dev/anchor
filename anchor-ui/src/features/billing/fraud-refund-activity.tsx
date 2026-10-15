import {
	Badge,
	type BadgeTone,
	Box,
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
	Stack,
	Text,
} from "@nanostackorg/design-system";
import type { FraudRefund, State } from "./billing-types";
import { formatBillingDate } from "./organization-workspace";

const statusDetails: Record<
	FraudRefund["status"],
	{ label: string; tone: BadgeTone }
> = {
	succeeded: { label: "Refunded", tone: "success" },
	skipped: { label: "Skipped", tone: "neutral" },
	processing: { label: "Pending", tone: "warning" },
	pending: { label: "Pending", tone: "warning" },
	failed: { label: "Failed", tone: "critical" },
	canceled: { label: "Canceled", tone: "neutral" },
	requires_action: { label: "Needs review", tone: "warning" },
	review_required: { label: "Needs review", tone: "warning" },
};

const reasons: Record<string, string> = {
	policy_disabled: "Automatic refunds were off when this warning arrived.",
	warning_not_actionable:
		"This warning does not qualify for an automatic refund.",
	formal_dispute: "The payment has a formal dispute and needs manual review.",
	charge_not_eligible: "The payment is not eligible for an automatic refund.",
	currency_mismatch: "The payment currency does not match the refund policy.",
	amount_exceeds_limit: "The original payment amount exceeds the policy limit.",
	no_refundable_balance: "The payment has no remaining refundable balance.",
	ownership_unverified:
		"The payment could not be linked safely to this product.",
	existing_refund_pending: "Another refund is still pending for this payment.",
	refund_requested:
		"A refund was requested for the full remaining captured amount.",
	refund_status_updated: "The refund status was refreshed from Stripe.",
	idempotency_window_expired:
		"The request needs manual review before another refund attempt.",
	refund_failed: "Stripe did not complete the refund.",
	refund_receipt_unverified:
		"The refund receipt could not be verified. Review it in Stripe before retrying.",
	refund_status_unknown:
		"Stripe returned an unknown refund status. Review it in Stripe before retrying.",
	charge_changed:
		"The payment changed before the refund could be verified. Review it in Stripe.",
};

export function formatFraudAmount(amount: number, currency: string): string {
	if (!Number.isSafeInteger(amount) || amount <= 0 || !currency)
		return "Amount unavailable";
	const code = currency.toUpperCase();
	if (!["usd", "cad", "eur"].includes(currency.toLowerCase()))
		return `${code} ${new Intl.NumberFormat("en-US").format(amount)} minor units`;
	return `${code} ${new Intl.NumberFormat("en-US", {
		minimumFractionDigits: 2,
		maximumFractionDigits: 2,
	}).format(amount / 100)}`;
}

export function FraudRefundActivity({
	state,
	organizationId,
}: { state: State; organizationId?: string }) {
	const refunds = state.fraud_refunds
		.filter(
			(refund) => !organizationId || refund.organization_id === organizationId,
		)
		.sort((left, right) => right.updated_at.localeCompare(left.updated_at));
	return (
		<Card variant="outline">
			<CardHeader>
				<Heading level={2}>Fraud refund activity</Heading>
				<CardDescription>
					Outcomes of early fraud warnings. Refunds leave the subscription and
					Anchor license unchanged.
				</CardDescription>
			</CardHeader>
			<CardContent>
				{refunds.length === 0 ? (
					<Empty>
						<EmptyHeader>
							<EmptyTitle>No fraud refund activity yet</EmptyTitle>
							<EmptyDescription>
								Warnings and their refund decisions will appear here, including
								skipped payments.
							</EmptyDescription>
						</EmptyHeader>
					</Empty>
				) : (
					<Stack as="ol" space="md" aria-label="Fraud refund activity">
						{refunds.map((refund) => {
							const status = statusDetails[refund.status];
							return (
								<Box
									as="li"
									key={refund.id}
									className="min-w-0 list-none border-b border-border pb-4 last:border-0 last:pb-0 break-words [overflow-wrap:anywhere]"
								>
									<Stack space="sm">
										<Inline>
											<Badge tone={status.tone}>{status.label}</Badge>
											<Text weight="medium">
												{formatFraudAmount(refund.amount, refund.currency)}
											</Text>
										</Inline>
										{!organizationId && (
											<Text tone="muted">
												{state.organizations.find(
													(item) => item.id === refund.organization_id,
												)?.name ||
													refund.organization_id ||
													"Unlinked organization"}
											</Text>
										)}
										{refund.reason && (
											<Text>
												{reasons[refund.reason] ??
													refund.reason.replaceAll("_", " ")}
											</Text>
										)}
										{refund.last_error && (
											<Text tone="critical">{refund.last_error}</Text>
										)}
										<Text tone="muted" size="xs">
											{formatBillingDate(refund.updated_at)}
										</Text>
										{refund.charge_id && (
											<Text tone="muted" size="xs" font="mono">
												Payment: {refund.charge_id}
											</Text>
										)}
										{refund.refund_id && (
											<Text tone="muted" size="xs" font="mono">
												Refund: {refund.refund_id}
											</Text>
										)}
										{refund.warning_id && (
											<Text tone="muted" size="xs" font="mono">
												Warning: {refund.warning_id}
											</Text>
										)}
									</Stack>
								</Box>
							);
						})}
					</Stack>
				)}
			</CardContent>
		</Card>
	);
}
