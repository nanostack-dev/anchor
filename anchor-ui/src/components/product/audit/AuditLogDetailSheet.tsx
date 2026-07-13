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
import * as React from "react";

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
	// Keep rendering the last entry while the sheet's exit transition runs,
	// otherwise the panel goes blank mid-animation.
	const lastEntryRef = React.useRef<AuditLogResponse | null>(null);
	if (entry) {
		lastEntryRef.current = entry;
	}
	const shownEntry = entry ?? lastEntryRef.current;

	return (
		<Sheet open={!!entry} onOpenChange={(open) => !open && onClose()}>
			<SheetContent side="right" className="sm:max-w-lg overflow-y-auto">
				{shownEntry && (
					<>
						<SheetHeader>
							<SheetTitle>
								<code className="rounded bg-muted px-1.5 py-0.5 text-sm">
									{shownEntry.action}
								</code>
							</SheetTitle>
							<SheetDescription>
								{dayjs(shownEntry.created_at).format("D MMMM YYYY H:mm:ss")}
							</SheetDescription>
						</SheetHeader>
						<div className="px-4 pb-6 space-y-4">
							<div>
								<StatusBadge
									tone={
										shownEntry.outcome === AuditLogOutcome.SUCCESS
											? "success"
											: "destructive"
									}
								>
									{shownEntry.outcome}
								</StatusBadge>
							</div>
							<Separator />
							<div>
								<h4 className="text-sm font-medium mb-1">Actor</h4>
								<DetailRow label="Type" value={shownEntry.actor_type} />
								<DetailRow label="Name" value={shownEntry.actor_name} />
								<DetailRow
									label="ID"
									value={
										shownEntry.actor_id && (
											<code className="text-xs">{shownEntry.actor_id}</code>
										)
									}
								/>
							</div>
							<Separator />
							<div>
								<h4 className="text-sm font-medium mb-1">Target</h4>
								<DetailRow label="Type" value={shownEntry.target_type} />
								<DetailRow label="Name" value={shownEntry.target_name} />
								<DetailRow
									label="ID"
									value={
										shownEntry.target_id && (
											<code className="text-xs">{shownEntry.target_id}</code>
										)
									}
								/>
								<DetailRow
									label="Organization"
									value={
										shownEntry.organization_id && (
											<code className="text-xs">
												{shownEntry.organization_id}
											</code>
										)
									}
								/>
							</div>
							<Separator />
							<div>
								<h4 className="text-sm font-medium mb-1">Context</h4>
								<DetailRow
									label="Entry ID"
									value={<code className="text-xs">{shownEntry.id}</code>}
								/>
								<DetailRow
									label="Request ID"
									value={
										shownEntry.request_id && (
											<code className="text-xs">{shownEntry.request_id}</code>
										)
									}
								/>
							</div>
							{shownEntry.metadata &&
								Object.keys(shownEntry.metadata).length > 0 && (
									<>
										<Separator />
										<div>
											<h4 className="text-sm font-medium mb-2">Metadata</h4>
											<pre className="rounded-md bg-muted p-3 text-xs overflow-x-auto">
												{JSON.stringify(shownEntry.metadata, null, 2)}
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
