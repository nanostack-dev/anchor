import { WorkflowParamType } from "@/client";
import {
	listEmailTemplatesOptions,
	listLicenseTemplatesOptions,
	searchProductRolesOptions,
} from "@/client/@tanstack/react-query.gen";
import { useQuery } from "@tanstack/react-query";

export interface ResourceOption {
	value: string;
	label: string;
}

export type WorkflowResources = Partial<
	Record<WorkflowParamType, ResourceOption[]>
>;

const LOOKUP_LIMIT = 100;

export function useWorkflowResources(
	productId: string,
	customEvents: ResourceOption[] = [],
): WorkflowResources {
	const path = { product_id: productId };
	const roles = useQuery({
		...searchProductRolesOptions({
			path,
			body: { pagination: { limit: LOOKUP_LIMIT, offset: 0 } },
		}),
		enabled: !!productId,
	});
	const licenseTemplates = useQuery({
		...listLicenseTemplatesOptions({ path }),
		enabled: !!productId,
		retry: false,
	});
	const emailTemplates = useQuery({
		...listEmailTemplatesOptions({ path, query: { limit: LOOKUP_LIMIT } }),
		enabled: !!productId,
		retry: false,
	});
	return {
		[WorkflowParamType.ROLE]: (roles.data?.items ?? []).map((role) => ({
			value: role.id,
			label: role.name,
		})),
		[WorkflowParamType.LICENSE_TEMPLATE]: (
			licenseTemplates.data?.items ?? []
		).map((template) => ({ value: template.id, label: template.name })),
		[WorkflowParamType.EMAIL_TEMPLATE]: (emailTemplates.data?.items ?? []).map(
			(template) => ({ value: template.slug, label: template.name }),
		),
		[WorkflowParamType.CUSTOM_EVENT]: customEvents,
	};
}
