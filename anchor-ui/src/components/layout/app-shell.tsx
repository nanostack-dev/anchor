import {
	AppShellBrand,
	type AppShellProps,
	AppShellSidebar,
	AppShell as SharedAppShell,
} from "@nanostackorg/design-system/blocks/app-shell";
import {
	SidebarInset,
	SidebarTrigger,
} from "@nanostackorg/design-system/components/sidebar";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { ReactNode } from "react";

type ChildrenProps = { children: ReactNode };

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
export function AppShellInset({ children }: ChildrenProps) {
	return <SidebarInset>{children}</SidebarInset>;
}
export function AppShellTopbar({ children }: ChildrenProps) {
	return (
		<Box
			as="header"
			data-slot="app-shell-topbar"
			className="sticky top-0 z-40 flex min-h-16 shrink-0 items-center gap-3 border-b bg-background/85 px-4 py-2 backdrop-blur-md"
		>
			{children}
		</Box>
	);
}
export function AppShellTopbarContent({ children }: ChildrenProps) {
	return (
		<Box
			data-slot="app-shell-topbar-content"
			className="flex min-w-0 flex-1 items-center gap-2"
		>
			{children}
		</Box>
	);
}
export function AppShellTopbarActions({ children }: ChildrenProps) {
	return (
		<Box
			data-slot="app-shell-topbar-actions"
			className="ml-auto flex shrink-0 items-center gap-2"
		>
			{children}
		</Box>
	);
}
export function AppShellSidebarTrigger() {
	return <SidebarTrigger />;
}
export function AppShellContent({ children }: ChildrenProps) {
	return (
		<Box
			id="app-shell-content"
			tabIndex={-1}
			data-slot="app-shell-content"
			className="min-h-0 flex-1 overflow-auto outline-none"
		>
			{children}
		</Box>
	);
}
export { AppShellBrand, AppShellSidebar };
