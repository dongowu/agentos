#[path = "../../../test_support/env_lock.rs"]
mod env_lock;

#[test]
fn scoped_env_var_restores_original_value_on_drop() {
    let key = "AGENTOS_ENV_LOCK_TEST";
    std::env::remove_var(key);

    {
        let _guard =
            env_lock::ScopedEnvVar::set(key, "scoped-value").expect("env guard should set value");
        assert_eq!(
            std::env::var(key).expect("scoped var should be visible"),
            "scoped-value"
        );
    }

    assert!(
        std::env::var_os(key).is_none(),
        "scoped env var should be restored after drop"
    );
}
