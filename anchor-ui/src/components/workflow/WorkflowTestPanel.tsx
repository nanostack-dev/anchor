import type {
	WorkflowCatalogResponse,
	WorkflowRunResponse,
	WorkflowTriggerResponse,
} from "@/client";
import {
	dryRunWorkflowMutation,
	listProductWorkflowRunsQueryKey,
	listWorkflowRunsQueryKey,
	runWorkflowMutation,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { ConfirmDialog } from "@nanostackorg/design-system/blocks/confirm-dialog";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import {
	Field,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { FlaskConical, Play } from "lucide-react";
import { useEffect, useId, useState } from "react";
import { WorkflowRunView } from "./WorkflowRunView";
import {
	type WorkflowDraft,
	draftToRequest,
	sampleEventData,
} from "./workflow-model";

export function WorkflowTestPanel({
	productId,
	workflowId,
	draft,
	trigger,
	catalog,
	dirty,
}: {
	productId: string;
	workflowId?: string;
	draft: WorkflowDraft;
	trigger: WorkflowTriggerResponse | undefined;
	catalog: WorkflowCatalogResponse;
	dirty: boolean;
}) {
	const queryClient = useQueryClient();
	const fieldPrefix = useId();
	const [eventData, setEventData] = useState<Record<string, string>>(() =>
		sampleEventData(trigger),
	);
	const [result, setResult] = useState<WorkflowRunResponse | null>(null);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		setEventData((previous) => sampleEventData(trigger, previous));
	}, [trigger]);

	const settle = (run: WorkflowRunResponse | null, failure?: unknown) => {
		setResult(run);
		setError(
			failure
				? (getApiErrorMessage(failure) ?? "The run could not start.")
				: null,
		);
	};
	const dryRun = useMutation({
		...dryRunWorkflowMutation(),
		onSuccess: (run) => settle(run),
		onError: (failure) => settle(null, failure),
	});
	const runNow = useMutation({
		...runWorkflowMutation(),
		onSuccess: (run) => {
			settle(run);
			void queryClient.invalidateQueries({
				queryKey: listWorkflowRunsQueryKey({
					path: { product_id: productId, workflow_id: workflowId ?? "" },
				}),
			});
			void queryClient.invalidateQueries({
				queryKey: listProductWorkflowRunsQueryKey({
					path: { product_id: productId },
				}),
			});
		},
		onError: (failure) => settle(null, failure),
	});

	return (
		<Card>
			<CardHeader>
				<CardTitle icon={FlaskConical}>Try it</CardTitle>
				<CardDescription>
					Feed the workflow a sample event. A dry run reads for real and only
					pretends to write.
				</CardDescription>
			</CardHeader>
			<CardContent>
				<Stack space="lg">
					<FieldGroup>
						{Object.keys(eventData).length === 0 ? (
							<Text size="sm" tone="muted">
								Pick a trigger to see the data its event carries.
							</Text>
						) : null}
						{Object.entries(eventData).map(([field, value]) => (
							<Field key={field}>
								<FieldLabel htmlFor={`${fieldPrefix}-${field}`} size="sm">
									event.data.{field}
								</FieldLabel>
								<Input
									id={`${fieldPrefix}-${field}`}
									size="sm"
									font="mono"
									value={value}
									onChange={(event) =>
										setEventData({ ...eventData, [field]: event.target.value })
									}
								/>
							</Field>
						))}
					</FieldGroup>
					<Inline space="sm" wrap>
						<Button
							variant="solid"
							tone="brand"
							size="sm"
							icon={FlaskConical}
							loading={dryRun.isPending}
							disabled={!trigger || draft.definition.steps.length === 0}
							onClick={() =>
								dryRun.mutate({
									path: { product_id: productId },
									body: {
										workflow: draftToRequest(draft),
										event_data: eventData,
									},
								})
							}
						>
							Dry run
						</Button>
						{workflowId ? (
							<ConfirmDialog
								trigger={
									<Button
										variant="outline"
										size="sm"
										icon={Play}
										loading={runNow.isPending}
										disabled={dirty}
									>
										Run for real
									</Button>
								}
								title="Run this workflow for real?"
								description="Every step writes to your product's resources, exactly as if the event had happened."
								confirmLabel="Run now"
								onConfirm={() =>
									runNow.mutate({
										path: { product_id: productId, workflow_id: workflowId },
										body: { event_data: eventData },
									})
								}
							/>
						) : null}
					</Inline>
					{workflowId && dirty ? (
						<Text size="xs" tone="muted">
							Save your changes to run the workflow for real.
						</Text>
					) : null}
					{error ? (
						<Text size="sm" tone="critical">
							{error}
						</Text>
					) : null}
					{result ? (
						<Box as="section" aria-label="Run result">
							<WorkflowRunView run={result} catalog={catalog} />
						</Box>
					) : null}
				</Stack>
			</CardContent>
		</Card>
	);
}
