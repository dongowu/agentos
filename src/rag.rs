use std::collections::{HashMap, HashSet};

#[derive(Debug, Clone)]
pub struct RagDocument {
    pub kind: String,
    pub name: String,
    pub content: String,
}

#[derive(Debug, Clone)]
pub struct RagSnippet {
    pub kind: String,
    pub name: String,
    pub score: f64,
    pub content: String,
}

#[derive(Debug, Clone)]
pub struct RagRetriever {
    top_k: usize,
    snippet_chars: usize,
    min_score: f64,
}

impl RagRetriever {
    pub fn new(top_k: usize, snippet_chars: usize, min_score: f64) -> Self {
        Self {
            top_k,
            snippet_chars,
            min_score,
        }
    }

    pub fn retrieve(&self, query: &str, docs: &[RagDocument]) -> Vec<RagSnippet> {
        let query_tokens = tokenize(query);
        if query_tokens.is_empty() {
            return Vec::new();
        }
        let query_set = query_tokens.iter().cloned().collect::<HashSet<_>>();
        let mut scored = Vec::new();

        for doc in docs {
            let doc_tokens = tokenize(&doc.content);
            if doc_tokens.is_empty() {
                continue;
            }

            let mut tf = HashMap::<String, usize>::new();
            for token in &doc_tokens {
                *tf.entry(token.clone()).or_insert(0) += 1;
            }

            let overlap = query_set.iter().filter(|qt| tf.contains_key(*qt)).count();
            let overlap_score = overlap as f64 / query_set.len() as f64;

            let prefix_bonus = query_set
                .iter()
                .filter(|qt| {
                    tf.keys()
                        .any(|dt| dt.starts_with((*qt).as_str()) || qt.starts_with(dt))
                })
                .count() as f64
                / query_set.len() as f64
                * 0.3;

            let score = overlap_score + prefix_bonus;
            if score < self.min_score {
                continue;
            }

            scored.push((score, doc));
        }

        scored.sort_by(|a, b| b.0.partial_cmp(&a.0).unwrap_or(std::cmp::Ordering::Equal));

        scored
            .into_iter()
            .take(self.top_k)
            .map(|(score, doc)| RagSnippet {
                kind: doc.kind.clone(),
                name: doc.name.clone(),
                score,
                content: truncate(&doc.content, self.snippet_chars),
            })
            .collect()
    }
}

impl Default for RagRetriever {
    fn default() -> Self {
        Self::new(6, 700, 0.25)
    }
}

fn tokenize(input: &str) -> Vec<String> {
    input
        .to_lowercase()
        .split(|c: char| !c.is_ascii_alphanumeric())
        .filter(|s| !s.is_empty())
        .map(stem)
        .collect()
}

fn stem(token: &str) -> String {
    if token.len() > 4 && token.ends_with("ing") {
        return token.trim_end_matches("ing").to_string();
    }
    if token.len() > 3 && token.ends_with("ed") {
        return token.trim_end_matches("ed").to_string();
    }
    if token.len() > 3 && token.ends_with('s') {
        return token.trim_end_matches('s').to_string();
    }
    token.to_string()
}

fn truncate(content: &str, max_chars: usize) -> String {
    if content.chars().count() <= max_chars {
        return content.to_string();
    }
    content.chars().take(max_chars).collect()
}

#[cfg(test)]
mod tests {
    use super::{RagDocument, RagRetriever};

    #[test]
    fn retrieves_top_relevant_documents() {
        let retriever = RagRetriever::default();
        let docs = vec![
            RagDocument {
                kind: "note".to_string(),
                name: "db".to_string(),
                content: "Database migration rollback strategy and risk handling".to_string(),
            },
            RagDocument {
                kind: "note".to_string(),
                name: "ui".to_string(),
                content: "Landing page color palette and typography guidelines".to_string(),
            },
        ];

        let hits = retriever.retrieve("rollback risk", &docs);
        assert_eq!(hits.len(), 1);
        assert_eq!(hits[0].name, "db");
        assert!(hits[0].score > 0.3);
    }
}
