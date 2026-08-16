import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import type { ReactElement } from "react";
import { OrganizationLicensePanel } from "./OrganizationLicensePanel";

interface OrganizationLicenseViewDialogProps {
	productId: string;
	organizationId: string;
	organizationName: string;
	trigger: ReactElement;
}

/**
 * One organization's license, read from the row it sits on. The panel derives
 * usage on its own read, which the list deliberately does not carry.
 */
export function OrganizationLicenseViewDialog({
	productId,
	organizationId,
	organizationName,
	trigger,
}: OrganizationLicenseViewDialogProps) {
	return (
		<Dialog>
			<DialogTrigger render={trigger} />
			<DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-3xl">
				<DialogHeader>
					<DialogTitle>{organizationName}</DialogTitle>
					<DialogDescription>
						What this organization is allowed, how much of each limit it has
						used, and its usage history.
					</DialogDescription>
				</DialogHeader>
				<OrganizationLicensePanel
					productId={productId}
					organizationId={organizationId}
				/>
			</DialogContent>
		</Dialog>
	);
}
