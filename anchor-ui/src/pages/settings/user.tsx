import { Page } from "@/components/common/Page";
import { EmptyState } from "@nanostackorg/design-system/blocks/empty-state";
import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { UserIcon } from "@phosphor-icons/react";

export default function UserSettingsPage() {
	return (
		<Page
			title="User Settings"
			description="Manage your profile and account settings here."
		>
			<Card>
				<CardContent>
					<EmptyState
						icon={UserIcon}
						title="Settings are coming soon"
						description="There are no additional settings to configure yet."
					/>
				</CardContent>
			</Card>
		</Page>
	);
}
