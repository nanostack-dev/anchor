import type { Preview } from "@storybook/react-vite";

import "../src/styles.css";

/**
 * anchor-ui is light-only — there is no theme toolbar here on purpose, and
 * stories must never author `dark:` classes. See `anchor-ui/AGENTS.md`.
 */
const preview: Preview = {
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
			test: "todo",
		},
	},
};

export default preview;
