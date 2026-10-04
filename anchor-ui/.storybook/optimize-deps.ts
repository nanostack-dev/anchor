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
	"@nanostackorg/design-system/blocks/app-shell",
	"@nanostackorg/design-system/blocks/copy-button",
	"@nanostackorg/design-system/blocks/empty-state",
	"@nanostackorg/design-system/blocks/page-header",
	"@nanostackorg/design-system/blocks/stat-card",
	"@nanostackorg/design-system/components/alert",
	"@nanostackorg/design-system/components/alert-dialog",
	"@nanostackorg/design-system/components/avatar",
	"@nanostackorg/design-system/components/badge",
	"@nanostackorg/design-system/components/breadcrumb",
	"@nanostackorg/design-system/components/button",
	"@nanostackorg/design-system/components/calendar",
	"@nanostackorg/design-system/components/card",
	"@nanostackorg/design-system/components/chart",
	"@nanostackorg/design-system/components/checkbox",
	"@nanostackorg/design-system/components/command",
	"@nanostackorg/design-system/components/dialog",
	"@nanostackorg/design-system/components/dropdown-menu",
	"@nanostackorg/design-system/components/empty",
	"@nanostackorg/design-system/components/field",
	"@nanostackorg/design-system/components/heading",
	"@nanostackorg/design-system/components/input",
	"@nanostackorg/design-system/components/input-group",
	"@nanostackorg/design-system/components/label",
	"@nanostackorg/design-system/components/popover",
	"@nanostackorg/design-system/components/progress",
	"@nanostackorg/design-system/components/scroll-area",
	"@nanostackorg/design-system/components/select",
	"@nanostackorg/design-system/components/separator",
	"@nanostackorg/design-system/components/sidebar",
	"@nanostackorg/design-system/components/skeleton",
	"@nanostackorg/design-system/components/spinner",
	"@nanostackorg/design-system/components/switch",
	"@nanostackorg/design-system/components/table",
	"@nanostackorg/design-system/components/tabs",
	"@nanostackorg/design-system/components/text",
	"@nanostackorg/design-system/components/text-link",
	"@nanostackorg/design-system/components/textarea",
	"@nanostackorg/design-system/components/toggle-group",
	"@nanostackorg/design-system/components/tooltip",
	"@nanostackorg/design-system/layout/box",
	"@nanostackorg/design-system/layout/columns",
	"@nanostackorg/design-system/layout/inline",
	"@nanostackorg/design-system/layout/spread",
	"@nanostackorg/design-system/layout/stack",
	"@monaco-editor/react",

	// React core
	"react",
	"react/jsx-runtime",
	"react/jsx-dev-runtime",
	"react-dom",
	"react-dom/client",

	// Storybook runtime reachable from stories and the preview
	"@storybook/react-vite",
	"storybook/test",

	// TanStack
	"@tanstack/react-form",
	"@tanstack/react-query",
	"@tanstack/react-router",
	"@tanstack/react-table",
	"@uidotdev/usehooks",

	// UI primitives and utilities reached from owned components
	"clsx",
	"date-fns",
	"dayjs",
	"dayjs/plugin/relativeTime",
	"lucide-react",
	"motion/react",
	"recharts",
	"@nanostackorg/design-system",
	"@nanostackorg/design-system/provider",
	"@nanostackorg/design-system/components/toast",
	"@phosphor-icons/react",
	"tailwind-merge",
	"web-vitals",
	"zod",
];
