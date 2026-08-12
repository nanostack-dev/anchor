import type { LicenseFieldResponse, LicenseTemplateResponse } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import dayjs from "dayjs";
import { type ReactElement, useState } from "react";
import { LicenseValueFields } from "./LicenseValueFields";

interface LicenseTemplateViewDialogProps {
	template: LicenseTemplateResponse;
	fields: LicenseFieldResponse[];
	trigger: ReactElement;
}

/** Read-only detail view of a single license template. */
export function LicenseTemplateViewDialog({
	template,
	fields,
	trigger,
}: LicenseTemplateViewDialogProps) {
	const [open, setOpen] = useState(false);

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent className="flex max-h-[90vh] flex-col p-0 sm:max-w-[600px]">
				<DialogHeader className="px-6 pt-6">
					<div className="flex items-center gap-2">
						<DialogTitle>{template.name}</DialogTitle>
						<StatusBadge
							tone={template.status === "ACTIVE" ? "success" : "neutral"}
						>
							{template.status}
						</StatusBadge>
					</div>
					<DialogDescription>
						{template.description || "No description."}
					</DialogDescription>
				</DialogHeader>

				<div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
					<p className="text-xs text-muted-foreground">
						Created {dayjs(template.created_at).format("D MMMM YYYY H:mm")} ·
						Updated {dayjs(template.updated_at).format("D MMMM YYYY H:mm")}
					</p>
					<LicenseValueFields fields={fields} values={template.values} />
				</div>
			</DialogContent>
		</Dialog>
	);
}
