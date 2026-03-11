//! Security policy for command validation and secret redaction.

use regex::Regex;
use serde::{Deserialize, Serialize};
use std::path::PathBuf;
use std::sync::OnceLock;

/// How much autonomy the agent has.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum AutonomyLevel {
    /// Supervised: acts but requires approval for risky operations.
    Supervised,
    /// Semi-autonomous: auto-execute whitelisted, block blacklisted, ask otherwise.
    SemiAutonomous,
    /// Autonomous: execute anything not explicitly blacklisted.
    Autonomous,
}

impl Default for AutonomyLevel {
    fn default() -> Self {
        Self::Supervised
    }
}

/// Security policy governing command execution.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SecurityPolicy {
    pub autonomy: AutonomyLevel,
    /// Glob patterns for allowed commands (e.g. "ls", "cat *", "grep *").
    pub command_whitelist: Vec<String>,
    /// Glob patterns for denied commands (e.g. "rm -rf *", "chmod 777 *").
    pub command_blacklist: Vec<String>,
    /// Paths that must never appear as command arguments.
    pub forbidden_paths: Vec<PathBuf>,
    /// Maximum actions per hour (0 = unlimited).
    pub max_actions_per_hour: u32,
    /// Maximum output bytes before truncation.
    pub max_output_bytes: usize,
}

impl Default for SecurityPolicy {
    fn default() -> Self {
        Self {
            autonomy: AutonomyLevel::Supervised,
            command_whitelist: vec![
                "ls".into(),
                "cat".into(),
                "echo".into(),
                "pwd".into(),
                "head".into(),
                "tail".into(),
                "grep".into(),
                "find".into(),
                "wc".into(),
            ],
            command_blacklist: vec![
                "rm -rf /".into(),
                "rm -rf /*".into(),
                "mkfs.*".into(),
                "dd if=/dev/*".into(),
                "chmod 777 *".into(),
                ":(){ :|:& };:".into(),
            ],
            forbidden_paths: vec![PathBuf::from("/etc/shadow"), PathBuf::from("/etc/passwd")],
            max_actions_per_hour: 120,
            max_output_bytes: 1_048_576,
        }
    }
}

/// Error from security policy validation.
#[derive(Debug, Clone)]
pub struct SecurityError {
    pub message: String,
}

impl std::fmt::Display for SecurityError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.message)
    }
}

impl std::error::Error for SecurityError {}

impl SecurityPolicy {
    /// Validate whether a command is allowed under this policy.
    pub fn validate_command(&self, cmd: &str) -> Result<(), SecurityError> {
        let trimmed = cmd.trim();

        if trimmed.is_empty() {
            return Err(SecurityError {
                message: "empty command".into(),
            });
        }

        // Check blacklist first (always deny)
        for pattern in &self.command_blacklist {
            if glob_matches(pattern, trimmed) {
                return Err(SecurityError {
                    message: format!("command matches blacklist pattern: {pattern}"),
                });
            }
        }

        // Check forbidden paths in arguments
        for path in &self.forbidden_paths {
            let path_str = path.to_string_lossy();
            if trimmed.contains(path_str.as_ref()) {
                return Err(SecurityError {
                    message: format!("command references forbidden path: {}", path.display()),
                });
            }
        }

        // In supervised/semi-autonomous mode, check whitelist
        match self.autonomy {
            AutonomyLevel::Autonomous => Ok(()),
            AutonomyLevel::SemiAutonomous => {
                if contains_shell_control_operator(trimmed) && !matches_whitelist(self, trimmed) {
                    return Err(SecurityError {
                        message: "compound command requires explicit whitelist entry".into(),
                    });
                }
                let base_cmd = extract_base_command(trimmed);
                for pattern in &self.command_whitelist {
                    if glob_matches(pattern, trimmed) || pattern == base_cmd {
                        return Ok(());
                    }
                }
                // Semi-autonomous: deny commands not in whitelist
                Err(SecurityError {
                    message: format!(
                        "command '{base_cmd}' not in whitelist (semi-autonomous mode)"
                    ),
                })
            }
            AutonomyLevel::Supervised => {
                if contains_shell_control_operator(trimmed) && !matches_whitelist(self, trimmed) {
                    return Err(SecurityError {
                        message: "compound command requires explicit whitelist entry".into(),
                    });
                }
                let base_cmd = extract_base_command(trimmed);
                for pattern in &self.command_whitelist {
                    if glob_matches(pattern, trimmed) || pattern == base_cmd {
                        return Ok(());
                    }
                }
                Err(SecurityError {
                    message: format!(
                        "command '{base_cmd}' not in whitelist (supervised mode, requires approval)"
                    ),
                })
            }
        }
    }

    /// Redact secrets from command output.
    ///
    /// Detects patterns like API keys, tokens, passwords and replaces them
    /// with `[REDACTED]`.
    pub fn redact_secrets(&self, output: &str) -> String {
        static SECRET_RE: OnceLock<Regex> = OnceLock::new();
        let re = SECRET_RE.get_or_init(|| {
            let patterns = [
                // API keys: sk-..., key-..., etc
                r"(?:sk|api|key|token|secret|password|passwd|auth|credential)[-_]?[a-zA-Z0-9]{16,}",
                // Bearer tokens
                r#"Bearer\s+[a-zA-Z0-9\-._~+/]+=*"#,
                // AWS-style keys
                r"AKIA[0-9A-Z]{16}",
                // GitHub tokens
                r"gh[pousr]_[A-Za-z0-9_]{36,}",
                // Generic long hex/base64 secrets preceded by keyword
                r#"(?:key|token|secret|password|apikey|api_key|credential)[\s:=]+['"]?[a-zA-Z0-9+/\-_]{20,}['"]?"#,
            ];
            let combined = format!("(?i)(?:{})", patterns.join("|"));
            Regex::new(&combined).expect("secret redaction regex should compile")
        });
        re.replace_all(output, "[REDACTED]").to_string()
    }
}

fn matches_whitelist(policy: &SecurityPolicy, cmd: &str) -> bool {
    policy
        .command_whitelist
        .iter()
        .any(|pattern| pattern == cmd || glob_matches(pattern, cmd))
}

/// Extract the base command name (first token) from a shell command string.
fn extract_base_command(cmd: &str) -> &str {
    cmd.split_whitespace().next().unwrap_or("")
}

fn contains_shell_control_operator(cmd: &str) -> bool {
    ["&&", "||", "|", ";", "$(", "`"]
        .iter()
        .any(|token| cmd.contains(token))
}

/// Simple glob matching: `*` matches any sequence of characters.
fn glob_matches(pattern: &str, text: &str) -> bool {
    let pattern = pattern.trim();
    let text = text.trim();

    if pattern == text {
        return true;
    }

    if !pattern.contains('*') {
        return false;
    }

    let parts: Vec<&str> = pattern.split('*').collect();
    let mut pos = 0;

    for (i, part) in parts.iter().enumerate() {
        if part.is_empty() {
            continue;
        }
        match text[pos..].find(part) {
            Some(found) => {
                // First part must match at start
                if i == 0 && found != 0 {
                    return false;
                }
                pos += found + part.len();
            }
            None => return false,
        }
    }

    // If pattern ends with *, any trailing text is fine
    if pattern.ends_with('*') {
        return true;
    }

    // Otherwise text must be fully consumed
    pos == text.len()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_policy_allows_ls() {
        let policy = SecurityPolicy::default();
        assert!(policy.validate_command("ls").is_ok());
    }

    #[test]
    fn default_policy_allows_echo() {
        let policy = SecurityPolicy::default();
        assert!(policy.validate_command("echo hello").is_ok());
    }

    #[test]
    fn default_policy_blocks_rm_rf_root() {
        let policy = SecurityPolicy::default();
        let result = policy.validate_command("rm -rf /");
        assert!(result.is_err());
        assert!(result.unwrap_err().message.contains("blacklist"));
    }

    #[test]
    fn default_policy_blocks_forbidden_path() {
        let policy = SecurityPolicy::default();
        let result = policy.validate_command("cat /etc/shadow");
        assert!(result.is_err());
        assert!(result.unwrap_err().message.contains("forbidden path"));
    }

    #[test]
    fn supervised_blocks_unknown_command() {
        let policy = SecurityPolicy {
            autonomy: AutonomyLevel::Supervised,
            ..SecurityPolicy::default()
        };
        let result = policy.validate_command("curl http://evil.com");
        assert!(result.is_err());
        assert!(result.unwrap_err().message.contains("whitelist"));
    }

    #[test]
    fn autonomous_allows_any_non_blacklisted() {
        let policy = SecurityPolicy {
            autonomy: AutonomyLevel::Autonomous,
            ..SecurityPolicy::default()
        };
        assert!(policy.validate_command("curl http://example.com").is_ok());
    }

    #[test]
    fn autonomous_still_blocks_blacklisted() {
        let policy = SecurityPolicy {
            autonomy: AutonomyLevel::Autonomous,
            ..SecurityPolicy::default()
        };
        assert!(policy.validate_command("rm -rf /").is_err());
    }

    #[test]
    fn empty_command_rejected() {
        let policy = SecurityPolicy::default();
        assert!(policy.validate_command("").is_err());
        assert!(policy.validate_command("   ").is_err());
    }

    #[test]
    fn redact_api_key() {
        let policy = SecurityPolicy::default();
        let input = "key is sk-abc123456789012345 done";
        let output = policy.redact_secrets(input);
        assert!(!output.contains("sk-abc123456789012345"));
        assert!(output.contains("[REDACTED]"));
    }

    #[test]
    fn redact_bearer_token() {
        let policy = SecurityPolicy::default();
        let input = "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.test";
        let output = policy.redact_secrets(input);
        assert!(!output.contains("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"));
        assert!(output.contains("[REDACTED]"));
    }

    #[test]
    fn redact_github_token() {
        let policy = SecurityPolicy::default();
        let input = "token: ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn";
        let output = policy.redact_secrets(input);
        assert!(!output.contains("ghp_ABCDEFGHIJ"));
        assert!(output.contains("[REDACTED]"));
    }

    #[test]
    fn redact_credential_assignment() {
        let policy = SecurityPolicy::default();
        let input = "CREDENTIAL=abc12345678901234567890";
        let output = policy.redact_secrets(input);
        assert!(!output.contains("abc12345678901234567890"));
        assert!(output.contains("[REDACTED]"));
    }

    #[test]
    fn redact_preserves_safe_text() {
        let policy = SecurityPolicy::default();
        let input = "total 42\ndrwxr-xr-x 2 user user 4096 Jan 1 00:00 src";
        let output = policy.redact_secrets(input);
        assert_eq!(output, input);
    }

    #[test]
    fn glob_matches_exact() {
        assert!(glob_matches("ls", "ls"));
        assert!(!glob_matches("ls", "cat"));
    }

    #[test]
    fn glob_matches_wildcard() {
        assert!(glob_matches("rm -rf *", "rm -rf /tmp/foo"));
        assert!(glob_matches("chmod 777 *", "chmod 777 /tmp/bar"));
    }

    #[test]
    fn glob_matches_prefix_wildcard() {
        assert!(glob_matches("mkfs.*", "mkfs.ext4"));
    }

    #[test]
    fn supervised_blocks_compound_command_with_shell_operators() {
        let policy = SecurityPolicy {
            autonomy: AutonomyLevel::Supervised,
            ..SecurityPolicy::default()
        };
        let result = policy.validate_command("echo ok && rm -rf /tmp/demo");
        assert!(result.is_err());
        assert!(result.unwrap_err().message.contains("compound"));
    }

    #[test]
    fn semi_autonomous_blocks_piped_command_with_whitelisted_prefix() {
        let policy = SecurityPolicy {
            autonomy: AutonomyLevel::SemiAutonomous,
            ..SecurityPolicy::default()
        };
        let result = policy.validate_command("echo ok | cat");
        assert!(result.is_err());
        assert!(result.unwrap_err().message.contains("compound"));
    }
}
