import { Page } from "@/components/common/Page";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
export default function AppSettingsPage() {
	return (
		<Page
			title="Application Settings"
			description="Manage application-wide settings here."
		>
			<EmptyState
				title="Settings are coming soon"
				description="There are no additional settings to configure yet."
			/>
		</Page>
	);
}
