use std::collections::{HashMap, HashSet};

use chrono::Utc;
use serde::{Deserialize, Serialize};
use uuid::Uuid;

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Conversation {
    pub id: String,
    pub pipeline_id: String,
    pub stage_id: String,
    pub topic: String,
    pub participants: Vec<String>,
    pub status: ConversationStatus,
    pub round_count: u32,
    pub max_rounds: u32,
    pub created_at: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub enum ConversationStatus {
    Active,
    Converged,
    Expired,
    Escalated,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Message {
    pub id: String,
    pub conversation_id: String,
    pub round: u32,
    pub from_role: String,
    pub message_type: String,
    pub body: String,
    pub timestamp: String,
}

#[derive(Debug, Clone)]
pub struct ConvergenceResult {
    pub converged: bool,
    pub expired: bool,
    pub should_escalate: bool,
}

#[derive(Debug, Default)]
pub struct MessageBus {
    conversations: HashMap<String, Conversation>,
    messages: HashMap<String, Vec<Message>>,
}

impl MessageBus {
    pub fn create_conversation(
        &mut self,
        pipeline_id: &str,
        stage_id: &str,
        topic: &str,
        participants: Vec<String>,
        max_rounds: u32,
    ) -> Conversation {
        let conversation = Conversation {
            id: format!("conv_{}", Uuid::new_v4().simple()),
            pipeline_id: pipeline_id.to_string(),
            stage_id: stage_id.to_string(),
            topic: topic.to_string(),
            participants,
            status: ConversationStatus::Active,
            round_count: 0,
            max_rounds,
            created_at: Utc::now().to_rfc3339(),
        };
        self.conversations
            .insert(conversation.id.clone(), conversation.clone());
        conversation
    }

    pub fn send(
        &mut self,
        conversation_id: &str,
        round: u32,
        from_role: &str,
        message_type: &str,
        body: &str,
    ) -> Message {
        let message = Message {
            id: format!("msg_{}", Uuid::new_v4().simple()),
            conversation_id: conversation_id.to_string(),
            round,
            from_role: from_role.to_string(),
            message_type: message_type.to_string(),
            body: body.to_string(),
            timestamp: Utc::now().to_rfc3339(),
        };
        self.messages
            .entry(conversation_id.to_string())
            .or_default()
            .push(message.clone());
        message
    }

    pub fn get_messages(&self, conversation_id: &str) -> Vec<Message> {
        self.messages
            .get(conversation_id)
            .cloned()
            .unwrap_or_default()
    }

    pub fn get_conversation(&self, conversation_id: &str) -> Option<Conversation> {
        self.conversations.get(conversation_id).cloned()
    }

    pub fn update_status(
        &mut self,
        conversation_id: &str,
        status: ConversationStatus,
        round_count: u32,
    ) {
        if let Some(conv) = self.conversations.get_mut(conversation_id) {
            conv.status = status;
            conv.round_count = round_count;
        }
    }
}

#[derive(Debug, Default)]
pub struct ConvergenceEngine;

impl ConvergenceEngine {
    pub fn check(
        &self,
        conversation: &Conversation,
        messages: &[Message],
        round: u32,
        force_expire: bool,
        force_escalate: bool,
    ) -> ConvergenceResult {
        if force_escalate {
            return ConvergenceResult {
                converged: false,
                expired: false,
                should_escalate: true,
            };
        }

        if force_expire {
            return ConvergenceResult {
                converged: false,
                expired: round >= conversation.max_rounds,
                should_escalate: false,
            };
        }

        let responders = messages
            .iter()
            .filter(|m| m.round == round)
            .map(|m| m.from_role.clone())
            .collect::<HashSet<_>>();
        let everyone_replied = conversation
            .participants
            .iter()
            .all(|p| responders.contains(p));

        if everyone_replied && round >= 2 {
            return ConvergenceResult {
                converged: true,
                expired: false,
                should_escalate: false,
            };
        }

        ConvergenceResult {
            converged: false,
            expired: round >= conversation.max_rounds,
            should_escalate: false,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::{ConvergenceEngine, ConversationStatus, MessageBus};

    #[test]
    fn converges_when_all_participants_respond_by_round_two() {
        let mut bus = MessageBus::default();
        let conv = bus.create_conversation(
            "pipe_x",
            "alignment",
            "team alignment",
            vec!["architect".to_string(), "coder".to_string()],
            5,
        );
        bus.send(&conv.id, 1, "architect", "discuss", "start");
        bus.send(&conv.id, 1, "coder", "propose", "feedback");
        let engine = ConvergenceEngine;
        let r1 = engine.check(&conv, &bus.get_messages(&conv.id), 1, false, false);
        assert!(!r1.converged);

        bus.send(&conv.id, 2, "architect", "discuss", "update");
        bus.send(&conv.id, 2, "coder", "propose", "agree");
        let r2 = engine.check(&conv, &bus.get_messages(&conv.id), 2, false, false);
        assert!(r2.converged);

        bus.update_status(&conv.id, ConversationStatus::Converged, 2);
        let saved = bus.get_conversation(&conv.id).expect("conversation");
        assert_eq!(saved.status, ConversationStatus::Converged);
    }
}
