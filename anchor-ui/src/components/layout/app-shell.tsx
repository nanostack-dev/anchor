import {
	AppShellBrand,
	AppShellContent,
	AppShellHeader,
	AppShellInset,
	type AppShellProps,
	AppShellSidebar,
	AppShell as SharedAppShell,
} from "@nanostackorg/design-system/blocks/app-shell";

export function AppShell(
	props: Pick<
		AppShellProps,
		"children" | "defaultOpen" | "open" | "onOpenChange"
	>,
) {
	return (
		<SharedAppShell
			sidebarWidth="lg"
			sidebarIconWidth="lg"
			skipLinkLabel="Skip to Main Content"
			mainId="app-shell-content"
			{...props}
		/>
	);
}

export {
	AppShellBrand,
	AppShellContent,
	AppShellHeader,
	AppShellInset,
	AppShellSidebar,
};
