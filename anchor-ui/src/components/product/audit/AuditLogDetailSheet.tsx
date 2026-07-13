import { AuditLogOutcome, type AuditLogResponse } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Separator } from "@/components/ui/separator";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
} from "@/components/ui/sheet";
import dayjs from "dayjs";

interface AuditLogDetailSheetProps {
	entry: AuditLogResponse | null;
	onClose: () => void;
}

function DetailRow({
	label,
	value,
}: {
	label: string;
	value: React.ReactNode;
}) {
	if (value === null || value === undefined || value === "") return null;
	return (
		<div className="grid grid-cols-[130px_1fr] gap-2 py-1.5 text-sm">
			<span className="text-muted-foreground">{label}</span>
			<span className="break-all">{value}</span>
		</div>
	);
}

export function AuditLogDetailSheet({
	entry,
	onClose,
}: AuditLogDetailSheetProps) {
	return (
		<Sheet open={!!entry} onOpenChange={(open) => !open && onClose()}>
			<SheetContent side="right" className="sm:max-w-lg overflow-y-auto">
				{entry && (
					<>
						<SheetHeader>
							<SheetTitle>
								<code className="rounded bg-muted px-1.5 py-0.5 text-sm">
									{entry.action}
								</code>
							</SheetTitle>
							<SheetDescription>
								{dayjs(entry.created_at).format("D MMMM YYYY H:mm:ss")}
							</SheetDescription>
						</SheetHeader>
						<div className="px-4 pb-6 space-y-4">
							<div>
								<StatusBadge
									tone={
										entry.outcome === AuditLogOutcome.SUCCESS
											? "success"
											: "destructive"
									}
								>
									{entry.outcome}
								</StatusBadge>
							</div>
							<Separator />
							<div>
								<h4 className="text-sm font-medium mb-1">Actor</h4>
								<DetailRow label="Type" value={entry.actor_type} />
								<DetailRow label="Name" value={entry.actor_name} />
								<DetailRow
									label="ID"
									value={
										entry.actor_id && (
											<code className="text-xs">{entry.actor_id}</code>
										)
									}
								/>
							</div>
							<Separator />
							<div>
								<h4 className="text-sm font-medium mb-1">Target</h4>
								<DetailRow label="Type" value={entry.target_type} />
								<DetailRow label="Name" value={entry.target_name} />
								<DetailRow
									label="ID"
									value={
										entry.target_id && (
											<code className="text-xs">{entry.target_id}</code>
										)
									}
								/>
								<DetailRow
									label="Organization"
									value={
										entry.organization_id && (
											<code className="text-xs">{entry.organization_id}</code>
										)
									}
								/>
							</div>
							<Separator />
							<div>
								<h4 className="text-sm font-medium mb-1">Context</h4>
								<DetailRow
									label="Entry ID"
									value={<code className="text-xs">{entry.id}</code>}
								/>
								<DetailRow
									label="Request ID"
									value={
										entry.request_id && (
											<code className="text-xs">{entry.request_id}</code>
										)
									}
								/>
							</div>
							{entry.metadata && Object.keys(entry.metadata).length > 0 && (
								<>
									<Separator />
									<div>
										<h4 className="text-sm font-medium mb-2">Metadata</h4>
										<pre className="rounded-md bg-muted p-3 text-xs overflow-x-auto">
											{JSON.stringify(entry.metadata, null, 2)}
										</pre>
									</div>
								</>
							)}
						</div>
					</>
				)}
			</SheetContent>
		</Sheet>
	);
}
