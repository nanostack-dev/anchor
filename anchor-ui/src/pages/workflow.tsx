import {
	getWorkflowCatalogOptions,
	getWorkflowOptions,
	listWorkflowRunsOptions,
	listWorkflowsOptions,
} from "@/client/@tanstack/react-query.gen";
import { Page } from "@/components/common/Page";
import { WorkflowBuilder } from "@/components/workflow/WorkflowBuilder";
import { WorkflowRunHistory } from "@/components/workflow/WorkflowRunHistory";
import { findRecipe } from "@/components/workflow/recipes";
import {
	draftFromResponse,
	emptyDraft,
} from "@/components/workflow/workflow-model";
import { useProduct } from "@/context/product/ProductContext";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyContent,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import {
	Tabs,
	TabsContent,
	TabsList,
	TabsTrigger,
} from "@nanostackorg/design-system/components/tabs";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

const RUNS_REFRESH_MS = 5_000;

export default function WorkflowPage({
	workflowId,
	recipeId,
}: {
	workflowId?: string;
	recipeId?: string;
}) {
	const { currentProduct } = useProduct();
	const navigate = useNavigate();
	const productId = currentProduct?.id ?? "";
	const enabled = Boolean(currentProduct);
	const catalogQuery = useQuery({
		...getWorkflowCatalogOptions({ path: { product_id: productId } }),
		enabled,
	});
	const workflowPath = { product_id: productId, workflow_id: workflowId ?? "" };
	const workflowQuery = useQuery({
		...getWorkflowOptions({ path: workflowPath }),
		enabled: enabled && Boolean(workflowId),
		retry: false,
	});
	const workflowsQuery = useQuery({
		...listWorkflowsOptions({ path: { product_id: productId } }),
		enabled,
	});
	const runsQuery = useQuery({
		...listWorkflowRunsOptions({ path: workflowPath, query: { limit: 50 } }),
		enabled: enabled && Boolean(workflowId),
		refetchInterval: RUNS_REFRESH_MS,
	});

	const backToList = () => void navigate({ to: ROUTE_PATHS.PRODUCT_WORKFLOWS });
	const openSaved = (id: string) =>
		id !== workflowId &&
		void navigate({
			to: ROUTE_PATHS.PRODUCT_WORKFLOW_DETAIL,
			params: { workflowId: id },
			replace: true,
		});

	if (!currentProduct) {
		return (
			<Page title="Workflow">
				<Empty>
					<EmptyHeader>
						<EmptyTitle>No product selected</EmptyTitle>
						<EmptyDescription>
							Pick a product from the top bar to edit its workflows.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			</Page>
		);
	}

	if (workflowId && workflowQuery.isError) {
		return (
			<Page title="Workflow">
				<Empty>
					<EmptyHeader>
						<EmptyTitle>Workflow not found</EmptyTitle>
						<EmptyDescription>
							It may have been deleted, or it belongs to another product.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			</Page>
		);
	}

	if (catalogQuery.isError || workflowsQuery.isError) {
		return (
			<Page title={workflowId ? "Workflow" : "New workflow"}>
				<Empty variant="outline">
					<EmptyHeader>
						<EmptyTitle>The workflow builder could not load</EmptyTitle>
						<EmptyDescription>
							The action catalog or your other workflows did not load.
						</EmptyDescription>
					</EmptyHeader>
					<EmptyContent>
						<Button
							variant="outline"
							onClick={() => {
								void catalogQuery.refetch();
								void workflowsQuery.refetch();
							}}
						>
							Try again
						</Button>
					</EmptyContent>
				</Empty>
			</Page>
		);
	}

	const catalog = catalogQuery.data;
	const workflow = workflowQuery.data;
	if (!catalog || !workflowsQuery.data || (workflowId && !workflow)) {
		return (
			<Page title={workflowId ? "Workflow" : "New workflow"}>
				<Stack space="md">
					<Skeleton />
					<Skeleton />
				</Stack>
			</Page>
		);
	}

	const recipe = findRecipe(recipeId);
	const initialDraft = workflow
		? draftFromResponse(workflow)
		: (recipe?.draft ?? emptyDraft());
	const builder = (
		<WorkflowBuilder
			key={workflow?.updated_at ?? recipeId ?? "new"}
			productId={productId}
			catalog={catalog}
			initialDraft={initialDraft}
			workflow={workflow}
			workflows={workflowsQuery.data.items}
			onSaved={(saved) => openSaved(saved.id)}
			onDeleted={backToList}
			onOpenWorkflow={(id) =>
				void navigate({
					to: ROUTE_PATHS.PRODUCT_WORKFLOW_DETAIL,
					params: { workflowId: id },
				})
			}
		/>
	);

	return (
		<Page
			title={workflow?.name ?? recipe?.title ?? "New workflow"}
			description={
				workflow
					? undefined
					: "Pick the event that starts it, narrow it with conditions, then lay out the steps. Every step can read the event and what earlier steps produced."
			}
			breadCrumbLabels={workflow ? { [workflow.id]: workflow.name } : undefined}
		>
			{workflow ? (
				<Tabs defaultValue="build">
					<TabsList>
						<TabsTrigger value="build">Build</TabsTrigger>
						<TabsTrigger value="runs">
							Runs{runsQuery.data ? ` (${runsQuery.data.count})` : ""}
						</TabsTrigger>
					</TabsList>
					<TabsContent value="build">{builder}</TabsContent>
					<TabsContent value="runs">
						<WorkflowRunHistory
							runs={runsQuery.data?.items ?? []}
							catalog={catalog}
						/>
					</TabsContent>
				</Tabs>
			) : (
				builder
			)}
		</Page>
	);
}
