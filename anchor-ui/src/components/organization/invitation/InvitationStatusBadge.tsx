import { OrganizationInvitationStatus } from "@/client";
import { StatusBadge, type StatusTone } from "@/components/common/StatusBadge";

const statusPresentation: Record<
	OrganizationInvitationStatus,
	{ label: string; tone: StatusTone }
> = {
	[OrganizationInvitationStatus.PENDING]: { label: "Pending", tone: "warning" },
	[OrganizationInvitationStatus.ACCEPTED]: {
		label: "Accepted",
		tone: "success",
	},
	[OrganizationInvitationStatus.EXPIRED]: { label: "Expired", tone: "neutral" },
};

export const invitationStatusOptions = Object.values(
	OrganizationInvitationStatus,
).map((status) => ({
	value: status,
	label: statusPresentation[status].label,
}));

export function InvitationStatusBadge({
	status,
}: { status: OrganizationInvitationStatus }) {
	const { label, tone } = statusPresentation[status];
	return <StatusBadge tone={tone}>{label}</StatusBadge>;
}
