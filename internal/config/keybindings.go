package config

// Keybinding groups beyond global. These are the stable strings users
// put under options.tui.keybindings.<group>. KeybindingGroupGlobal is
// declared next to the original global actions in config.go.
const (
	KeybindingGroupEditor     = "editor"
	KeybindingGroupChat       = "chat"
	KeybindingGroupInitialize = "initialize"
)

// Editor group action names (the message composer surface).
const (
	KeybindActionEditorAddFile              = "add_file"
	KeybindActionEditorSendMessage          = "send_message"
	KeybindActionEditorOpenEditor           = "open_editor"
	KeybindActionEditorNewline              = "newline"
	KeybindActionEditorAddImage             = "add_image"
	KeybindActionEditorPasteImage           = "paste_image"
	KeybindActionEditorMentionFile          = "mention_file"
	KeybindActionEditorCommands             = "commands"
	KeybindActionEditorAttachmentDeleteMode = "attachment_delete_mode"
	KeybindActionEditorEscape               = "escape"
	KeybindActionEditorDeleteAllAttachments = "delete_all_attachments"
	KeybindActionEditorHistoryPrev          = "history_prev"
	KeybindActionEditorHistoryNext          = "history_next"
)

// Chat group action names (the messages list surface).
const (
	KeybindActionChatNewSession     = "new_session"
	KeybindActionChatAddAttachment  = "add_attachment"
	KeybindActionChatCancel         = "cancel"
	KeybindActionChatTab            = "tab"
	KeybindActionChatDetails        = "details"
	KeybindActionChatTogglePills    = "toggle_pills"
	KeybindActionChatPillLeft       = "pill_left"
	KeybindActionChatPillRight      = "pill_right"
	KeybindActionChatDown           = "down"
	KeybindActionChatUp             = "up"
	KeybindActionChatUpDown         = "up_down"
	KeybindActionChatUpOneItem      = "up_one_item"
	KeybindActionChatDownOneItem    = "down_one_item"
	KeybindActionChatUpDownOneItem  = "up_down_one_item"
	KeybindActionChatPageDown       = "page_down"
	KeybindActionChatPageUp         = "page_up"
	KeybindActionChatHalfPageDown   = "half_page_down"
	KeybindActionChatHalfPageUp     = "half_page_up"
	KeybindActionChatHome           = "home"
	KeybindActionChatEnd            = "end"
	KeybindActionChatCopy           = "copy"
	KeybindActionChatClearHighlight = "clear_highlight"
	KeybindActionChatExpand         = "expand"
	KeybindActionChatScrollLeft     = "scroll_left"
	KeybindActionChatScrollRight    = "scroll_right"
)

// Initialize group action names (the first-run onboarding prompt).
const (
	KeybindActionInitializeYes    = "yes"
	KeybindActionInitializeNo     = "no"
	KeybindActionInitializeSwitch = "switch"
	KeybindActionInitializeEnter  = "enter"
)

// KeybindingDescriptor describes one overridable keybinding: its group,
// action name, default keys, and help metadata.
//
// HelpShortcut is the curated key shown in the footer (it can differ
// from Defaults[0] — e.g. "models" binds [ctrl+m, ctrl+l] but shows
// ctrl+l). HelpLabel is the short footer label; an empty HelpLabel means
// the binding has no footer help (e.g. history navigation). Description
// is the longer text shown by `crush keybindings`.
type KeybindingDescriptor struct {
	Group        string
	Action       string
	Defaults     []string
	HelpShortcut string
	HelpLabel    string
	Description  string
}

// KeybindingCatalog is the single source of truth for every overridable
// keybinding. Both the TUI keymap construction (internal/ui, via
// common.Binding) and the `crush keybindings` command read from this
// table, so the defaults and help metadata can never drift between them.
//
// Fields are positional: {Group, Action, Defaults, HelpShortcut, HelpLabel, Description}.
var KeybindingCatalog = []KeybindingDescriptor{
	// global
	{KeybindingGroupGlobal, KeybindActionQuit, []string{"ctrl+c"}, "ctrl+c", "quit", "Quit crush"},
	{KeybindingGroupGlobal, KeybindActionHelp, []string{"ctrl+g"}, "ctrl+g", "more", "Show more help"},
	{KeybindingGroupGlobal, KeybindActionCommands, []string{"ctrl+p"}, "ctrl+p", "commands", "Open the command palette"},
	// ctrl+m (mnemonic) is the advertised model-picker key; ctrl+l stays a
	// working alias but isn't shown, since multiplexers commonly bind it
	// (vim-tmux-navigator's pane-right) so it never reaches us.
	{KeybindingGroupGlobal, KeybindActionModels, []string{"ctrl+m", "ctrl+l"}, "ctrl+m", "models", "Open the model picker"},
	{KeybindingGroupGlobal, KeybindActionSuspend, []string{"ctrl+z"}, "ctrl+z", "suspend", "Suspend crush"},
	{KeybindingGroupGlobal, KeybindActionSessions, []string{"ctrl+s"}, "ctrl+s", "sessions", "Open the sessions list"},
	{KeybindingGroupGlobal, KeybindActionTab, []string{"tab"}, "tab", "change focus", "Change focus"},
	{KeybindingGroupGlobal, KeybindActionToggleYolo, []string{"ctrl+y"}, "ctrl+y", "toggle yolo", "Toggle yolo (auto-approve) mode"},

	// editor (message composer)
	{KeybindingGroupEditor, KeybindActionEditorAddFile, []string{"/"}, "/", "add file", "Add a file to the prompt"},
	{KeybindingGroupEditor, KeybindActionEditorSendMessage, []string{"enter"}, "enter", "send", "Send the message"},
	{KeybindingGroupEditor, KeybindActionEditorOpenEditor, []string{"ctrl+o"}, "ctrl+o", "open editor", "Open the message in $EDITOR"},
	// shift+enter is the advertised newline key; ctrl+j stays as a working
	// alias but isn't shown, since terminal multiplexers commonly bind it
	// (e.g. vim-tmux-navigator's pane-down) and it then never reaches us.
	{KeybindingGroupEditor, KeybindActionEditorNewline, []string{"shift+enter", "ctrl+j"}, "shift+enter", "newline", "Insert a newline"},
	{KeybindingGroupEditor, KeybindActionEditorAddImage, []string{"ctrl+f"}, "ctrl+f", "add image", "Add an image to the prompt"},
	{KeybindingGroupEditor, KeybindActionEditorPasteImage, []string{"ctrl+v"}, "ctrl+v", "paste image from clipboard", "Paste an image from the clipboard"},
	{KeybindingGroupEditor, KeybindActionEditorMentionFile, []string{"@"}, "@", "mention file", "Mention a file with @"},
	{KeybindingGroupEditor, KeybindActionEditorCommands, []string{"/"}, "/", "commands", "Open commands from an empty composer"},
	{KeybindingGroupEditor, KeybindActionEditorAttachmentDeleteMode, []string{"ctrl+r"}, "ctrl+r+{i}", "delete attachment at index i", "Enter attachment delete mode"},
	{KeybindingGroupEditor, KeybindActionEditorEscape, []string{"esc", "alt+esc"}, "esc", "cancel delete mode", "Cancel attachment delete mode"},
	{KeybindingGroupEditor, KeybindActionEditorDeleteAllAttachments, []string{"r"}, "ctrl+r+r", "delete all attachments", "Delete all attachments"},
	{KeybindingGroupEditor, KeybindActionEditorHistoryPrev, []string{"up"}, "", "", "Recall the previous prompt from history"},
	{KeybindingGroupEditor, KeybindActionEditorHistoryNext, []string{"down"}, "", "", "Recall the next prompt from history"},

	// chat (messages list)
	{KeybindingGroupChat, KeybindActionChatNewSession, []string{"ctrl+n"}, "ctrl+n", "new session", "Start a new session"},
	{KeybindingGroupChat, KeybindActionChatAddAttachment, []string{"ctrl+f"}, "ctrl+f", "add attachment", "Add an attachment"},
	{KeybindingGroupChat, KeybindActionChatCancel, []string{"esc", "alt+esc"}, "esc", "cancel", "Cancel the running task"},
	{KeybindingGroupChat, KeybindActionChatTab, []string{"tab"}, "tab", "change focus", "Change focus"},
	{KeybindingGroupChat, KeybindActionChatDetails, []string{"ctrl+d"}, "ctrl+d", "toggle details", "Toggle message details"},
	{KeybindingGroupChat, KeybindActionChatTogglePills, []string{"ctrl+t", "ctrl+space"}, "ctrl+t", "toggle tasks", "Toggle the tasks panel"},
	{KeybindingGroupChat, KeybindActionChatPillLeft, []string{"left"}, "←/→", "switch section", "Switch section left"},
	{KeybindingGroupChat, KeybindActionChatPillRight, []string{"right"}, "←/→", "switch section", "Switch section right"},
	{KeybindingGroupChat, KeybindActionChatDown, []string{"down", "ctrl+j", "j"}, "↓", "down", "Scroll down"},
	{KeybindingGroupChat, KeybindActionChatUp, []string{"up", "ctrl+k", "k"}, "↑", "up", "Scroll up"},
	{KeybindingGroupChat, KeybindActionChatUpDown, []string{"up", "down"}, "↑↓", "scroll", "Scroll up/down"},
	{KeybindingGroupChat, KeybindActionChatUpOneItem, []string{"shift+up", "K"}, "shift+↑", "up one item", "Move up one item"},
	{KeybindingGroupChat, KeybindActionChatDownOneItem, []string{"shift+down", "J"}, "shift+↓", "down one item", "Move down one item"},
	{KeybindingGroupChat, KeybindActionChatUpDownOneItem, []string{"shift+up", "shift+down"}, "shift+↑↓", "scroll one item", "Scroll one item at a time"},
	{KeybindingGroupChat, KeybindActionChatPageDown, []string{"pgdown", " ", "f"}, "f/pgdn", "page down", "Page down"},
	{KeybindingGroupChat, KeybindActionChatPageUp, []string{"pgup", "b"}, "b/pgup", "page up", "Page up"},
	{KeybindingGroupChat, KeybindActionChatHalfPageDown, []string{"d", "ctrl+d"}, "d", "half page down", "Half page down"},
	{KeybindingGroupChat, KeybindActionChatHalfPageUp, []string{"u", "ctrl+u"}, "u", "half page up", "Half page up"},
	{KeybindingGroupChat, KeybindActionChatHome, []string{"g", "home"}, "g", "home", "Go to the top"},
	{KeybindingGroupChat, KeybindActionChatEnd, []string{"G", "end"}, "G", "end", "Go to the bottom"},
	{KeybindingGroupChat, KeybindActionChatCopy, []string{"c", "y", "C", "Y"}, "c/y", "copy", "Copy the selection"},
	{KeybindingGroupChat, KeybindActionChatClearHighlight, []string{"esc", "alt+esc"}, "esc", "clear selection", "Clear the selection"},
	{KeybindingGroupChat, KeybindActionChatExpand, []string{"space"}, "space", "expand/collapse", "Expand or collapse the focused item"},
	{KeybindingGroupChat, KeybindActionChatScrollLeft, []string{"shift+left", "H"}, "shift+←/H", "scroll left", "Scroll the viewport left"},
	{KeybindingGroupChat, KeybindActionChatScrollRight, []string{"shift+right", "L"}, "shift+→/L", "scroll right", "Scroll the viewport right"},

	// initialize (onboarding prompt)
	{KeybindingGroupInitialize, KeybindActionInitializeYes, []string{"y", "Y"}, "y", "yes", "Confirm yes"},
	{KeybindingGroupInitialize, KeybindActionInitializeNo, []string{"n", "N", "esc", "alt+esc"}, "n", "no", "Decline"},
	{KeybindingGroupInitialize, KeybindActionInitializeSwitch, []string{"left", "right", "tab"}, "tab", "switch", "Switch the highlighted option"},
	{KeybindingGroupInitialize, KeybindActionInitializeEnter, []string{"enter"}, "enter", "select", "Select the highlighted option"},
}

// LookupKeybinding returns the catalog descriptor for group.action.
func LookupKeybinding(group, action string) (KeybindingDescriptor, bool) {
	for i := range KeybindingCatalog {
		if KeybindingCatalog[i].Group == group && KeybindingCatalog[i].Action == action {
			return KeybindingCatalog[i], true
		}
	}
	return KeybindingDescriptor{}, false
}

// knownKeybindings returns the set of recognized group→action names,
// derived from the catalog. Used by ValidateKeybindings.
func knownKeybindings() map[string]map[string]bool {
	known := make(map[string]map[string]bool)
	for _, d := range KeybindingCatalog {
		if known[d.Group] == nil {
			known[d.Group] = make(map[string]bool)
		}
		known[d.Group][d.Action] = true
	}
	return known
}
