import { Badge } from "@nanostackorg/design-system/components/badge";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { motion, useReducedMotion } from "motion/react";
import type { ReactNode } from "react";
import { AnchorLogoMark } from "./AnchorLogoMark";

export function AnchorAnimatedBackdrop({ children }: { children?: ReactNode }) {
	return (
		<Box className="absolute inset-0 overflow-hidden bg-surface-subtle">
			<Box
				aria-hidden
				className="pointer-events-none absolute -right-24 -top-24 size-96 rounded-full bg-primary/8 blur-3xl"
			/>
			<Box
				aria-hidden
				className="pointer-events-none absolute -bottom-24 -left-24 size-96 rounded-full bg-info/8 blur-3xl"
			/>
			{children}
		</Box>
	);
}
export function FloatingAnchorMark({ size = "lg" }: { size?: "md" | "lg" }) {
	const reducedMotion = useReducedMotion();
	return (
		<motion.div
			initial={reducedMotion ? false : { opacity: 0, y: 12 }}
			animate={{ opacity: 1, y: 0 }}
			transition={{ duration: 0.28, ease: [0.23, 1, 0.32, 1] }}
		>
			<Box className="text-primary">
				<AnchorLogoMark size={size} />
			</Box>
		</motion.div>
	);
}
export function LoginAnimationPanel() {
	return (
		<AnchorAnimatedBackdrop>
			<Box className="flex h-full items-center justify-center p-12 text-center">
				<Stack space="xl" align="center">
					<FloatingAnchorMark />
					<Stack space="sm" align="center">
						<Heading level={2}>Anchor</Heading>
						<Text tone="muted">
							Organizations, identity &amp; access — anchored.
						</Text>
					</Stack>
					<Inline space="sm" align="center">
						<Badge>Identity</Badge>
						<Badge>RBAC</Badge>
						<Badge>Multi-tenancy</Badge>
					</Inline>
				</Stack>
			</Box>
		</AnchorAnimatedBackdrop>
	);
}
