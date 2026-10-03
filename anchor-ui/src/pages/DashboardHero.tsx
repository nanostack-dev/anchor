import {
	PageHeader,
	PageHeaderContent,
	PageHeaderDescription,
	PageHeaderTitle,
} from "@nanostackorg/design-system/blocks/page-header";
type DashboardHeroProps = { subtitle: string };
export function DashboardHero({ subtitle }: DashboardHeroProps) {
	return (
		<PageHeader>
			<PageHeaderContent>
				<PageHeaderTitle>Dashboard</PageHeaderTitle>
				<PageHeaderDescription>{subtitle}</PageHeaderDescription>
			</PageHeaderContent>
		</PageHeader>
	);
}
