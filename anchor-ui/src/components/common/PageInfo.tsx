import {
	Alert,
	AlertDescription,
	AlertTitle,
} from "@nanostackorg/design-system/components/alert";
import { TextLink } from "@nanostackorg/design-system/components/text-link";
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
		<Alert tone="info">
			<InfoIcon aria-hidden />
			<AlertTitle>{title}</AlertTitle>
			<AlertDescription>
				{description}
				{linkTo && linkText && (
					<>
						{" "}
						<TextLink href={linkTo}>{linkText}</TextLink>
					</>
				)}
			</AlertDescription>
		</Alert>
	);
}
