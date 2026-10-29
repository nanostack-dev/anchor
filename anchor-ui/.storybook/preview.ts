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
			test: "error",
			config: {
				rules: [
					{
						// Base UI's focus guards are aria-hidden with tabindex 0 by
						// design: they catch Tab to keep focus inside an open popup.
						// axe flags them whenever a check lands while one is mounted.
						id: "aria-hidden-focus",
						selector: '[aria-hidden="true"]:not([data-base-ui-focus-guard])',
					},
				],
			},
		},
	},
};

export default preview;
