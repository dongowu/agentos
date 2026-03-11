use std::ffi::{OsStr, OsString};
use std::io;
use std::path::{Path, PathBuf};
use std::sync::{Mutex, MutexGuard, OnceLock};
use std::time::{Duration, Instant};

pub struct ScopedEnvVar {
    key: String,
    original: Option<OsString>,
    _process_guard: MutexGuard<'static, ()>,
    lock_dir: PathBuf,
}

impl ScopedEnvVar {
    pub fn set<K, V>(key: K, value: V) -> io::Result<Self>
    where
        K: Into<String>,
        V: AsRef<OsStr>,
    {
        let key = key.into();
        let process_guard = process_env_lock()
            .lock()
            .expect("env lock should not be poisoned");
        let lock_dir = acquire_cross_process_lock(&key)?;
        let original = std::env::var_os(&key);
        std::env::set_var(&key, value);

        Ok(Self {
            key,
            original,
            _process_guard: process_guard,
            lock_dir,
        })
    }
}

impl Drop for ScopedEnvVar {
    fn drop(&mut self) {
        if let Some(original) = &self.original {
            std::env::set_var(&self.key, original);
        } else {
            std::env::remove_var(&self.key);
        }
        let _ = std::fs::remove_dir_all(&self.lock_dir);
    }
}

#[allow(dead_code)]
pub fn prepend_path(path: &Path, original: &OsStr) -> OsString {
    if original.is_empty() {
        return path.as_os_str().to_owned();
    }

    let mut joined = OsString::new();
    joined.push(path.as_os_str());
    joined.push(":");
    joined.push(original);
    joined
}

fn process_env_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}

fn acquire_cross_process_lock(key: &str) -> io::Result<PathBuf> {
    let lock_dir = std::env::temp_dir().join(format!("agentos-env-lock-{}", sanitize(key)));
    if let Some(parent) = lock_dir.parent() {
        std::fs::create_dir_all(parent)?;
    }
    let deadline = Instant::now() + Duration::from_secs(10);

    loop {
        match std::fs::create_dir(&lock_dir) {
            Ok(()) => {
                let _ = std::fs::write(lock_dir.join("owner.pid"), std::process::id().to_string());
                return Ok(lock_dir);
            }
            Err(err) if err.kind() == io::ErrorKind::AlreadyExists => {
                if clear_stale_lock(&lock_dir)? {
                    continue;
                }
                if Instant::now() >= deadline {
                    return Err(io::Error::new(
                        io::ErrorKind::TimedOut,
                        format!("timed out waiting for env lock: {key}"),
                    ));
                }
                std::thread::sleep(Duration::from_millis(10));
            }
            Err(err) => return Err(err),
        }
    }
}

fn sanitize(key: &str) -> String {
    key.chars()
        .map(|ch| match ch {
            'A'..='Z' | 'a'..='z' | '0'..='9' | '-' | '_' => ch,
            _ => '-',
        })
        .collect()
}

fn clear_stale_lock(lock_dir: &Path) -> io::Result<bool> {
    #[cfg(unix)]
    {
        let owner = lock_dir.join("owner.pid");
        let Ok(contents) = std::fs::read_to_string(&owner) else {
            return Ok(false);
        };
        let Ok(pid) = contents.trim().parse::<u32>() else {
            return Ok(false);
        };
        if process_is_running(pid) {
            return Ok(false);
        }
        std::fs::remove_dir_all(lock_dir)?;
        return Ok(true);
    }

    #[cfg(not(unix))]
    {
        let _ = lock_dir;
        Ok(false)
    }
}

#[cfg(unix)]
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
