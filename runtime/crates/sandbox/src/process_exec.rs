use crate::{truncate_output, ExecutionResult, RuntimeError};
use std::time::{Duration, Instant};
use tokio::io::{AsyncRead, AsyncReadExt};

struct CapturedOutput {
    bytes: Vec<u8>,
    truncated: bool,
}

pub(crate) async fn execute_command<F>(
    cmd: &mut tokio::process::Command,
    timeout: Duration,
    max_output_bytes: usize,
    timeout_context: Option<&'static str>,
    map_spawn_error: F,
) -> Result<ExecutionResult, RuntimeError>
where
    F: FnOnce(std::io::Error) -> RuntimeError,
{
    cmd.stdout(std::process::Stdio::piped());
    cmd.stderr(std::process::Stdio::piped());
    configure_process_group(cmd);

    let start = Instant::now();

    let mut child = cmd.spawn().map_err(map_spawn_error)?;
    let stdout_pipe = child
        .stdout
        .take()
        .ok_or_else(|| RuntimeError::Other("failed to capture stdout pipe".into()))?;
    let stderr_pipe = child
        .stderr
        .take()
        .ok_or_else(|| RuntimeError::Other("failed to capture stderr pipe".into()))?;

    let stdout_handle =
        tokio::spawn(async move { read_capped_output(stdout_pipe, max_output_bytes).await });
    let stderr_handle =
        tokio::spawn(async move { read_capped_output(stderr_pipe, max_output_bytes).await });

    let status = match tokio::time::timeout(timeout, child.wait()).await {
        Ok(status) => status.map_err(RuntimeError::IoError)?,
        Err(_) => {
            terminate_child_with_fallback(&mut child, kill_process_group).await;
            let _ = stdout_handle.await;
            let _ = stderr_handle.await;
            return Err(RuntimeError::Timeout {
                elapsed: timeout,
                context: timeout_context,
            });
        }
    };

    let stdout = stdout_handle
        .await
        .map_err(|err| RuntimeError::Other(format!("stdout capture task failed: {err}")))?
        .map_err(RuntimeError::IoError)?;
    let stderr = stderr_handle
        .await
        .map_err(|err| RuntimeError::Other(format!("stderr capture task failed: {err}")))?
        .map_err(RuntimeError::IoError)?;

    let duration = start.elapsed();

    Ok(ExecutionResult {
        exit_code: status.code().unwrap_or(-1),
        stdout: stdout.bytes,
        stderr: stderr.bytes,
        duration,
        truncated: stdout.truncated || stderr.truncated,
    })
}

async fn read_capped_output<R>(
    mut reader: R,
    max_output_bytes: usize,
) -> Result<CapturedOutput, std::io::Error>
where
    R: AsyncRead + Unpin,
{
    let capture_limit = max_output_bytes.saturating_add(1);
    let mut buf = Vec::with_capacity(capture_limit.min(8192));
    let mut chunk = [0u8; 8192];

    loop {
        let read = reader.read(&mut chunk).await?;
        if read == 0 {
            break;
        }

        if buf.len() < capture_limit {
            let remaining = capture_limit - buf.len();
            let to_copy = read.min(remaining);
            buf.extend_from_slice(&chunk[..to_copy]);
        }
    }

    if buf.len() > max_output_bytes {
        let (bytes, truncated) = truncate_output(&buf, max_output_bytes);
        return Ok(CapturedOutput { bytes, truncated });
    }

    Ok(CapturedOutput {
        bytes: buf,
        truncated: false,
    })
}

async fn terminate_child_with_fallback<F>(child: &mut tokio::process::Child, kill_group: F)
where
    F: FnOnce(u32) -> std::io::Result<()>,
{
    let killed_group = child.id().is_some_and(|pid| kill_group(pid).is_ok());
    if !killed_group {
        let _ = child.kill().await;
    }
    let _ = child.wait().await;
}

fn configure_process_group(_cmd: &mut tokio::process::Command) {
    #[cfg(unix)]
    _cmd.process_group(0);
}

#[cfg(unix)]
fn kill_process_group(pid: u32) -> std::io::Result<()> {
    unsafe extern "C" {
        fn kill(pid: i32, sig: i32) -> i32;
    }

    const SIGKILL: i32 = 9;
    let result = unsafe { kill(-(pid as i32), SIGKILL) };
    if result == 0 {
        return Ok(());
    }

    let err = std::io::Error::last_os_error();
    if err.raw_os_error() == Some(3) {
        return Ok(());
    }
    Err(err)
}

#[cfg(test)]
mod tests {
    use super::{execute_command, read_capped_output, terminate_child_with_fallback};
    use crate::RuntimeError;
    #[cfg(not(target_os = "windows"))]
    use std::fs;
    #[cfg(not(target_os = "windows"))]
    use std::path::PathBuf;
    use std::time::Duration;
    #[cfg(not(target_os = "windows"))]
    use std::time::Instant;
    use tokio::io::AsyncReadExt;
    #[cfg(not(target_os = "windows"))]
    mod temp_paths {
        include!(concat!(env!("CARGO_MANIFEST_DIR"), "/../../test_support/temp_paths.rs"));
    }

    #[tokio::test]
    async fn read_capped_output_preserves_small_streams() {
        let reader = tokio::io::repeat(b'x').take(16);
        let captured = read_capped_output(reader, 64)
            .await
            .expect("small stream should be captured");

        assert_eq!(captured.bytes.len(), 16);
        assert!(!captured.truncated);
    }

    #[tokio::test]
    async fn read_capped_output_truncates_large_streams_without_unbounded_growth() {
        let reader = tokio::io::repeat(b'y').take(4096);
        let captured = read_capped_output(reader, 32)
            .await
            .expect("large stream should be captured");

        assert!(captured.truncated);
        assert!(captured.bytes.len() <= 64);
        assert!(
            String::from_utf8_lossy(&captured.bytes).contains("[output truncated]"),
            "expected truncation marker in captured output"
        );
    }

    #[cfg(not(target_os = "windows"))]
    #[tokio::test]
    async fn execute_command_captures_output_and_exit_code() {
        let mut cmd = tokio::process::Command::new("/bin/sh");
        cmd.arg("-c").arg("printf hello; printf error >&2; exit 7");

        let result = execute_command(
            &mut cmd,
            Duration::from_secs(1),
            1024,
            None,
            RuntimeError::IoError,
        )
        .await
        .expect("command should succeed");

        assert_eq!(result.exit_code, 7);
        assert_eq!(String::from_utf8_lossy(&result.stdout), "hello");
        assert_eq!(String::from_utf8_lossy(&result.stderr), "error");
        assert!(!result.truncated);
    }

    #[cfg(not(target_os = "windows"))]
    #[tokio::test]
    async fn execute_command_kills_timed_out_process_group() {
        let work_dir = unique_test_dir("helper-timeout-cleanup");
        fs::create_dir_all(&work_dir).expect("temp dir should be created");
        let pid_file = work_dir.join("shell.pid");

        let mut cmd = tokio::process::Command::new("/bin/sh");
        cmd.current_dir(&work_dir);
        cmd.arg("-c").arg("echo $$ > shell.pid; sleep 60");

        let start = Instant::now();
        let result = execute_command(
            &mut cmd,
            Duration::from_millis(200),
            1024,
            Some("helper"),
            RuntimeError::IoError,
        )
        .await;
        let elapsed = start.elapsed();
        let err = result.expect_err("timeout command should fail");

        assert!(err.to_string().contains("helper command timed out"));
        let pid = wait_for_pid_file(&pid_file).await;
        let still_running = process_is_running(pid);
        if still_running {
            force_kill_process(pid);
        }

        fs::remove_file(&pid_file).ok();
        fs::remove_dir_all(&work_dir).ok();
        assert!(
            elapsed < Duration::from_secs(5),
            "timed out command took too long to return: {elapsed:?}"
        );
        assert!(
            !still_running,
            "timed out shell process {pid} was left running"
        );
    }

    #[cfg(not(target_os = "windows"))]
    #[tokio::test]
    async fn terminate_child_falls_back_to_direct_kill_when_group_kill_fails() {
        let mut cmd = tokio::process::Command::new("/bin/sleep");
        cmd.arg("60");
        let mut child = cmd.spawn().expect("sleep should spawn");
        let pid = child.id().expect("sleep should have a pid");

        terminate_child_with_fallback(&mut child, |_pid| {
            Err(std::io::Error::other("synthetic process-group failure"))
        })
        .await;

        assert!(
            !process_is_running(pid),
            "fallback child kill should terminate process {pid}"
        );
    }

    #[cfg(not(target_os = "windows"))]
    fn unique_test_dir(label: &str) -> PathBuf {
        temp_paths::unique_test_dir(&format!("process-exec-{label}"))
    }

    #[cfg(not(target_os = "windows"))]
    async fn wait_for_pid_file(path: &PathBuf) -> u32 {
        for _ in 0..20 {
            if let Ok(contents) = fs::read_to_string(path) {
                if let Ok(pid) = contents.trim().parse::<u32>() {
                    return pid;
                }
            }
            tokio::time::sleep(Duration::from_millis(50)).await;
        }
        panic!("pid file was not created: {}", path.display());
    }

    #[cfg(not(target_os = "windows"))]
    fn process_is_running(pid: u32) -> bool {
        std::process::Command::new("kill")
            .arg("-0")
            .arg(pid.to_string())
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .status()
            .map(|status| status.success())
            .unwrap_or(false)
    }

    #[cfg(not(target_os = "windows"))]
    fn force_kill_process(pid: u32) {
        let _ = std::process::Command::new("kill")
            .arg("-9")
            .arg(pid.to_string())
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .status();
    }
}
