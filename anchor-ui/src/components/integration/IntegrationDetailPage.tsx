import { Page } from "@/components/common/Page";
import {
	StatCard,
	StatCardLabel,
	StatCardValue,
} from "@nanostackorg/design-system/blocks/stat-card";
import {
	Alert,
	AlertDescription,
} from "@nanostackorg/design-system/components/alert";
import {
	Badge,
	type BadgeTone,
} from "@nanostackorg/design-system/components/badge";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import type { ReactNode } from "react";

export interface IntegrationAuditEntry {
	id: string;
	title: string;
	description: string;
	timestamp: string;
	severity?: "info" | "success" | "warning" | "error";
}
interface IntegrationDetailPageProps {
	title: string;
	description: string;
	backLink: ReactNode;
	summary: { label: string; value: string | number }[];
	children: ReactNode;
	auditEntries: IntegrationAuditEntry[];
	auditIsLoading?: boolean;
	auditErrorMessage?: string | null;
	auditTitle?: string;
	onLoadMoreAudit?: () => void;
	auditIsLoadingMore?: boolean;
	auditLoadMoreErrorMessage?: string | null;
	auditTotal?: number;
	breadCrumbLabels?: Record<string, string>;
}
const auditTone: Record<
	NonNullable<IntegrationAuditEntry["severity"]>,
	BadgeTone
> = { info: "info", success: "success", warning: "warning", error: "critical" };
const summaryColumns: Record<number, string> = {
	1: "md:grid-cols-1",
	2: "md:grid-cols-2",
	3: "md:grid-cols-3",
	4: "md:grid-cols-4",
};
export function IntegrationDetailPage({
	title,
	description,
	backLink,
	summary,
	children,
	auditEntries,
	auditIsLoading = false,
	auditErrorMessage,
	auditTitle = "Audit log",
	onLoadMoreAudit,
	auditIsLoadingMore = false,
	auditLoadMoreErrorMessage = null,
	auditTotal,
	breadCrumbLabels,
}: IntegrationDetailPageProps) {
	const sortedAuditEntries = [...auditEntries].sort(
		(a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime(),
	);
	return (
		<Page
			title={title}
			description={description}
			breadCrumbLabels={breadCrumbLabels}
		>
			<Stack space="lg">
				{backLink}
				{summary.length > 0 && (
					<Box
						className={`grid grid-cols-1 gap-3 sm:grid-cols-2 ${summaryColumns[Math.min(summary.length, 4)] ?? "md:grid-cols-4"}`}
					>
						{summary.map((item) => (
							<StatCard key={item.label} variant="outline" size="sm">
								<StatCardLabel>{item.label}</StatCardLabel>
								<StatCardValue>{item.value}</StatCardValue>
							</StatCard>
						))}
					</Box>
				)}
				{children}
				<Card variant="outline">
					<CardHeader>
						<CardTitle>{auditTitle}</CardTitle>
						<CardDescription>
							Recent integration activity. Most recent events appear first.
						</CardDescription>
					</CardHeader>
					<CardContent>
						{auditIsLoading ? (
							<Inline>
								<Spinner />
								<Text tone="muted">Loading audit activity...</Text>
							</Inline>
						) : auditErrorMessage ? (
							<Alert tone="critical">
								<AlertDescription>{auditErrorMessage}</AlertDescription>
							</Alert>
						) : sortedAuditEntries.length === 0 ? (
							<Text tone="muted">No activity recorded yet.</Text>
						) : (
							<Stack space="md">
								{sortedAuditEntries.map((entry) => (
									<Box
										key={entry.id}
										className="flex flex-wrap items-start justify-between gap-3 border-b pb-3 [overflow-wrap:anywhere] last:border-0 last:pb-0"
									>
										<Stack space="xs">
											<Inline>
												<Badge tone={auditTone[entry.severity ?? "info"]}>
													{entry.severity ?? "info"}
												</Badge>
												<Text weight="medium">{entry.title}</Text>
											</Inline>
											<Text tone="muted" size="xs">
												{entry.description}
											</Text>
										</Stack>
										<Text tone="muted" size="xs">
											{new Date(entry.timestamp).toLocaleString()}
										</Text>
									</Box>
								))}
								{onLoadMoreAudit ? (
									<Spread space="md">
										{auditLoadMoreErrorMessage ? (
											<Text size="xs" tone="critical" role="alert">
												{auditLoadMoreErrorMessage}
											</Text>
										) : (
											<Text size="xs" tone="muted">
												{auditTotal === undefined
													? `Showing ${sortedAuditEntries.length}`
													: `Showing ${sortedAuditEntries.length} of ${auditTotal}`}
											</Text>
										)}
										<Button
											variant="outline"
											size="sm"
											onClick={onLoadMoreAudit}
											disabled={auditIsLoadingMore}
										>
											{auditIsLoadingMore ? "Loading…" : "Load older activity"}
										</Button>
									</Spread>
								) : null}
							</Stack>
						)}
					</CardContent>
				</Card>
			</Stack>
		</Page>
	);
}
