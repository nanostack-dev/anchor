import { Page } from "@/components/common/Page";
import { PlatformUserDatatable } from "@/components/platform/PlatformUserDatatable";

export default function PlatformUsersPage() {
	return (
		<Page
			title={"Platform Users"}
			description={
				"Manage and review platform-level users. Search, filter, and take actions."
			}
		>
			<PlatformUserDatatable />
		</Page>
	);
}
