import {
	getWorkflowCatalogOptions,
	listProductWorkflowRunsOptions,
	listWorkflowsOptions,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { WorkflowList } from "@/components/workflow/WorkflowList";
import { useProduct } from "@/context/product/ProductContext";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Plus } from "lucide-react";

const RUNS_REFRESH_MS = 5_000;

export default function WorkflowsPage() {
	const { currentProduct } = useProduct();
	const navigate = useNavigate();
	const path = { product_id: currentProduct?.id ?? "" };
	const enabled = Boolean(currentProduct);
	const catalogQuery = useQuery({
		...getWorkflowCatalogOptions({ path }),
		enabled,
	});
	const workflowsQuery = useQuery({
		...listWorkflowsOptions({ path }),
		enabled,
	});
	const runsQuery = useQuery({
		...listProductWorkflowRunsOptions({ path, query: { limit: 50 } }),
		enabled,
		refetchInterval: RUNS_REFRESH_MS,
	});

	const openWorkflow = (workflowId: string) =>
		void navigate({
			to: ROUTE_PATHS.PRODUCT_WORKFLOW_DETAIL,
			params: { workflowId },
		});
	const createWorkflow = (recipe?: { id: string }) =>
		void navigate({
			to: ROUTE_PATHS.PRODUCT_WORKFLOW_NEW,
			search: recipe ? { recipe: recipe.id } : {},
		});

	return (
		<Page
			title="Workflows"
			description="Automate your product's resources. When a product event happens, a workflow reads and changes organizations, workspaces, members, invitations, users, licenses and email, the way you lay it out."
			actions={
				currentProduct ? (
					<Button tone="brand" icon={Plus} onClick={() => createWorkflow()}>
						New workflow
					</Button>
				) : null
			}
		>
			{!currentProduct ? (
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product from the top bar to automate its resources.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : workflowsQuery.isPending ? (
				<Skeleton />
			) : (
				<WorkflowList
					workflows={workflowsQuery.data?.items ?? []}
					runs={runsQuery.data?.items ?? []}
					catalog={catalogQuery.data}
					onOpen={openWorkflow}
					onCreate={createWorkflow}
				/>
			)}
		</Page>
	);
}
