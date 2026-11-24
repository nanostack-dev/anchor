import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { ReactNode } from "react";

/** Helper copy under a license section: readable size, capped line length. */
export function LicenseSectionIntro({ children }: { children: ReactNode }) {
	return (
		<Box className="max-w-prose">
			<Text size="sm" tone="muted">
				{children}
			</Text>
		</Box>
	);
}
