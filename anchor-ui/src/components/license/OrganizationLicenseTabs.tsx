import { ROUTE_PATHS } from "@/routes/routePaths";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { useLocation } from "@tanstack/react-router";

const licenseTabs = [
	{ segment: "usage", label: "Usage" },
	{ segment: "changes", label: "Changes" },
	{ segment: "values", label: "Values" },
	{ segment: "billing", label: "Billing" },
] as const;

export function OrganizationLicenseTabs({
	organizationId,
}: { organizationId: string }) {
	const { pathname } = useLocation();
	const segment = pathname.split("/").pop();
	const activeSegment = licenseTabs.some((tab) => tab.segment === segment)
		? segment
		: "usage";
	return (
		<Inline as="nav" space="xs" aria-label="License sections">
			{licenseTabs.map((tab) => (
				<ButtonLink
					key={tab.segment}
					href={`${ROUTE_PATHS.ORGANIZATION_LICENSE_DETAIL.replace("$organizationId", encodeURIComponent(organizationId))}/${tab.segment}`}
					variant={activeSegment === tab.segment ? "soft" : "ghost"}
					tone={activeSegment === tab.segment ? "brand" : "neutral"}
					aria-current={activeSegment === tab.segment ? "page" : undefined}
				>
					{tab.label}
				</ButtonLink>
			))}
		</Inline>
	);
}
