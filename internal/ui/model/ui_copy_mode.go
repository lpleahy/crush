package model

import (
	tea "charm.land/bubbletea/v2"

	"github.com/charmbracelet/crush/internal/ui/common"
	"github.com/charmbracelet/crush/internal/ui/util"
)

// handleCopyModeKey routes a key to the conversation copy mode. A yank
// copies the clean selection to the OS clipboard (OSC 52 + native) and
// returns to the composer; a plain Esc exit leaves focus on the chat.
func (m *UI) handleCopyModeKey(msg tea.KeyMsg) tea.Cmd {
	yanked, exit := m.chat.HandleCopyModeKey(msg.String())
	var cmds []tea.Cmd
	if yanked != "" {
		cmds = append(cmds,
			common.SetSystemClipboard(yanked),
			util.ReportInfo("Copied selection to clipboard"),
		)
		// Hop straight back to the input side after copying.
		m.focus = uiFocusEditor
		m.chat.Blur()
		cmds = append(cmds, m.textarea.Focus())
	}
	_ = exit // esc-exit just leaves copy mode; focus stays on the chat
	return tea.Batch(cmds...)
}
