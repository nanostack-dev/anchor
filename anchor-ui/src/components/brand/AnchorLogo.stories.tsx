import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";
import { AnchorIcon, AnchorLogo } from "./AnchorLogo";

const meta = {
	title: "Brand/Anchor logo",
	component: AnchorLogo,
	parameters: { layout: "centered" },
} satisfies Meta<typeof AnchorLogo>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Sizes: Story = {
	render: () => (
		<Stack space="lg">
			<Inline space="lg">
				<Box className="text-primary">
					<AnchorLogo size="hero" label="Anchor" />
				</Box>
				<Box className="text-primary">
					<AnchorLogo size="xl" />
				</Box>
				<Box className="text-foreground">
					<AnchorLogo size="lg" />
				</Box>
				<Box className="text-foreground">
					<AnchorLogo size="md" />
				</Box>
				<Box className="text-foreground">
					<AnchorLogo size="sm" />
				</Box>
			</Inline>
			<Inline space="md">
				<AnchorIcon size="lg" />
				<AnchorIcon size="md" />
				<AnchorIcon size="sm" />
				<Text as="span">Anchor</Text>
			</Inline>
		</Stack>
	),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByRole("img", { name: "Anchor" })).toBeVisible();
		await expect(
			canvasElement.querySelectorAll('svg[aria-hidden="true"]'),
		).toHaveLength(7);
	},
};
