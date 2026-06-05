import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Link } from "@tanstack/react-router";
import { InfoIcon } from "lucide-react";

export interface PageInfoProps {
	title: string;
	description: string;
	linkTo?: string;
	linkText?: string;
}

export function PageInfo({
	title,
	description,
	linkTo,
	linkText,
}: PageInfoProps) {
	return (
		<Alert className="border-border bg-accent-soft text-accent-foreground">
			<InfoIcon className="text-primary" />
			<AlertTitle>{title}</AlertTitle>
			<AlertDescription className="text-accent-foreground/90">
				{description}
				{linkTo && linkText && (
					<>
						{" "}
						<Link
							to={linkTo}
							className="inline-flex items-center gap-1 text-primary underline underline-offset-4 hover:text-primary/80"
						>
							{linkText}
						</Link>
					</>
				)}
			</AlertDescription>
		</Alert>
	);
}
