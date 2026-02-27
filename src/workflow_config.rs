use std::collections::HashMap;
use std::fs;
use std::path::Path;

use anyhow::{Context, Result};
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkflowCatalog {
    pub workflows: HashMap<String, WorkflowDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkflowDefinition {
    pub name: String,
    pub stages: Vec<StageDefinition>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StageDefinition {
    pub id: String,
    pub name: String,
    pub agent: String,
    #[serde(default)]
    pub collaborators: Vec<String>,
    #[serde(default, alias = "humanGate")]
    pub human_gate: bool,
    #[serde(default)]
    pub inputs: Vec<StageInput>,
    #[serde(default)]
    pub outputs: Vec<StageOutputSpec>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StageInput {
    #[serde(alias = "fromStage")]
    pub from_stage: String,
    #[serde(alias = "type")]
    pub kind: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StageOutputSpec {
    #[serde(alias = "type")]
    pub kind: String,
    pub name: String,
}

impl WorkflowCatalog {
    pub fn load_or_default(path: Option<&Path>) -> Result<Self> {
        if let Some(path) = path {
            if path.exists() {
                return Self::load_from_yaml(path);
            }
        }
        Ok(Self::default_catalog())
    }

    pub fn load_from_yaml(path: &Path) -> Result<Self> {
        let raw = fs::read_to_string(path)
            .with_context(|| format!("failed to read workflow file {}", path.display()))?;
        serde_yaml::from_str::<WorkflowCatalog>(&raw)
            .with_context(|| format!("failed to parse workflow yaml {}", path.display()))
    }

    pub fn get(&self, workflow_id: &str) -> Option<&WorkflowDefinition> {
        self.workflows.get(workflow_id)
    }

    fn default_catalog() -> Self {
        let mut workflows = HashMap::new();
        workflows.insert("mvp".to_string(), default_mvp());
        workflows.insert("default".to_string(), default_full());
        workflows.insert("autonomy".to_string(), default_autonomy());
        Self { workflows }
    }
}

fn default_mvp() -> WorkflowDefinition {
    WorkflowDefinition {
        name: "MVP".to_string(),
        stages: vec![
            StageDefinition {
                id: "prd".to_string(),
                name: "PRD".to_string(),
                agent: "pm".to_string(),
                collaborators: Vec::new(),
                human_gate: false,
                inputs: Vec::new(),
                outputs: vec![StageOutputSpec {
                    kind: "prd".to_string(),
                    name: "PRD.md".to_string(),
                }],
            },
            StageDefinition {
                id: "implementation".to_string(),
                name: "Implementation".to_string(),
                agent: "coder".to_string(),
                collaborators: vec!["pm".to_string()],
                human_gate: false,
                inputs: vec![StageInput {
                    from_stage: "prd".to_string(),
                    kind: "prd".to_string(),
                }],
                outputs: vec![StageOutputSpec {
                    kind: "source-code".to_string(),
                    name: "source.md".to_string(),
                }],
            },
        ],
    }
}

fn default_full() -> WorkflowDefinition {
    WorkflowDefinition {
        name: "Default".to_string(),
        stages: vec![
            StageDefinition {
                id: "prd".to_string(),
                name: "PRD".to_string(),
                agent: "pm".to_string(),
                collaborators: vec!["architect".to_string()],
                human_gate: false,
                inputs: Vec::new(),
                outputs: vec![
                    StageOutputSpec {
                        kind: "prd".to_string(),
                        name: "PRD.md".to_string(),
                    },
                    StageOutputSpec {
                        kind: "acceptance-criteria".to_string(),
                        name: "acceptance-criteria.md".to_string(),
                    },
                ],
            },
            StageDefinition {
                id: "alignment".to_string(),
                name: "Alignment".to_string(),
                agent: "architect".to_string(),
                collaborators: vec![
                    "pm".to_string(),
                    "coder".to_string(),
                    "reviewer".to_string(),
                    "tester".to_string(),
                ],
                human_gate: false,
                inputs: vec![
                    StageInput {
                        from_stage: "prd".to_string(),
                        kind: "prd".to_string(),
                    },
                    StageInput {
                        from_stage: "prd".to_string(),
                        kind: "acceptance-criteria".to_string(),
                    },
                ],
                outputs: vec![StageOutputSpec {
                    kind: "discussion-summary".to_string(),
                    name: "alignment-summary.md".to_string(),
                }],
            },
            StageDefinition {
                id: "design".to_string(),
                name: "Design".to_string(),
                agent: "architect".to_string(),
                collaborators: vec!["coder".to_string()],
                human_gate: false,
                inputs: vec![StageInput {
                    from_stage: "alignment".to_string(),
                    kind: "discussion-summary".to_string(),
                }],
                outputs: vec![
                    StageOutputSpec {
                        kind: "tech-design".to_string(),
                        name: "tech-design.md".to_string(),
                    },
                    StageOutputSpec {
                        kind: "task-list".to_string(),
                        name: "tasks.md".to_string(),
                    },
                ],
            },
            StageDefinition {
                id: "implementation".to_string(),
                name: "Implementation".to_string(),
                agent: "coder".to_string(),
                collaborators: vec!["architect".to_string()],
                human_gate: false,
                inputs: vec![
                    StageInput {
                        from_stage: "design".to_string(),
                        kind: "tech-design".to_string(),
                    },
                    StageInput {
                        from_stage: "design".to_string(),
                        kind: "task-list".to_string(),
                    },
                ],
                outputs: vec![StageOutputSpec {
                    kind: "source-code".to_string(),
                    name: "source.md".to_string(),
                }],
            },
            StageDefinition {
                id: "review".to_string(),
                name: "Review".to_string(),
                agent: "reviewer".to_string(),
                collaborators: vec!["coder".to_string()],
                human_gate: false,
                inputs: vec![StageInput {
                    from_stage: "implementation".to_string(),
                    kind: "source-code".to_string(),
                }],
                outputs: vec![StageOutputSpec {
                    kind: "review-report".to_string(),
                    name: "review.md".to_string(),
                }],
            },
            StageDefinition {
                id: "testing".to_string(),
                name: "Testing".to_string(),
                agent: "tester".to_string(),
                collaborators: vec!["coder".to_string()],
                human_gate: false,
                inputs: vec![StageInput {
                    from_stage: "implementation".to_string(),
                    kind: "source-code".to_string(),
                }],
                outputs: vec![StageOutputSpec {
                    kind: "test-report".to_string(),
                    name: "testing.md".to_string(),
                }],
            },
        ],
    }
}

fn default_autonomy() -> WorkflowDefinition {
    WorkflowDefinition {
        name: "Autonomy".to_string(),
        stages: vec![
            StageDefinition {
                id: "implementation".to_string(),
                name: "Implementation".to_string(),
                agent: "coder".to_string(),
                collaborators: vec!["architect".to_string()],
                human_gate: false,
                inputs: Vec::new(),
                outputs: vec![StageOutputSpec {
                    kind: "source-code".to_string(),
                    name: "source.md".to_string(),
                }],
            },
            StageDefinition {
                id: "review".to_string(),
                name: "Review".to_string(),
                agent: "reviewer".to_string(),
                collaborators: vec!["coder".to_string()],
                human_gate: false,
                inputs: vec![StageInput {
                    from_stage: "implementation".to_string(),
                    kind: "source-code".to_string(),
                }],
                outputs: vec![StageOutputSpec {
                    kind: "review-report".to_string(),
                    name: "review.md".to_string(),
                }],
            },
            StageDefinition {
                id: "testing".to_string(),
                name: "Testing".to_string(),
                agent: "tester".to_string(),
                collaborators: vec!["coder".to_string()],
                human_gate: false,
                inputs: vec![
                    StageInput {
                        from_stage: "implementation".to_string(),
                        kind: "source-code".to_string(),
                    },
                    StageInput {
                        from_stage: "review".to_string(),
                        kind: "review-report".to_string(),
                    },
                ],
                outputs: vec![StageOutputSpec {
                    kind: "test-report".to_string(),
                    name: "testing.md".to_string(),
                }],
            },
        ],
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use super::WorkflowCatalog;

    #[test]
    fn loads_default_catalog_when_no_file() {
        let catalog = WorkflowCatalog::load_or_default(None).expect("catalog");
        assert!(catalog.get("mvp").is_some());
        assert!(catalog.get("default").is_some());
        assert!(catalog.get("autonomy").is_some());
    }

    #[test]
    fn loads_catalog_from_yaml_file() {
        let dir = tempfile::tempdir().expect("tempdir");
        let file = dir.path().join("workflows.yaml");
        fs::write(
            &file,
            r#"
workflows:
  custom:
    name: Custom
    stages:
      - id: plan
        name: Plan
        agent: pm
        collaborators: [architect]
        humanGate: true
        outputs:
          - type: plan-doc
            name: plan.md
"#,
        )
        .expect("write yaml");

        let catalog = WorkflowCatalog::load_from_yaml(&file).expect("load");
        let workflow = catalog.get("custom").expect("workflow");
        assert_eq!(workflow.stages.len(), 1);
        assert!(workflow.stages[0].human_gate);
        assert_eq!(workflow.stages[0].outputs[0].kind, "plan-doc");
    }
}
