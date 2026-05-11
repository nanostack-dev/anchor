import { Page } from "@/components/common/Page";

export default function DashboardPage() {
	return (
		<div className="space-y-12">
			<Page
				variant="default"
				title="Page Variant: Default"
				description="This is the default variant."
			>
				<div className="p-4">
					<p>
						The <b>default</b> variant uses a standard width and layout.
					</p>
				</div>
			</Page>
			<Page
				variant="wide"
				title="Page Variant: Wide"
				description="This is the wide variant."
			>
				<div className="p-4">
					<p>
						The <b>wide</b> variant allows more horizontal space for content.
					</p>
				</div>
			</Page>
			<Page
				variant="narrow"
				title="Page Variant: Narrow"
				description="This is the narrow variant."
			>
				<div className="p-4">
					<p>
						The <b>narrow</b> variant restricts the content width for focused
						layouts.
					</p>
				</div>
			</Page>
		</div>
	);
}
