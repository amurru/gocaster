package gocaster

import tea "charm.land/bubbletea/v2"

func InitialModel() model {
	return model{}
}

func (m model) Init() tea.Cmd {
	// Just return `nil`, which means "no I/O right now, please."
	return nil
}
