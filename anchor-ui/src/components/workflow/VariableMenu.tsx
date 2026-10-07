import { Button } from "@nanostackorg/design-system/components/button";
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuTrigger,
} from "@nanostackorg/design-system/components/dropdown-menu";
import { Braces } from "lucide-react";
import { type WorkflowVariable, groupBy } from "./workflow-model";

export function VariableMenu({
	variables,
	onPick,
	label = "Insert a value",
}: {
	variables: WorkflowVariable[];
	onPick: (path: string) => void;
	label?: string;
}) {
	return (
		<DropdownMenu>
			<DropdownMenuTrigger
				render={
					<Button
						variant="ghost"
						size="xs"
						icon={Braces}
						aria-label={label}
						disabled={variables.length === 0}
					/>
				}
			/>
			<DropdownMenuContent width="md" align="end">
				{groupBy(variables, (variable) => variable.source).map(
					([source, group]) => (
						<DropdownMenuGroup key={source}>
							<DropdownMenuLabel>{source}</DropdownMenuLabel>
							{group.map((variable) => (
								<DropdownMenuItem
									key={variable.path}
									onClick={() => onPick(variable.path)}
								>
									{variable.label}
								</DropdownMenuItem>
							))}
						</DropdownMenuGroup>
					),
				)}
			</DropdownMenuContent>
		</DropdownMenu>
	);
}
