/**
 * Bare specifiers that must be pre-bundled before the Storybook browser tests run.
 *
 * Why this file exists
 * -------------------
 * If Vite discovers a bare specifier *during* a test run it re-optimizes and
 * reloads the page, and vitest reports "Vite unexpectedly reloaded a test". The
 * story file executing at that moment either collects 0 tests or fails outright.
 * Which file loses depends on story ordering, so it presents as a random flake —
 * and CI, always starting cold, is where it bites hardest.
 *
 * Observed on the first run in this repo: `@tanstack/react-router` reaching the
 * stories through `StoryRouter` re-optimized mid-run and took out 13 of 25 tests
 * across three files. The same run passed cleanly on a warm cache, which is
 * exactly how this failure mode disguises itself as flake.
 *
 * Why it is shared
 * ---------------
 * `storybookTest` merges the Storybook config into the vitest project, and
 * `viteFinal` appends to the config *Storybook* builds, which does not contain
 * vitest's own `optimizeDeps.include`. A list kept only in `vitest.workspace.ts`
 * is therefore silently replaced. Both sides read this one constant so the
 * clobber is harmless. (echopoint learned this the hard way — its two lists had
 * drifted to 8 and 14 entries, and only the Storybook one was taking effect.)
 *
 * Maintaining it
 * -------------
 * `optimizeDeps` matches exact specifiers, so a subpath import needs its own
 * entry — listing `@base-ui/react` would not cover `@base-ui/react/checkbox`.
 * If a run logs "new dependencies optimized: X", X belongs in this list.
 */
export const OPTIMIZE_DEPS_INCLUDE = [
	// React core
	"react",
	"react/jsx-runtime",
	"react/jsx-dev-runtime",
	"react-dom",
	"react-dom/client",

	// Storybook runtime reachable from stories and the preview
	"@storybook/react-vite",
	"storybook/test",

	// Base UI — subpath imports, so each needs naming explicitly. This is the
	// full set used by src/components/ui, because any owned component may
	// compose any primitive.
	"@base-ui/react/accordion",
	"@base-ui/react/alert-dialog",
	"@base-ui/react/avatar",
	"@base-ui/react/button",
	"@base-ui/react/checkbox",
	"@base-ui/react/collapsible",
	"@base-ui/react/context-menu",
	"@base-ui/react/dialog",
	"@base-ui/react/drawer",
	"@base-ui/react/input",
	"@base-ui/react/menu",
	"@base-ui/react/menubar",
	"@base-ui/react/merge-props",
	"@base-ui/react/navigation-menu",
	"@base-ui/react/popover",
	"@base-ui/react/preview-card",
	"@base-ui/react/progress",
	"@base-ui/react/radio",
	"@base-ui/react/radio-group",
	"@base-ui/react/scroll-area",
	"@base-ui/react/select",
	"@base-ui/react/separator",
	"@base-ui/react/slider",
	"@base-ui/react/switch",
	"@base-ui/react/tabs",
	"@base-ui/react/toggle",
	"@base-ui/react/toggle-group",
	"@base-ui/react/tooltip",
	"@base-ui/react/use-render",

	// TanStack
	"@tanstack/react-form",
	"@tanstack/react-query",
	"@tanstack/react-router",
	"@tanstack/react-table",

	// UI primitives and utilities reached from owned components
	"class-variance-authority",
	"clsx",
	"cmdk",
	"date-fns",
	"lucide-react",
	"motion/react",
	"sonner",
	"tailwind-merge",
	"zod",
];
