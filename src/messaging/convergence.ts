import type { MessageBus } from './message-bus.js';

export interface ConvergenceResult {
  converged: boolean;
  expired: boolean;
  shouldEscalate: boolean;
  agreedProposal?: string;
}

export class ConvergenceEngine {
  private bus: MessageBus;

  constructor(bus: MessageBus) {
    this.bus = bus;
  }

  async checkConvergence(conversationId: string): Promise<ConvergenceResult> {
    const conv = await this.bus.getConversation(conversationId);
    if (!conv) {
      return { converged: false, expired: false, shouldEscalate: false };
    }

    const messages = await this.bus.getMessages(conversationId);

    // Find the latest proposal
    const proposals = messages.filter((m) => m.type === 'propose');
    if (proposals.length === 0) {
      const expired = conv.roundCount >= conv.maxRounds;
      return { converged: false, expired, shouldEscalate: expired };
    }

    const latestProposal = proposals[proposals.length - 1]!;

    // Check if all other participants agreed to this proposal
    const otherParticipants = conv.participants.filter((p) => p !== latestProposal.fromRole);
    const agreements = messages.filter(
      (m) => m.type === 'agree' && m.parentId === latestProposal.id,
    );
    const agreedRoles = new Set(agreements.map((a) => a.fromRole));
    const allAgreed = otherParticipants.every((p) => agreedRoles.has(p));

    if (allAgreed) {
      return {
        converged: true,
        expired: false,
        shouldEscalate: false,
        agreedProposal: latestProposal.id,
      };
    }

    const expired = conv.roundCount >= conv.maxRounds;
    return { converged: false, expired, shouldEscalate: expired };
  }
}
