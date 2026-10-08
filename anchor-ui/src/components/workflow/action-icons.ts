import {
	Award,
	Building2,
	FolderKanban,
	type LucideIcon,
	Mail,
	UserRound,
	Users,
	Webhook,
	Zap,
} from "lucide-react";

const groupIcons: Record<string, LucideIcon> = {
	Organizations: Building2,
	Workspaces: FolderKanban,
	Members: Users,
	Users: UserRound,
	Licensing: Award,
	Email: Mail,
	Custom: Webhook,
};

export function actionGroupIcon(group: string | undefined): LucideIcon {
	return (group && groupIcons[group]) || Zap;
}

const groupTones: Record<string, string> = {
	Organizations: "bg-primary/10 text-primary",
	Workspaces: "bg-info/12 text-info-on-tint",
	Members: "bg-success/12 text-success-on-tint",
	Users: "bg-info/12 text-info-on-tint",
	Licensing: "bg-warning/15 text-warning-on-tint",
	Email: "bg-success/12 text-success-on-tint",
	Custom: "bg-foreground/[0.06] text-foreground",
};

export function actionGroupTone(group: string | undefined): string {
	return (group && groupTones[group]) || "bg-muted text-foreground";
}
