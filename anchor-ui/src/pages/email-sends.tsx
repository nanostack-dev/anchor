import { Page } from "@/components/common/Page";
import { EmailSendsDatatable } from "@/components/email/EmailSendsDatatable";

export default function EmailSendsPage() {
  return (
    <Page
      title="Send History"
      description="View all email send records and their delivery status for this product."
      variant="full"
    >
      <EmailSendsDatatable />
    </Page>
  );
}
