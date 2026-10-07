import {
	Award,
	Building2,
	FolderKanban,
	type LucideIcon,
	Mail,
	UserRound,
	Users,
	Zap,
} from "lucide-react";

const groupIcons: Record<string, LucideIcon> = {
	Organizations: Building2,
	Workspaces: FolderKanban,
	Members: Users,
	Users: UserRound,
	Licensing: Award,
	Email: Mail,
};

export function actionGroupIcon(group: string | undefined): LucideIcon {
	return (group && groupIcons[group]) || Zap;
}
