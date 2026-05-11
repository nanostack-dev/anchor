import { Page } from "@/components/common/Page";

export default function AppSettingsPage() {
	return (
		<Page>
			<h1 className="text-2xl font-bold mb-4">Application Settings</h1>
			<div>
				<p className="text-muted-foreground">
					Manage application-wide settings here.
				</p>
				{/* Add application settings form or content here */}
			</div>
		</Page>
	);
}
