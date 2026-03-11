use std::path::PathBuf;
use std::time::{SystemTime, UNIX_EPOCH};

pub fn unique_test_dir(label: &str) -> PathBuf {
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock should be after epoch")
        .as_nanos();
    std::env::temp_dir().join(format!(
        "agentos-{label}-{}-{nonce}",
        std::process::id()
    ))
}
