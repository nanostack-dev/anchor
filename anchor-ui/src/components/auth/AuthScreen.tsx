import { TextLink } from "@nanostackorg/design-system/components/text-link";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { ReactNode } from "react";
import { AnchorLogoMark } from "./AnchorLogoMark";
import { LoginAnimationPanel } from "./LoginAnimationPanel";

export function AuthScreen({ children }: { children: ReactNode }) {
	return (
		<Box className="grid min-h-svh lg:grid-cols-2">
			<Box className="relative flex flex-col items-center justify-center gap-8 p-6 md:p-10">
				<Box className="self-start">
					<TextLink href="/" aria-label="Anchor home">
						<AnchorLogoMark size="sm" />
						Anchor
					</TextLink>
				</Box>
				<Box className="grid w-full max-w-lg flex-1 content-center">
					{children}
				</Box>
			</Box>
			<Box className="relative hidden overflow-hidden lg:block">
				<LoginAnimationPanel />
			</Box>
		</Box>
	);
}
