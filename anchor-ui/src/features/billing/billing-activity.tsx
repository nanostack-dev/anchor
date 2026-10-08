import {
	Badge,
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
import type { State } from "./billing-types";
import { formatBillingDate } from "./organization-workspace";

export function BillingActivity({
	state,
	organizationId,
}: { state: State; organizationId?: string }) {
	const events = state.events
		.filter(
			(event) => !organizationId || event.organization_id === organizationId,
		)
		.sort((left, right) => right.received_at.localeCompare(left.received_at));
	return (
		<Card variant="outline">
			<CardHeader>
				<Heading level={2}>Billing activity</Heading>
				<CardDescription>
					Stripe events and their license synchronization outcomes.
				</CardDescription>
			</CardHeader>
			<CardContent>
				{events.length === 0 ? (
					<Empty>
						<EmptyHeader>
							<EmptyTitle>No billing events yet</EmptyTitle>
							<EmptyDescription>
								Subscription and payment activity will appear here after Stripe
								sends events.
							</EmptyDescription>
						</EmptyHeader>
					</Empty>
				) : (
					<Stack as="ol" space="md" aria-label="Billing activity">
						{events.map((event) => (
							<Box
								as="li"
								key={event.id}
								className="list-none border-b border-border pb-4 last:border-0 last:pb-0 break-words [overflow-wrap:anywhere]"
							>
								<Stack space="sm">
									<Inline>
										<Badge
											tone={
												event.status === "processed"
													? "success"
													: event.status === "error"
														? "critical"
														: event.status === "pending"
															? "warning"
															: "neutral"
											}
										>
											{event.status}
										</Badge>
										<Text weight="medium">{event.type}</Text>
									</Inline>
									{!organizationId && (
										<Text tone="muted">
											{state.organizations.find(
												(organization) =>
													organization.id === event.organization_id,
											)?.name ??
												(event.organization_id || "No organization")}
										</Text>
									)}
									<Text tone="muted" size="xs">
										{formatBillingDate(event.received_at)}
									</Text>
									<Text tone="muted" size="xs" font="mono">
										{event.id}
									</Text>
									{event.last_error && (
										<Text tone="critical">{event.last_error}</Text>
									)}
								</Stack>
							</Box>
						))}
					</Stack>
				)}
			</CardContent>
		</Card>
	);
}
