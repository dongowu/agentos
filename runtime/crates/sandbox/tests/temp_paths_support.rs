#[path = "../../../test_support/temp_paths.rs"]
mod temp_paths;

#[test]
fn unique_test_dir_is_namespaced_under_system_temp_and_unique() {
    let first = temp_paths::unique_test_dir("sandbox-support");
    let second = temp_paths::unique_test_dir("sandbox-support");

    assert!(first.starts_with(std::env::temp_dir()));
    assert!(first.to_string_lossy().contains("sandbox-support"));
    assert_ne!(first, second, "unique_test_dir should avoid collisions");
}
