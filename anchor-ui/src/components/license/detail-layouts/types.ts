import type {
	LicenseFieldResponse,
	LicenseFieldUsageResponse,
	LicenseTemplateValues,
	OrganizationLicenseChangeResponse,
} from "@/client";

/**
 * Every candidate layout for the organization license detail takes exactly
 * this, so the comparison between them is about arrangement and lazy loading
 * and nothing else.
 */
export interface LicenseDetailLayoutProps {
	usage: Record<string, LicenseFieldUsageResponse>;
	fields: LicenseFieldResponse[];
	values: LicenseTemplateValues;
	history: OrganizationLicenseChangeResponse[];
	templateName: (templateId: string) => string;
	/**
	 * Fired the first time a limit's series is actually needed. In the shipped
	 * page this is the query; here it records that nothing was fetched for the
	 * other nineteen limits.
	 */
	onLoadSeries?: (field: string) => void;
	/** Fired the first time the change history is actually needed. */
	onLoadHistory?: () => void;
}
