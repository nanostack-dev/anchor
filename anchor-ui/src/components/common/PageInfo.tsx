import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Link } from "@tanstack/react-router";
import { Info } from "lucide-react";

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
		<Alert className="border-blue-200 bg-blue-50/50">
			<Info className="text-blue-600" />
			<AlertTitle className="text-blue-900">{title}</AlertTitle>
			<AlertDescription className="text-blue-800">
				{description}
				{linkTo && linkText && (
					<>
						{" "}
						<Link
							to={linkTo}
							className="text-blue-600 hover:text-blue-800 underline inline-flex items-center gap-1"
						>
							{linkText}
						</Link>
					</>
				)}
			</AlertDescription>
		</Alert>
	);
}
