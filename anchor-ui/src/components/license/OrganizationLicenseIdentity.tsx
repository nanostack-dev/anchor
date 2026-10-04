import { Card, CardContent } from "@nanostackorg/design-system/components/card";
import { Text } from "@nanostackorg/design-system/components/text";
import { Column, Columns } from "@nanostackorg/design-system/layout/columns";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import dayjs from "dayjs";

export function OrganizationLicenseIdentity({
	templateName,
	instantiatedAt,
}: { templateName: string; instantiatedAt: string }) {
	return (
		<Columns collapseBelow="sm" space="lg">
			<Column width="1/2">
				<Card variant="soft" size="sm">
					<CardContent>
						<Stack space="xs">
							<Text size="xs" tone="muted" weight="semibold">
								Template
							</Text>
							<Text weight="medium">{templateName}</Text>
						</Stack>
					</CardContent>
				</Card>
			</Column>
			<Column width="1/2">
				<Card variant="soft" size="sm">
					<CardContent>
						<Stack space="xs">
							<Text size="xs" tone="muted" weight="semibold">
								Instantiated
							</Text>
							<Text>
								<time dateTime={instantiatedAt}>
									{dayjs(instantiatedAt).format("D MMMM YYYY H:mm")}
								</time>
							</Text>
						</Stack>
					</CardContent>
				</Card>
			</Column>
		</Columns>
	);
}
