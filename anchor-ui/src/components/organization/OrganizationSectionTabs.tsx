import { ROUTE_PATHS } from "@/routes/routePaths";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Inline } from "@nanostackorg/design-system/layout/inline";

export type OrganizationSection = "license" | "roles";

const sections: { id: OrganizationSection; label: string; path: string }[] = [
	{
		id: "license",
		label: "License",
		path: ROUTE_PATHS.ORGANIZATION_LICENSE_DETAIL,
	},
	{ id: "roles", label: "Roles", path: ROUTE_PATHS.ORGANIZATION_ROLES },
];

export function OrganizationSectionTabs({
	organizationId,
	active,
}: {
	organizationId: string;
	active: OrganizationSection;
}) {
	return (
		<Inline as="nav" space="xs" aria-label="Organization sections">
			{sections.map((section) => (
				<ButtonLink
					key={section.id}
					href={section.path.replace(
						"$organizationId",
						encodeURIComponent(organizationId),
					)}
					variant={active === section.id ? "solid" : "outline"}
					tone={active === section.id ? "brand" : "neutral"}
					size="sm"
					aria-current={active === section.id ? "page" : undefined}
				>
					{section.label}
				</ButtonLink>
			))}
		</Inline>
	);
}
