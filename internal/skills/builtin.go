package skills

func builtinDefinitions() []Definition {
	return []Definition{
		{
			Name:           "command.exec",
			RuntimeProfile: "default",
			Fields:         []FieldDefinition{{Names: []string{"cmd", "command"}, Type: "string", Required: true}},
		},
		{
			Name:           "file.read",
			RuntimeProfile: "default",
			Fields:         []FieldDefinition{{Names: []string{"path"}, Type: "string", Required: true}},
		},
		{
			Name:           "file.write",
			RuntimeProfile: "default",
			Fields: []FieldDefinition{
				{Names: []string{"path"}, Type: "string", Required: true},
				{Names: []string{"content"}, Type: "string", Required: true},
			},
		},
		{
			Name:           "http.request",
			RuntimeProfile: "default",
			Fields: []FieldDefinition{
				{Names: []string{"url"}, Type: "string", Required: true},
				{Names: []string{"method"}, Type: "string", Required: false},
			},
		},
		{
			Name:           "browser.step",
			RuntimeProfile: "browser",
			Fields:         nil,
		},
	}
}
