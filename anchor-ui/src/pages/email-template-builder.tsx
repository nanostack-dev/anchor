import { EmailTemplateBuilder } from "@/components/email/EmailTemplateBuilder";
import { useProduct } from "@/context/product/ProductContext";

interface EmailTemplateBuilderPageProps {
	templateId: string;
}

export default function EmailTemplateBuilderPage({
	templateId,
}: EmailTemplateBuilderPageProps) {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<div className="flex items-center justify-center p-8">
				<p className="text-muted-foreground text-sm">
					Select a product to edit templates
				</p>
			</div>
		);
	}

	return (
		<div className="h-full min-h-0 flex flex-col">
			<EmailTemplateBuilder
				productId={currentProduct.id}
				templateId={templateId}
			/>
		</div>
	);
}
