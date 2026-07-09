import { Menubar as MenubarPrimitive } from "@base-ui/react/menubar";
import type * as React from "react";

import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuPortal,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuSeparator,
	DropdownMenuShortcut,
	DropdownMenuSub,
	DropdownMenuSubContent,
	DropdownMenuSubTrigger,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { cn } from "@/lib/utils";

function Menubar({ className, ...props }: MenubarPrimitive.Props) {
	return (
		<MenubarPrimitive
			data-slot="menubar"
			className={cn(
				"bg-background flex h-9 items-center gap-1 rounded-md border p-1 shadow-xs",
				className,
			)}
			{...props}
		/>
	);
}

function MenubarMenu({ ...props }: React.ComponentProps<typeof DropdownMenu>) {
	return <DropdownMenu data-slot="menubar-menu" {...props} />;
}

function MenubarGroup({
	...props
}: React.ComponentProps<typeof DropdownMenuGroup>) {
	return <DropdownMenuGroup data-slot="menubar-group" {...props} />;
}

function MenubarPortal({
	...props
}: React.ComponentProps<typeof DropdownMenuPortal>) {
	return <DropdownMenuPortal data-slot="menubar-portal" {...props} />;
}

function MenubarRadioGroup({
	...props
}: React.ComponentProps<typeof DropdownMenuRadioGroup>) {
	return <DropdownMenuRadioGroup data-slot="menubar-radio-group" {...props} />;
}

function MenubarTrigger({
	className,
	...props
}: React.ComponentProps<typeof DropdownMenuTrigger>) {
	return (
		<DropdownMenuTrigger
			data-slot="menubar-trigger"
			className={cn(
				"focus:bg-accent focus:text-accent-foreground data-popup-open:bg-accent data-popup-open:text-accent-foreground flex items-center rounded-sm px-2 py-1 text-sm font-medium outline-hidden select-none",
				className,
			)}
			{...props}
		/>
	);
}

function MenubarContent({
	className,
	align = "start",
	alignOffset = -4,
	sideOffset = 8,
	...props
}: React.ComponentProps<typeof DropdownMenuContent>) {
	return (
		<DropdownMenuContent
			data-slot="menubar-content"
			align={align}
			alignOffset={alignOffset}
			sideOffset={sideOffset}
			className={cn("min-w-[12rem]", className)}
			{...props}
		/>
	);
}

function MenubarItem({
	className,
	inset,
	variant = "default",
	...props
}: React.ComponentProps<typeof DropdownMenuItem>) {
	return (
		<DropdownMenuItem
			data-slot="menubar-item"
			inset={inset}
			variant={variant}
			className={className}
			{...props}
		/>
	);
}

function MenubarCheckboxItem({
	className,
	children,
	...props
}: React.ComponentProps<typeof DropdownMenuCheckboxItem>) {
	return (
		<DropdownMenuCheckboxItem
			data-slot="menubar-checkbox-item"
			className={cn("rounded-xs", className)}
			{...props}
		>
			{children}
		</DropdownMenuCheckboxItem>
	);
}

function MenubarRadioItem({
	className,
	children,
	...props
}: React.ComponentProps<typeof DropdownMenuRadioItem>) {
	return (
		<DropdownMenuRadioItem
			data-slot="menubar-radio-item"
			className={cn("rounded-xs", className)}
			{...props}
		>
			{children}
		</DropdownMenuRadioItem>
	);
}

function MenubarLabel({
	className,
	inset,
	...props
}: React.ComponentProps<typeof DropdownMenuLabel>) {
	return (
		<DropdownMenuLabel
			data-slot="menubar-label"
			inset={inset}
			className={className}
			{...props}
		/>
	);
}

function MenubarSeparator({
	className,
	...props
}: React.ComponentProps<typeof DropdownMenuSeparator>) {
	return (
		<DropdownMenuSeparator
			data-slot="menubar-separator"
			className={className}
			{...props}
		/>
	);
}

function MenubarShortcut({
	className,
	...props
}: React.ComponentProps<"span">) {
	return (
		<DropdownMenuShortcut
			data-slot="menubar-shortcut"
			className={className}
			{...props}
		/>
	);
}

function MenubarSub({
	...props
}: React.ComponentProps<typeof DropdownMenuSub>) {
	return <DropdownMenuSub data-slot="menubar-sub" {...props} />;
}

function MenubarSubTrigger({
	className,
	inset,
	children,
	...props
}: React.ComponentProps<typeof DropdownMenuSubTrigger>) {
	return (
		<DropdownMenuSubTrigger
			data-slot="menubar-sub-trigger"
			inset={inset}
			className={className}
			{...props}
		>
			{children}
		</DropdownMenuSubTrigger>
	);
}

function MenubarSubContent({
	className,
	...props
}: React.ComponentProps<typeof DropdownMenuSubContent>) {
	return (
		<DropdownMenuSubContent
			data-slot="menubar-sub-content"
			className={className}
			{...props}
		/>
	);
}

export {
	Menubar,
	MenubarPortal,
	MenubarMenu,
	MenubarTrigger,
	MenubarContent,
	MenubarGroup,
	MenubarSeparator,
	MenubarLabel,
	MenubarItem,
	MenubarShortcut,
	MenubarCheckboxItem,
	MenubarRadioGroup,
	MenubarRadioItem,
	MenubarSub,
	MenubarSubTrigger,
	MenubarSubContent,
};
