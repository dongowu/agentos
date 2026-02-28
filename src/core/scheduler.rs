use std::collections::HashSet;

use crate::core::models::TaskNode;

pub fn next_runnable(
    all_tasks: &[TaskNode],
    completed: &HashSet<String>,
    running: &HashSet<String>,
    max_parallel: usize,
) -> Vec<TaskNode> {
    let available_slots = max_parallel.saturating_sub(running.len());
    if available_slots == 0 {
        return Vec::new();
    }

    all_tasks
        .iter()
        .filter(|task| !completed.contains(&task.id) && !running.contains(&task.id))
        .filter(|task| task.depends_on.iter().all(|dep| completed.contains(dep)))
        .take(available_slots)
        .cloned()
        .collect()
}

#[cfg(test)]
mod tests {
    use std::collections::HashSet;

    use super::next_runnable;
    use crate::core::models::{Department, TaskNode};

    #[test]
    fn respects_dependencies_and_parallel_limit() {
        let tasks = vec![
            TaskNode {
                id: "a".to_string(),
                title: "A".to_string(),
                owner_role: "pm".to_string(),
                department: Department::Product,
                depends_on: Vec::new(),
            },
            TaskNode {
                id: "b".to_string(),
                title: "B".to_string(),
                owner_role: "coder".to_string(),
                department: Department::Engineering,
                depends_on: vec!["a".to_string()],
            },
            TaskNode {
                id: "c".to_string(),
                title: "C".to_string(),
                owner_role: "tester".to_string(),
                department: Department::Qa,
                depends_on: Vec::new(),
            },
        ];

        let mut completed = HashSet::new();
        let running = HashSet::new();
        let first = next_runnable(&tasks, &completed, &running, 1);
        assert_eq!(first.len(), 1);
        assert_eq!(first[0].id, "a");

        completed.insert("a".to_string());
        let second = next_runnable(&tasks, &completed, &running, 2);
        assert_eq!(second.len(), 2);
        assert!(second.iter().any(|task| task.id == "b"));
        assert!(second.iter().any(|task| task.id == "c"));
    }
}
