import { EventEmitter } from 'node:events';
import { ulid } from 'ulid';
import type { AgentRole, Conversation, ConversationStatus, Message } from './types.js';

export interface MessageRepository {
  saveMessage(msg: Message): Promise<void>;
  getMessages(conversationId: string): Promise<Message[]>;
  saveConversation(conv: Conversation): Promise<void>;
  getConversation(id: string): Promise<Conversation | null>;
  updateConversationStatus(id: string, status: ConversationStatus, roundCount?: number): Promise<void>;
}

export class MessageBus {
  private emitter = new EventEmitter();
  private repo: MessageRepository;

  constructor(repo: MessageRepository) {
    this.repo = repo;
  }

  async send(msg: Omit<Message, 'id' | 'timestamp'>): Promise<Message> {
    const full: Message = {
      ...msg,
      id: ulid(),
      timestamp: new Date().toISOString(),
    };
    await this.repo.saveMessage(full);

    // Update conversation round count on proposals
    if (msg.type === 'propose') {
      const conv = await this.repo.getConversation(msg.conversationId);
      if (conv) {
        await this.repo.updateConversationStatus(conv.id, conv.status, conv.roundCount + 1);
      }
    }

    // Emit to specific role or broadcast
    if (full.routing === 'unicast' && full.toRole) {
      this.emitter.emit(`msg:${full.toRole}`, full);
    } else {
      // broadcast / meeting — emit to all
      this.emitter.emit('msg:all', full);
    }

    return full;
  }

  subscribe(role: AgentRole, handler: (msg: Message) => Promise<void>): void {
    this.emitter.on(`msg:${role}`, handler);
    this.emitter.on('msg:all', (msg: Message) => {
      // Don't double-deliver to sender
      if (msg.fromRole !== role) {
        handler(msg);
      }
    });
  }

  async createConversation(opts: {
    pipelineId: string;
    stageId?: string;
    topic: string;
    participants: AgentRole[];
    maxRounds?: number;
  }): Promise<Conversation> {
    const conv: Conversation = {
      id: ulid(),
      pipelineId: opts.pipelineId,
      stageId: opts.stageId,
      topic: opts.topic,
      participants: opts.participants,
      status: 'active',
      roundCount: 0,
      maxRounds: opts.maxRounds ?? 5,
    };
    await this.repo.saveConversation(conv);
    return conv;
  }

  async getConversation(id: string): Promise<Conversation | null> {
    return this.repo.getConversation(id);
  }

  async getMessages(conversationId: string): Promise<Message[]> {
    return this.repo.getMessages(conversationId);
  }
}