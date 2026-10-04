import { Button } from "@nanostackorg/design-system/components/button";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";

export function LicenseAdjustmentBar({
	fields,
	saving,
	onDiscard,
	onSave,
}: {
	fields: string[];
	saving: boolean;
	onDiscard: () => void;
	onSave: () => void;
}) {
	return (
		<Box className="sticky bottom-4 flex flex-wrap items-center justify-between gap-3 rounded-3xl bg-surface-elevated p-4 shadow-lg ring-1 ring-border">
			<Stack space="xxs">
				<Text weight="medium">
					{fields.length} field{fields.length === 1 ? "" : "s"} changed
				</Text>
				<Text size="xs" font="mono" tone="muted">
					{fields.join(", ")}
				</Text>
			</Stack>
			<Inline space="sm">
				<Button variant="ghost" size="sm" onClick={onDiscard} disabled={saving}>
					Discard
				</Button>
				<Button
					variant="solid"
					tone="brand"
					size="sm"
					onClick={onSave}
					loading={saving}
				>
					Adjust this customer
				</Button>
			</Inline>
		</Box>
	);
}
