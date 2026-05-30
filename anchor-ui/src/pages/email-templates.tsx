import { Page } from "@/components/common/Page";
import { EmailTemplatesDatatable } from "@/components/email/EmailTemplatesDatatable";

export default function EmailTemplatesPage() {
	return (
		<Page
			title="Email Templates"
			description="Create and manage transactional email templates for this product."
			variant="full"
		>
			<EmailTemplatesDatatable />
		</Page>
	);
}
