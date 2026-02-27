use regex::Regex;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct GuardDecision {
    pub allowed: bool,
    pub reason: Option<String>,
}

#[derive(Debug)]
pub struct ExecutionGuard {
    shell_blocklist: Vec<Regex>,
    protected_path_patterns: Vec<Regex>,
}

impl Default for ExecutionGuard {
    fn default() -> Self {
        Self {
            shell_blocklist: vec![
                Regex::new(r"\brm\s+-rf\b").expect("valid regex"),
                Regex::new(r"\bsudo\b").expect("valid regex"),
                Regex::new(r"\bshutdown\b").expect("valid regex"),
                Regex::new(r"\breboot\b").expect("valid regex"),
                Regex::new(r"\bpoweroff\b").expect("valid regex"),
                Regex::new(r"\bcurl\b[^|]*\|\s*(sh|bash)\b").expect("valid regex"),
                Regex::new(r"\bwget\b[^|]*\|\s*(sh|bash)\b").expect("valid regex"),
                Regex::new(r"\bmkfs\b").expect("valid regex"),
                Regex::new(r"\bdd\s+if=").expect("valid regex"),
                Regex::new(r"\bgit\s+reset\s+--hard\b").expect("valid regex"),
            ],
            protected_path_patterns: vec![
                Regex::new(r"(^|/)\.git(/|$)").expect("valid regex"),
                Regex::new(r"(^|/)\.env($|\.|/)").expect("valid regex"),
                Regex::new(r"\.(pem|key|p12)$").expect("valid regex"),
            ],
        }
    }
}

impl ExecutionGuard {
    pub fn validate_shell(&self, command: &str) -> GuardDecision {
        let normalized = command.trim();
        if normalized.is_empty() {
            return GuardDecision {
                allowed: false,
                reason: Some("Empty command is not allowed".to_string()),
            };
        }

        for pattern in &self.shell_blocklist {
            if pattern.is_match(normalized) {
                return GuardDecision {
                    allowed: false,
                    reason: Some(format!(
                        "Command blocked by guardrail: {}",
                        pattern.as_str()
                    )),
                };
            }
        }

        GuardDecision {
            allowed: true,
            reason: None,
        }
    }

    pub fn validate_filesystem(&self, operation: &str, target_path: &str) -> GuardDecision {
        if !matches!(operation, "writeFile" | "deleteFile" | "createDirectory") {
            return GuardDecision {
                allowed: true,
                reason: None,
            };
        }

        let normalized = target_path.replace('\\', "/");
        for pattern in &self.protected_path_patterns {
            if pattern.is_match(&normalized) {
                return GuardDecision {
                    allowed: false,
                    reason: Some(format!("Path blocked by guardrail: {}", pattern.as_str())),
                };
            }
        }

        GuardDecision {
            allowed: true,
            reason: None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::ExecutionGuard;

    #[test]
    fn blocks_dangerous_shell_commands() {
        let guard = ExecutionGuard::default();
        let blocked = guard.validate_shell("rm -rf /");
        assert!(!blocked.allowed);
        assert!(blocked.reason.unwrap().contains("guardrail"));

        let allowed = guard.validate_shell("echo safe");
        assert!(allowed.allowed);
    }

    #[test]
    fn blocks_sensitive_filesystem_targets() {
        let guard = ExecutionGuard::default();
        let blocked = guard.validate_filesystem("writeFile", "/tmp/project/.env");
        assert!(!blocked.allowed);

        let allowed = guard.validate_filesystem("writeFile", "/tmp/project/docs/readme.md");
        assert!(allowed.allowed);
    }
}
