import { Page } from "@/components/common/Page";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { GearIcon } from "@phosphor-icons/react";

export default function AppSettingsPage() {
	return (
		<Page
			title="Application Settings"
			description="Manage application-wide settings here."
		>
			<Card>
				<CardContent>
					<EmptyState
						icon={GearIcon}
						title="Settings are coming soon"
						description="There are no additional settings to configure yet."
					/>
				</CardContent>
			</Card>
		</Page>
	);
}
