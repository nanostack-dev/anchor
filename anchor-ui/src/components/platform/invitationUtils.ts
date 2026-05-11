import { registerRoute } from "@/routes/platform/register";

export const generateInvitationLink = (
	tenantId: string,
	email: string,
	invitationId: string,
): string => {
	return `${window.location.origin}/${registerRoute.path}?invitationCode=${encodeURIComponent(invitationId)}&tenantId=${encodeURIComponent(tenantId)}&email=${encodeURIComponent(email)}`;
};
