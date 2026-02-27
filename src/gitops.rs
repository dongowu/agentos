use std::ffi::OsStr;
use std::path::Path;
use std::process::Command;

use anyhow::{anyhow, Context, Result};

#[derive(Debug, Clone)]
pub struct CheckpointResult {
    pub commit: String,
}

pub fn create_checkpoint(cwd: &Path, message: &str) -> Result<CheckpointResult> {
    ensure_repository(cwd)?;
    let before_head = current_head(cwd).ok();

    run_git(cwd, ["add", "-A"])?;
    let status = run_git(cwd, ["status", "--porcelain"])?;
    let has_changes = !status.trim().is_empty();

    if has_changes || before_head.is_none() {
        let mut commit_args = vec!["commit", "-m", message];
        if before_head.is_none() && !has_changes {
            commit_args.push("--allow-empty");
        }
        run_git(cwd, commit_args)?;
    }

    let head = current_head(cwd).context("failed to resolve HEAD after checkpoint")?;
    Ok(CheckpointResult { commit: head })
}

pub fn rollback_to_checkpoint(cwd: &Path, checkpoint: &str) -> Result<()> {
    ensure_repository(cwd)?;
    run_git(cwd, ["reset", "--hard", checkpoint])?;
    run_git(cwd, ["clean", "-fd"])?;
    Ok(())
}

fn ensure_repository(cwd: &Path) -> Result<()> {
    if run_git(cwd, ["rev-parse", "--is-inside-work-tree"]).is_err() {
        run_git(cwd, ["init"])?;
    }

    run_git(cwd, ["config", "user.name", "AI Orchestrator"])?;
    run_git(cwd, ["config", "user.email", "orchestrator@local"])?;
    Ok(())
}

fn current_head(cwd: &Path) -> Result<String> {
    let output = run_git(cwd, ["rev-parse", "HEAD"])?;
    output
        .split_whitespace()
        .next()
        .map(ToString::to_string)
        .ok_or_else(|| anyhow!("git rev-parse HEAD returned empty output"))
}

fn run_git<I, S>(cwd: &Path, args: I) -> Result<String>
where
    I: IntoIterator<Item = S>,
    S: AsRef<OsStr>,
{
    let output = Command::new("git")
        .args(args)
        .current_dir(cwd)
        .output()
        .with_context(|| format!("failed to execute git in {}", cwd.display()))?;

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    let combined = format!("{stdout}{stderr}");
    if output.status.success() {
        Ok(combined)
    } else {
        Err(anyhow!("git command failed: {}", combined.trim()))
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::{create_checkpoint, rollback_to_checkpoint};

    #[test]
    fn creates_and_rolls_back_checkpoint() {
        let temp = tempfile::tempdir().expect("tempdir");
        let file_a = temp.path().join("a.txt");
        fs::write(&file_a, "v1").expect("write v1");

        let checkpoint = create_checkpoint(temp.path(), "initial").expect("checkpoint");
        assert!(!checkpoint.commit.is_empty());

        fs::write(&file_a, "v2").expect("write v2");
        let file_b = temp.path().join("danger.txt");
        fs::write(&file_b, "unsafe").expect("write danger");

        rollback_to_checkpoint(temp.path(), &checkpoint.commit).expect("rollback");

        let content = fs::read_to_string(&file_a).expect("read file_a");
        assert_eq!(content, "v1");
        assert!(!file_b.exists());
    }
}
