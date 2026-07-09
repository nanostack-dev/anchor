import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import type * as React from "react";

import {
	Sidebar,
	SidebarInset,
	SidebarProvider,
	SidebarTrigger,
} from "@/components/ui/sidebar";
import { TooltipProvider } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

/**
 * Nanostack application shell. Mirrors the shared Nanostack `AppShell`
 * composition so Anchor renders the same shell experience while keeping
 * product/tenant behavior local.
 */
type AppShellProps = React.ComponentProps<typeof SidebarProvider> & {
	skipLinkHref?: string | null;
};

function AppShell({
	children,
	className,
	skipLinkHref = "#app-shell-content",
	style,
	...props
}: AppShellProps) {
	return (
		<SidebarProvider
			data-slot="app-shell"
			style={
				{
					"--sidebar-width": "15.25rem",
					"--sidebar-width-icon": "3.25rem",
					...style,
				} as React.CSSProperties
			}
			className={cn("bg-background text-foreground min-h-svh", className)}
			{...props}
		>
			{skipLinkHref ? (
				<a
					href={skipLinkHref}
					className="focus:border-border focus:bg-popover focus:text-popover-foreground focus:ring-ring/30 sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:inline-flex focus:h-9 focus:items-center focus:rounded-4xl focus:border focus:px-3 focus:text-sm focus:font-medium focus:shadow-lg focus:ring-3 focus:outline-none"
				>
					Skip to Main Content
				</a>
			) : null}
			<TooltipProvider>{children}</TooltipProvider>
		</SidebarProvider>
	);
}

function AppShellSidebar({
	collapsible = "icon",
	variant = "sidebar",
	className,
	...props
}: React.ComponentProps<typeof Sidebar>) {
	return (
		<Sidebar
			collapsible={collapsible}
			variant={variant}
			className={className}
			{...props}
		/>
	);
}

function AppShellInset({
	className,
	...props
}: React.ComponentProps<typeof SidebarInset>) {
	return (
		<SidebarInset
			data-slot="app-shell-inset"
			className={cn("min-w-0 overflow-hidden", className)}
			{...props}
		/>
	);
}

function AppShellTopbar({
	className,
	...props
}: React.ComponentProps<"header">) {
	return (
		<header
			data-slot="app-shell-topbar"
			className={cn(
				"bg-background/85 backdrop-blur-md sticky top-0 z-40 flex min-h-16 shrink-0 items-center gap-3 border-b px-4 py-2",
				className,
			)}
			{...props}
		/>
	);
}

function AppShellTopbarContent({
	className,
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="app-shell-topbar-content"
			className={cn("flex min-w-0 flex-1 items-center gap-2", className)}
			{...props}
		/>
	);
}

function AppShellTopbarActions({
	className,
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			data-slot="app-shell-topbar-actions"
			className={cn("ml-auto flex shrink-0 items-center gap-2", className)}
			{...props}
		/>
	);
}

function AppShellSidebarTrigger({
	className,
	...props
}: React.ComponentProps<typeof SidebarTrigger>) {
	return <SidebarTrigger className={className} {...props} />;
}

function AppShellContent({
	className,
	id = "app-shell-content",
	...props
}: React.ComponentProps<"div">) {
	return (
		<div
			id={id}
			data-slot="app-shell-content"
			className={cn("min-h-0 flex-1 overflow-auto", className)}
			{...props}
		/>
	);
}

type AppShellBrandProps = useRender.ComponentProps<"button"> & {
	/** Brand mark, e.g. a white logo `<img>` or `<svg>`. Rendered inside the tile. */
	logo?: React.ReactNode;
	/** Primary brand name. */
	name: React.ReactNode;
	/** Optional secondary line under the name. */
	description?: React.ReactNode;
	/** Override the tile classes (defaults to the sidebar-primary tile). */
	tileClassName?: string;
};

/**
 * Brand row for the app shell sidebar header: a colored tile holding the brand
 * mark, plus a name and an optional sub-line. The text collapses away when the
 * sidebar is in icon mode. Render as a link/button via the `render` prop.
 */
function AppShellBrand({
	logo,
	name,
	description,
	render,
	className,
	tileClassName,
	...props
}: AppShellBrandProps) {
	const brandClassName = cn(
		"group/brand flex w-full items-center gap-2.5 rounded-lg p-1.5 text-left transition-colors hover:bg-muted focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
		"group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:gap-0 group-data-[collapsible=icon]:p-0",
		className,
	);
	const inner = (
		<>
			<span
				data-slot="app-shell-brand-tile"
				className={cn(
					"grid size-8 shrink-0 place-items-center rounded-lg bg-sidebar-primary text-sidebar-primary-foreground [&_img]:size-[22px] [&_svg]:size-[22px]",
					tileClassName,
				)}
			>
				{logo}
			</span>
			<span className="grid min-w-0 group-data-[collapsible=icon]:hidden">
				<span className="truncate text-sm font-semibold leading-tight">
					{name}
				</span>
				{description ? (
					<span className="truncate text-[11px] leading-tight text-muted-foreground">
						{description}
					</span>
				) : null}
			</span>
		</>
	);

	// `render` (e.g. a router Link) becomes the row element, with the tile + text
	// injected as its children. Without it, we fall back to a plain button.
	return useRender({
		defaultTagName: "button",
		render,
		state: { slot: "app-shell-brand" },
		props: mergeProps<"button">(
			{
				className: brandClassName,
				children: inner,
				...(render ? {} : { type: "button" as const }),
			},
			props,
		),
	});
}

export {
	AppShell,
	AppShellBrand,
	AppShellContent,
	AppShellInset,
	AppShellSidebar,
	AppShellSidebarTrigger,
	AppShellTopbar,
	AppShellTopbarActions,
	AppShellTopbarContent,
};
