//! Integration tests for SecurityPolicy.

use agentos_sandbox::security::{AutonomyLevel, SecurityPolicy};

#[test]
fn default_policy_allows_ls() {
    let policy = SecurityPolicy::default();
    assert!(policy.validate_command("ls").is_ok());
}

#[test]
fn default_policy_allows_cat() {
    let policy = SecurityPolicy::default();
    assert!(policy.validate_command("cat").is_ok());
}

#[test]
fn default_policy_allows_echo_with_args() {
    let policy = SecurityPolicy::default();
    assert!(policy.validate_command("echo hello world").is_ok());
}

#[test]
fn default_policy_blocks_rm_rf_root() {
    let policy = SecurityPolicy::default();
    let result = policy.validate_command("rm -rf /");
    assert!(result.is_err());
}

#[test]
fn default_policy_blocks_forbidden_path() {
    let policy = SecurityPolicy::default();
    let result = policy.validate_command("cat /etc/shadow");
    assert!(result.is_err());
}

#[test]
fn supervised_blocks_unlisted_command() {
    let policy = SecurityPolicy {
        autonomy: AutonomyLevel::Supervised,
        ..SecurityPolicy::default()
    };
    let result = policy.validate_command("curl http://example.com");
    assert!(result.is_err());
    assert!(result.unwrap_err().message.contains("whitelist"));
}

#[test]
fn semi_autonomous_blocks_unlisted() {
    let policy = SecurityPolicy {
        autonomy: AutonomyLevel::SemiAutonomous,
        ..SecurityPolicy::default()
    };
    let result = policy.validate_command("wget http://example.com");
    assert!(result.is_err());
}

#[test]
fn autonomous_allows_non_blacklisted() {
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
fn autonomous_still_blocks_forbidden_paths() {
    let policy = SecurityPolicy {
        autonomy: AutonomyLevel::Autonomous,
        ..SecurityPolicy::default()
    };
    assert!(policy.validate_command("cat /etc/passwd").is_err());
}

#[test]
fn empty_command_rejected() {
    let policy = SecurityPolicy::default();
    assert!(policy.validate_command("").is_err());
    assert!(policy.validate_command("   ").is_err());
}

#[test]
fn custom_whitelist() {
    let policy = SecurityPolicy {
        autonomy: AutonomyLevel::Supervised,
        command_whitelist: vec!["cargo".into(), "rustc".into()],
        ..SecurityPolicy::default()
    };
    assert!(policy.validate_command("cargo build").is_ok());
    assert!(policy.validate_command("rustc --version").is_ok());
    assert!(policy.validate_command("gcc main.c").is_err());
}

#[test]
fn custom_blacklist() {
    let policy = SecurityPolicy {
        autonomy: AutonomyLevel::Autonomous,
        command_blacklist: vec!["shutdown *".into()],
        ..SecurityPolicy::default()
    };
    assert!(policy.validate_command("shutdown -h now").is_err());
    assert!(policy.validate_command("ls").is_ok());
}

// --- Secret redaction tests ---

#[test]
fn redact_api_key_pattern() {
    let policy = SecurityPolicy::default();
    let input = "Found key: sk-abcdef1234567890abcdef";
    let output = policy.redact_secrets(input);
    assert!(!output.contains("sk-abcdef1234567890abcdef"));
    assert!(output.contains("[REDACTED]"));
}

#[test]
fn redact_bearer_token() {
    let policy = SecurityPolicy::default();
    let input = "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig";
    let output = policy.redact_secrets(input);
    assert!(!output.contains("eyJhbGciOiJIUzI1NiJ9"));
    assert!(output.contains("[REDACTED]"));
}

#[test]
fn redact_github_personal_token() {
    let policy = SecurityPolicy::default();
    let input = "GITHUB_TOKEN=ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmn";
    let output = policy.redact_secrets(input);
    assert!(!output.contains("ghp_ABCDEFGHIJ"));
    assert!(output.contains("[REDACTED]"));
}

#[test]
fn redact_aws_key() {
    let policy = SecurityPolicy::default();
    let input = "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE";
    let output = policy.redact_secrets(input);
    assert!(!output.contains("AKIAIOSFODNN7EXAMPLE"));
    assert!(output.contains("[REDACTED]"));
}

#[test]
fn redact_key_value_pair() {
    let policy = SecurityPolicy::default();
    let input = "api_key=abc123def456ghi789jkl012mno";
    let output = policy.redact_secrets(input);
    assert!(output.contains("[REDACTED]"), "output was: {output}");
}

#[test]
fn safe_output_not_redacted() {
    let policy = SecurityPolicy::default();
    let input =
        "total 42\ndrwxr-xr-x 2 user user 4096 Jan 1 src\n-rw-r--r-- 1 user user 100 main.rs";
    let output = policy.redact_secrets(input);
    assert_eq!(output, input);
}

#[test]
fn redact_multiple_secrets_in_one_string() {
    let policy = SecurityPolicy::default();
    let input = "key=sk-aaaa1111bbbb2222cccc and token: Bearer eyJhbGciOiJIUzI1NiJ9.x.y";
    let output = policy.redact_secrets(input);
    assert!(!output.contains("sk-aaaa1111bbbb2222cccc"));
    assert!(!output.contains("eyJhbGciOiJIUzI1NiJ9"));
}

// --- Serialization ---

#[test]
fn policy_serializes_to_json() {
    let policy = SecurityPolicy::default();
    let json = serde_json::to_string(&policy).expect("should serialize");
    assert!(json.contains("supervised"));
    assert!(json.contains("command_whitelist"));
}

#[test]
fn policy_deserializes_from_json() {
    let json = r#"{
        "autonomy": "autonomous",
        "command_whitelist": ["ls"],
        "command_blacklist": [],
        "forbidden_paths": [],
        "max_actions_per_hour": 60,
        "max_output_bytes": 512000
    }"#;
    let policy: SecurityPolicy = serde_json::from_str(json).expect("should deserialize");
    assert_eq!(policy.autonomy, AutonomyLevel::Autonomous);
    assert_eq!(policy.max_actions_per_hour, 60);
    assert_eq!(policy.max_output_bytes, 512000);
}
