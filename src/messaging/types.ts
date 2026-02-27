export type MessageType = 'discuss' | 'propose' | 'challenge' | 'agree' | 'deliver' | 'escalate';
export type RoutingMode = 'unicast' | 'broadcast' | 'meeting';
export type ConversationStatus = 'active' | 'converged' | 'escalated' | 'closed';
export type AgentRole = 'pm' | 'architect' | 'coder' | 'reviewer' | 'tester';

export interface Message {
  id: string;
  conversationId: string;
  parentId?: string;
  type: MessageType;
  routing: RoutingMode;
  fromRole: AgentRole;
  toRole?: AgentRole;
  subject?: string;
  body: string;
  metadata?: Record<string, unknown>;
  timestamp: string;
}

export interface Conversation {
  id: string;
  pipelineId: string;
  stageId?: string;
  topic: string;
  participants: AgentRole[];
  status: ConversationStatus;
  roundCount: number;
  maxRounds: number;
}
