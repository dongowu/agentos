import { generateId } from "../utils/id.js";
import type { DecisionStatus, PendingDecision } from "./types.js";

export interface HumanGateResult {
  approved: boolean;
  feedback?: string;
}

export class HumanGate {
  private decisions = new Map<string, PendingDecision>();
  private resolvers = new Map<string, (result: HumanGateResult) => void>();

  /**
   * Workflow-facing method: pause the pipeline and wait for human approval.
   * Matches the HumanGate interface in workflow/types.ts.
   */
  async request(
    pipelineId: string,
    stageId: string,
    _outputs: Array<{ artifactType: string; name: string; filePath: string }>,
  ): Promise<HumanGateResult> {
    const description = `Review outputs for stage "${stageId}"`;
    const decision = this.pause(pipelineId, stageId, description);

    // Return a promise that resolves when approve/reject is called
    return new Promise<HumanGateResult>((resolve) => {
      this.resolvers.set(decision.id, resolve);
    });
  }

  pause(pipelineId: string, stageId: string, description: string): PendingDecision {
    const decision: PendingDecision = {
      id: generateId("dec"),
      pipelineId,
      stageId,
      type: "human_gate",
      description,
      status: "pending",
      createdAt: new Date().toISOString(),
    };
    this.decisions.set(decision.id, decision);
    return decision;
  }

  approve(decisionId: string): PendingDecision {
    const decision = this.resolve(decisionId, "approved");
    const resolver = this.resolvers.get(decisionId);
    if (resolver) {
      resolver({ approved: true });
      this.resolvers.delete(decisionId);
    }
    return decision;
  }

  reject(decisionId: string, reason?: string): PendingDecision {
    const decision = this.resolve(decisionId, "rejected", reason);
    const resolver = this.resolvers.get(decisionId);
    if (resolver) {
      resolver({ approved: false, feedback: reason });
      this.resolvers.delete(decisionId);
    }
    return decision;
  }

  listPending(): PendingDecision[] {
    return [...this.decisions.values()].filter((d) => d.status === "pending");
  }

  getDecision(id: string): PendingDecision | undefined {
    return this.decisions.get(id);
  }

  private resolve(id: string, status: DecisionStatus, reason?: string): PendingDecision {
    const decision = this.decisions.get(id);
    if (!decision) {
      throw new Error(`Decision not found: ${id}`);
    }
    if (decision.status !== "pending") {
      throw new Error(`Decision ${id} already resolved as ${decision.status}`);
    }
    decision.status = status;
    decision.resolvedAt = new Date().toISOString();
    if (reason) {
      decision.reason = reason;
    }
    return decision;
  }
}
