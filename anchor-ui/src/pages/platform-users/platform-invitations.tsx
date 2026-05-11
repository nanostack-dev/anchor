import { Page } from "@/components/common/Page";
import { PlatformInvitationDatatable } from "@/components/platform/PlatformInvitationDatatable";

export default function PlatformInvitationsPage() {
	return (
		<Page
			title={"Platform Invitations"}
			description={
				"Manage and review platform-level invitations. Search, filter, and take actions."
			}
		>
			<PlatformInvitationDatatable />
		</Page>
	);
}
