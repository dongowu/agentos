import type { PendingDecision } from "./types.js";
import { HumanGate } from "./gate.js";

export class EscalationHandler {
  constructor(private gate: HumanGate) {}

  escalate(pipelineId: string, conversationId: string, reason: string): PendingDecision {
    const decision = this.gate.pause(pipelineId, conversationId, reason);
    // Override the type to escalation (gate.pause defaults to human_gate)
    (decision as { type: PendingDecision["type"] }).type = "escalation";
    return decision;
  }
}
