import type { WorkflowActionResponse } from "@/client";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@nanostackorg/design-system/components/dropdown-menu";
import { Plus } from "lucide-react";
import { Fragment } from "react";
import { groupBy } from "./workflow-model";

export function AddStepMenu({
	actions,
	onPick,
}: {
	actions: WorkflowActionResponse[];
	onPick: (action: WorkflowActionResponse) => void;
}) {
	const groups = groupBy(actions, (action) => action.group);
	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				render={<Button variant="outline" icon={Plus} width="fill" />}
			>
				Add a step
			</DropdownMenuTrigger>
			<DropdownMenuContent width="md" align="center">
				{groups.map(([group, items], index) => (
					<Fragment key={group}>
						{index > 0 ? <DropdownMenuSeparator /> : null}
						<DropdownMenuGroup>
							<DropdownMenuLabel>{group}</DropdownMenuLabel>
							{items.map((action) => (
								<DropdownMenuItem
									key={action.type}
									onClick={() => onPick(action)}
								>
									{action.name}
								</DropdownMenuItem>
							))}
						</DropdownMenuGroup>
					</Fragment>
				))}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
