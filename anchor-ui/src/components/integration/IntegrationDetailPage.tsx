import { Page } from "@/components/common/Page";
import { Badge } from "@/components/ui/badge";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { Spinner } from "@/components/ui/spinner";
import { Clock3 } from "lucide-react";
import type { ReactNode } from "react";

export interface IntegrationAuditEntry {
	id: string;
	title: string;
	description: string;
	timestamp: string;
	severity?: "info" | "success" | "warning" | "error";
}

interface SummaryItem {
	label: string;
	value: string | number;
}

interface IntegrationDetailPageProps {
	title: string;
	description: string;
	backLink: ReactNode;
	summary: SummaryItem[];
	children: ReactNode;
	auditEntries: IntegrationAuditEntry[];
	auditIsLoading?: boolean;
	auditErrorMessage?: string | null;
	auditTitle?: string;
}

function severityClassName(
	severity: IntegrationAuditEntry["severity"],
): string {
	switch (severity) {
		case "success":
			return "bg-success";
		case "warning":
			return "bg-warning";
		case "error":
			return "bg-destructive";
		default:
			return "bg-muted-foreground";
	}
}

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
}: IntegrationDetailPageProps) {
	const sortedAuditEntries = [...auditEntries].sort((a, b) => {
		return new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime();
	});

	return (
		<Page title={title} description={description}>
			<div className="space-y-6 pb-6">
				<div className="space-y-3">
					<div>{backLink}</div>
					{summary.length > 0 ? (
						<div className="grid grid-cols-2 gap-3 md:grid-cols-4">
							{summary.map((item) => (
								<div key={item.label} className="rounded-xl border bg-card p-3">
									<p className="text-xs text-muted-foreground">{item.label}</p>
									<p className="text-sm font-medium">{item.value}</p>
								</div>
							))}
						</div>
					) : null}
				</div>

				{children}

				<Card>
					<CardHeader>
						<CardTitle className="inline-flex items-center gap-2">
							<Clock3 className="size-4" />
							{auditTitle}
						</CardTitle>
						<CardDescription>
							Recent integration activity. Most recent events appear first.
						</CardDescription>
					</CardHeader>
					<CardContent>
						{auditIsLoading ? (
							<div className="flex items-center gap-2 rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
								<Spinner className="size-4 text-current" />
								Loading audit activity...
							</div>
						) : auditErrorMessage ? (
							<div className="rounded-xl border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
								{auditErrorMessage}
							</div>
						) : sortedAuditEntries.length === 0 ? (
							<div className="rounded-xl border border-dashed p-6 text-sm text-muted-foreground">
								No activity recorded yet.
							</div>
						) : (
							<div className="space-y-3">
								{sortedAuditEntries.map((entry) => (
									<div className="rounded-xl border bg-card p-3" key={entry.id}>
										<div className="flex items-start justify-between gap-3">
											<div className="flex items-start gap-2">
												<span
													className={`mt-1 size-2.5 shrink-0 rounded-full ${severityClassName(entry.severity)}`}
												/>
												<div>
													<p className="text-sm font-medium">{entry.title}</p>
													<p className="text-xs text-muted-foreground">
														{entry.description}
													</p>
												</div>
											</div>
											<Badge variant="secondary" className="text-[10px]">
												{new Date(entry.timestamp).toLocaleString()}
											</Badge>
										</div>
									</div>
								))}
							</div>
						)}
					</CardContent>
				</Card>
			</div>
		</Page>
	);
}
