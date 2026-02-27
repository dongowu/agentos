export type DecisionStatus = "pending" | "approved" | "rejected";

export interface PendingDecision {
  id: string;
  pipelineId: string;
  stageId: string;
  type: "human_gate" | "escalation";
  description: string;
  status: DecisionStatus;
  reason?: string;
  createdAt: string;
  resolvedAt?: string;
}
