import { Page } from "@/components/common/Page";
import { WebhookEndpointDatatable } from "@/components/product/webhooks/WebhookEndpointDatatable";
import { useProduct } from "@/hooks/useProduct";

export default function ProductWebhooksPage() {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Page>
				<div className="flex h-64 items-center justify-center">
					<p className="text-muted-foreground">
						Please select a product to manage webhook endpoints.
					</p>
				</div>
			</Page>
		);
	}

	return (
		<Page
			title="Webhooks"
			description={`Deliver ${currentProduct.name} events to your own services. Anchor signs every POST, retries failures across roughly 21 hours and keeps a per-attempt delivery log.`}
			variant="full"
		>
			<WebhookEndpointDatatable productId={currentProduct.id} />
		</Page>
	);
}
