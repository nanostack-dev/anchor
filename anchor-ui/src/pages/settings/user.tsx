import { Page } from "@/components/common/Page";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
export default function UserSettingsPage() {
	return (
		<Page
			title="User Settings"
			description="Manage your profile and account settings here."
		>
			<EmptyState
				title="Settings are coming soon"
				description="There are no additional settings to configure yet."
			/>
		</Page>
	);
}
