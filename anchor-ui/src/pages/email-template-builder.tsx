import { EmailTemplateBuilder } from "@/components/email/EmailTemplateBuilder";
import { useProduct } from "@/context/product/ProductContext";
import {
	Empty,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";

interface EmailTemplateBuilderPageProps {
	templateId: string;
}

export default function EmailTemplateBuilderPage({
	templateId,
}: EmailTemplateBuilderPageProps) {
	const { currentProduct } = useProduct();

	if (!currentProduct) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyTitle>Select a product to edit templates</EmptyTitle>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<EmailTemplateBuilder
			productId={currentProduct.id}
			templateId={templateId}
		/>
	);
}
