//! Native runtime -- executes commands directly on the host OS.

use crate::{
    truncate_output, ExecutionResult, ExecutionSpec, RuntimeAdapter, RuntimeError, SAFE_ENV_VARS,
};
use async_trait::async_trait;
use std::path::PathBuf;
use std::time::Instant;

// ---------------------------------------------------------------------------
// Shell detection (zeroclaw pattern)
// ---------------------------------------------------------------------------

#[derive(Debug, Clone, PartialEq, Eq)]
struct ShellProgram {
    kind: ShellKind,
    program: PathBuf,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum ShellKind {
    Sh,
    Bash,
    Cmd,
}

impl ShellProgram {
    fn add_args(&self, cmd: &mut tokio::process::Command, command: &str) {
        match self.kind {
            ShellKind::Sh | ShellKind::Bash => {
                cmd.arg("-c").arg(command);
            }
            ShellKind::Cmd => {
                cmd.arg("/C").arg(command);
            }
        }
    }
}

fn detect_shell() -> Option<ShellProgram> {
    #[cfg(target_os = "windows")]
    {
        detect_shell_impl(true)
    }
    #[cfg(not(target_os = "windows"))]
    {
        detect_shell_impl(false)
    }
}

fn detect_shell_impl(is_windows: bool) -> Option<ShellProgram> {
    if is_windows {
        for (name, kind) in [
            ("bash", ShellKind::Bash),
            ("sh", ShellKind::Sh),
            ("cmd", ShellKind::Cmd),
        ] {
            if let Ok(program) = which_lookup(name) {
                return Some(ShellProgram { kind, program });
            }
        }
        // Fallback: COMSPEC
        if let Some(comspec) = std::env::var_os("COMSPEC") {
            return Some(ShellProgram {
                kind: ShellKind::Cmd,
                program: PathBuf::from(comspec),
            });
        }
        None
    } else {
        for (name, kind) in [("sh", ShellKind::Sh), ("bash", ShellKind::Bash)] {
            if let Ok(program) = which_lookup(name) {
                return Some(ShellProgram { kind, program });
            }
        }
        None
    }
}

/// Simple which(1) equivalent: check if binary exists on PATH.
fn which_lookup(name: &str) -> Result<PathBuf, ()> {
    let path_var = std::env::var_os("PATH").unwrap_or_default();
    for dir in std::env::split_paths(&path_var) {
        let candidate = dir.join(name);
        if candidate.is_file() {
            return Ok(candidate);
        }
    }
    Err(())
}

// ---------------------------------------------------------------------------
// NativeRuntime
// ---------------------------------------------------------------------------

/// Executes commands directly on the host OS via a detected shell.
pub struct NativeRuntime {
    shell: Option<ShellProgram>,
}

impl NativeRuntime {
    pub fn new() -> Self {
        Self {
            shell: detect_shell(),
        }
    }

    #[doc(hidden)]
    pub fn new_without_shell() -> Self {
        Self { shell: None }
    }
}

impl Default for NativeRuntime {
    fn default() -> Self {
        Self::new()
    }
}

#[async_trait]
impl RuntimeAdapter for NativeRuntime {
    fn name(&self) -> &str {
        "native"
    }

    fn has_shell_access(&self) -> bool {
        self.shell.is_some()
    }

    fn has_filesystem_access(&self) -> bool {
        true
    }

    async fn execute(&self, spec: ExecutionSpec) -> Result<ExecutionResult, RuntimeError> {
        let shell = self
            .shell
            .as_ref()
            .ok_or_else(|| RuntimeError::ShellNotFound(
                "no usable shell found (tried: sh, bash). Install a POSIX shell on PATH.".into(),
            ))?;

        let mut cmd = tokio::process::Command::new(&shell.program);
        shell.add_args(&mut cmd, &spec.command);

        if let Some(ref dir) = spec.working_dir {
            cmd.current_dir(dir);
        }

        // Environment isolation: clear everything, re-add only safe vars.
        cmd.env_clear();
        for key in SAFE_ENV_VARS {
            if let Ok(val) = std::env::var(key) {
                cmd.env(key, val);
            }
        }
        // Merge caller-provided env vars.
        for (k, v) in &spec.env {
            cmd.env(k, v);
        }

        cmd.stdout(std::process::Stdio::piped());
        cmd.stderr(std::process::Stdio::piped());

        let start = Instant::now();

        let output = tokio::time::timeout(spec.timeout, cmd.output())
            .await
            .map_err(|_| RuntimeError::Timeout {
                elapsed: spec.timeout,
            })?
            .map_err(RuntimeError::IoError)?;

        let duration = start.elapsed();

        let (stdout, stdout_truncated) = truncate_output(&output.stdout, spec.max_output_bytes);
        let (stderr, stderr_truncated) = truncate_output(&output.stderr, spec.max_output_bytes);

        Ok(ExecutionResult {
            exit_code: output.status.code().unwrap_or(-1),
            stdout,
            stderr,
            duration,
            truncated: stdout_truncated || stderr_truncated,
        })
    }
}
