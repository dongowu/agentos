#[derive(Debug, Default, Clone)]
pub struct TraceLog {
    entries: Vec<String>,
}

impl TraceLog {
    pub fn push(&mut self, event: impl Into<String>) {
        self.entries.push(event.into());
    }

    pub fn into_entries(self) -> Vec<String> {
        self.entries
    }
}
