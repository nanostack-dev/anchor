import { Toaster } from "@nanostackorg/design-system/components/toast";
import { DesignSystemProvider } from "@nanostackorg/design-system/provider";
import type { Preview } from "@storybook/react-vite";
import { createElement } from "react";

import "../src/styles.css";

/**
 * anchor-ui is light-only — there is no theme toolbar here on purpose, and
 * stories must never author `dark:` classes. See `anchor-ui/AGENTS.md`.
 */
const preview: Preview = {
	decorators: [
		(Story) =>
			createElement(
				DesignSystemProvider,
				null,
				createElement(Toaster, null, createElement(Story)),
			),
	],
	parameters: {
		layout: "centered",
		controls: {
			matchers: {
				color: /(background|color)$/i,
				date: /Date$/i,
			},
		},
		a11y: {
			// Report violations without failing the run. Promote to "error" once
			// the owned components are clean.
			test: "error",
		},
	},
};

export default preview;
