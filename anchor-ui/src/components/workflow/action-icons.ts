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
