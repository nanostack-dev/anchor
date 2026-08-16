import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
	organizationLicenseChangesRoute,
	organizationLicenseUsageRoute,
	organizationLicenseValuesRoute,
} from "@/routes/organizations/organization-license.$organizationId";
import { Link, useLocation, useNavigate } from "@tanstack/react-router";

/**
 * Built per render rather than at module scope. The route module imports the
 * page that renders this component, so at import time these route objects are
 * still in their temporal dead zone.
 */
const licenseTabs = () =>
	[
		{ to: organizationLicenseUsageRoute.to, segment: "usage", label: "Usage" },
		{
			to: organizationLicenseChangesRoute.to,
			segment: "changes",
			label: "Changes",
		},
		{
			to: organizationLicenseValuesRoute.to,
			segment: "values",
			label: "Values",
		},
	] as const;

interface OrganizationLicenseTabsProps {
	organizationId: string;
}

/**
 * Each tab is a real link to its own route, so a colleague can be sent
 * straight to the change history and the browser's back button means what it
 * looks like. The tab semantics stay, which is what keyboard users navigate
 * by, so arrow keys move through the same three destinations.
 */
export function OrganizationLicenseTabs({
	organizationId,
}: OrganizationLicenseTabsProps) {
	const navigate = useNavigate();
	const { pathname } = useLocation();
	const tabs = licenseTabs();
	const segment = pathname.split("/").pop();
	const active = tabs.find((tab) => tab.segment === segment)?.to ?? tabs[0].to;

	return (
		<Tabs
			value={active}
			onValueChange={(value) => {
				const tab = tabs.find((candidate) => candidate.to === value);
				if (tab) navigate({ to: tab.to, params: { organizationId } });
			}}
		>
			<TabsList variant="line">
				{tabs.map((tab) => (
					<TabsTrigger
						key={tab.to}
						value={tab.to}
						nativeButton={false}
						render={<Link to={tab.to} params={{ organizationId }} />}
					>
						{tab.label}
					</TabsTrigger>
				))}
			</TabsList>
		</Tabs>
	);
}
