package config

// Dialog and completion keybinding groups. These cover the overlay
// dialogs (model picker, sessions, permissions, etc.), the autocomplete
// popup, and the shared dialog-close key. KeybindingGroupGlobal and the
// editor/chat/initialize groups live in keybindings.go.
const (
	KeybindingGroupDialog        = "dialog"
	KeybindingGroupCompletions   = "completions"
	KeybindingGroupModels        = "models"
	KeybindingGroupSessions      = "sessions"
	KeybindingGroupCommands      = "commands"
	KeybindingGroupFilePicker    = "filepicker"
	KeybindingGroupArguments     = "arguments"
	KeybindingGroupPermissions   = "permissions"
	KeybindingGroupReasoning     = "reasoning"
	KeybindingGroupNotifications = "notifications"
	KeybindingGroupQuit          = "quit"
	KeybindingGroupOAuth         = "oauth"
	KeybindingGroupAPIKey        = "api_key"
)

// dialogKeybindings holds the catalog entries for every dialog and the
// completions popup. They're appended to KeybindingCatalog in init so the
// catalog stays the single source of truth for the TUI and the
// `crush keybindings` listing.
//
// Fields are positional: {Group, Action, Defaults, HelpShortcut, HelpLabel, Description}.
var dialogKeybindings = []KeybindingDescriptor{
	// dialog (shared close binding, used by most overlay dialogs)
	{KeybindingGroupDialog, "close", []string{"esc", "alt+esc"}, "esc", "exit", "Close the dialog"},

	// completions (the @-mention / autocomplete popup)
	{KeybindingGroupCompletions, "down", []string{"down"}, "down", "move down", "Move down in completions"},
	{KeybindingGroupCompletions, "up", []string{"up"}, "up", "move up", "Move up in completions"},
	{KeybindingGroupCompletions, "select", []string{"enter", "tab", "ctrl+y"}, "enter", "select", "Select the completion"},
	{KeybindingGroupCompletions, "cancel", []string{"esc", "alt+esc"}, "esc", "cancel", "Dismiss completions"},
	{KeybindingGroupCompletions, "down_insert", []string{"ctrl+n"}, "ctrl+n", "insert next", "Insert next completion"},
	{KeybindingGroupCompletions, "up_insert", []string{"ctrl+p"}, "ctrl+p", "insert previous", "Insert previous completion"},

	// models (the model picker dialog)
	{KeybindingGroupModels, "tab", []string{"tab", "shift+tab"}, "tab", "toggle type", "Toggle large/small model"},
	{KeybindingGroupModels, "select", []string{"enter", "ctrl+y"}, "enter", "confirm", "Confirm the model"},
	{KeybindingGroupModels, "edit", []string{"ctrl+e"}, "ctrl+e", "edit", "Edit the model config"},
	{KeybindingGroupModels, "up_down", []string{"up", "down"}, "↑/↓", "choose", "Move through the list"},
	{KeybindingGroupModels, "next", []string{"down", "ctrl+n"}, "↓", "next item", "Next item"},
	{KeybindingGroupModels, "previous", []string{"up", "ctrl+p"}, "↑", "previous item", "Previous item"},

	// sessions (the sessions list dialog)
	{KeybindingGroupSessions, "select", []string{"enter", "tab", "ctrl+y"}, "enter", "choose", "Open the session"},
	{KeybindingGroupSessions, "next", []string{"down", "ctrl+n"}, "↓", "next item", "Next item"},
	{KeybindingGroupSessions, "previous", []string{"up", "ctrl+p"}, "↑", "previous item", "Previous item"},
	{KeybindingGroupSessions, "up_down", []string{"up", "down"}, "↑↓", "choose", "Move through the list"},
	{KeybindingGroupSessions, "delete", []string{"ctrl+x"}, "ctrl+x", "delete", "Delete the session"},
	{KeybindingGroupSessions, "rename", []string{"ctrl+r"}, "ctrl+r", "rename", "Rename the session"},
	{KeybindingGroupSessions, "confirm_rename", []string{"enter"}, "enter", "confirm", "Confirm the rename"},
	{KeybindingGroupSessions, "cancel_rename", []string{"esc"}, "esc", "cancel", "Cancel the rename"},
	{KeybindingGroupSessions, "confirm_delete", []string{"y"}, "y", "delete", "Confirm the delete"},
	{KeybindingGroupSessions, "cancel_delete", []string{"n", "esc"}, "n", "cancel", "Cancel the delete"},

	// commands (the command palette dialog)
	{KeybindingGroupCommands, "select", []string{"enter", "ctrl+y"}, "enter", "confirm", "Run the command"},
	{KeybindingGroupCommands, "up_down", []string{"up", "down"}, "↑/↓", "choose", "Move through the list"},
	{KeybindingGroupCommands, "next", []string{"down"}, "↓", "next item", "Next item"},
	{KeybindingGroupCommands, "previous", []string{"up", "ctrl+p"}, "↑", "previous item", "Previous item"},
	{KeybindingGroupCommands, "tab", []string{"tab"}, "tab", "switch selection", "Switch selection"},
	{KeybindingGroupCommands, "shift_tab", []string{"shift+tab"}, "shift+tab", "switch selection prev", "Switch selection (reverse)"},

	// filepicker (the file picker dialog — its own close)
	{KeybindingGroupFilePicker, "select", []string{"enter"}, "enter", "accept", "Accept the file"},
	{KeybindingGroupFilePicker, "down", []string{"down", "j"}, "down/j", "move down", "Move down"},
	{KeybindingGroupFilePicker, "up", []string{"up", "k"}, "up/k", "move up", "Move up"},
	{KeybindingGroupFilePicker, "forward", []string{"right", "l"}, "right/l", "move forward", "Enter the directory"},
	{KeybindingGroupFilePicker, "backward", []string{"left", "h"}, "left/h", "move backward", "Go up a directory"},
	{KeybindingGroupFilePicker, "navigate", []string{"right", "l", "left", "h", "up", "k", "down", "j"}, "↑↓←→", "navigate", "Navigate the tree"},
	{KeybindingGroupFilePicker, "close", []string{"esc", "alt+esc"}, "esc", "close/exit", "Close the file picker"},

	// arguments (the command-arguments prompt)
	{KeybindingGroupArguments, "confirm", []string{"enter"}, "enter", "confirm", "Confirm the arguments"},
	{KeybindingGroupArguments, "next", []string{"down", "tab"}, "↓/tab", "next", "Next field"},
	{KeybindingGroupArguments, "previous", []string{"up", "shift+tab"}, "↑/shift+tab", "previous", "Previous field"},

	// permissions (the tool-permission prompt)
	{KeybindingGroupPermissions, "left", []string{"left", "h"}, "←", "previous", "Previous option"},
	{KeybindingGroupPermissions, "right", []string{"right", "l"}, "→", "next", "Next option"},
	{KeybindingGroupPermissions, "tab", []string{"tab"}, "tab", "next option", "Next option"},
	{KeybindingGroupPermissions, "select", []string{"enter", "ctrl+y"}, "enter", "confirm", "Confirm the choice"},
	{KeybindingGroupPermissions, "allow", []string{"a", "A", "ctrl+a"}, "a", "allow", "Allow once"},
	{KeybindingGroupPermissions, "allow_session", []string{"s", "S", "ctrl+s"}, "s", "allow session", "Allow for the session"},
	{KeybindingGroupPermissions, "deny", []string{"d", "D"}, "d", "deny", "Deny"},
	{KeybindingGroupPermissions, "toggle_diff_mode", []string{"t"}, "t", "toggle diff view", "Toggle the diff view"},
	{KeybindingGroupPermissions, "toggle_fullscreen", []string{"f"}, "f", "toggle fullscreen", "Toggle fullscreen"},
	{KeybindingGroupPermissions, "scroll_up", []string{"shift+up", "K"}, "shift+↑", "scroll up", "Scroll the diff up"},
	{KeybindingGroupPermissions, "scroll_down", []string{"shift+down", "J"}, "shift+↓", "scroll down", "Scroll the diff down"},
	{KeybindingGroupPermissions, "scroll_left", []string{"shift+left", "H"}, "shift+←", "scroll left", "Scroll the diff left"},
	{KeybindingGroupPermissions, "scroll_right", []string{"shift+right", "L"}, "shift+→", "scroll right", "Scroll the diff right"},
	{KeybindingGroupPermissions, "choose", []string{"left", "right"}, "←/→", "choose", "Choose an option"},
	{KeybindingGroupPermissions, "scroll", []string{"shift+left", "shift+down", "shift+up", "shift+right"}, "shift+←↓↑→", "scroll", "Scroll the diff"},

	// reasoning (the reasoning-effort picker)
	{KeybindingGroupReasoning, "select", []string{"enter", "ctrl+y"}, "enter", "confirm", "Confirm the effort"},
	{KeybindingGroupReasoning, "next", []string{"down", "ctrl+n"}, "↓", "next item", "Next item"},
	{KeybindingGroupReasoning, "previous", []string{"up", "ctrl+p"}, "↑", "previous item", "Previous item"},
	{KeybindingGroupReasoning, "up_down", []string{"up", "down"}, "↑/↓", "choose", "Move through the list"},

	// notifications (the notification settings picker)
	{KeybindingGroupNotifications, "select", []string{"enter", "ctrl+y"}, "enter", "confirm", "Confirm the choice"},
	{KeybindingGroupNotifications, "next", []string{"down", "ctrl+n"}, "↓", "next item", "Next item"},
	{KeybindingGroupNotifications, "previous", []string{"up", "ctrl+p"}, "↑", "previous item", "Previous item"},
	{KeybindingGroupNotifications, "up_down", []string{"up", "down"}, "↑/↓", "choose", "Move through the list"},

	// quit (the quit-confirmation dialog)
	{KeybindingGroupQuit, "left_right", []string{"left", "right"}, "←/→", "switch options", "Switch options"},
	{KeybindingGroupQuit, "enter_space", []string{"enter", " "}, "enter/space", "confirm", "Confirm the choice"},
	{KeybindingGroupQuit, "yes", []string{"y", "Y", "ctrl+c"}, "y/Y/ctrl+c", "yes", "Quit"},
	{KeybindingGroupQuit, "no", []string{"n", "N"}, "n/N", "no", "Cancel quit"},
	{KeybindingGroupQuit, "tab", []string{"tab"}, "tab", "switch options", "Switch options"},
	{KeybindingGroupQuit, "quit", []string{"ctrl+c"}, "ctrl+c", "quit", "Force quit"},

	// oauth (the device-flow sign-in dialog)
	{KeybindingGroupOAuth, "copy", []string{"c"}, "c", "copy code", "Copy the device code"},
	{KeybindingGroupOAuth, "submit", []string{"enter", "ctrl+y"}, "enter", "copy & open", "Copy the code and open the URL"},
	{KeybindingGroupOAuth, "finish", []string{"enter", "ctrl+y", "esc"}, "enter", "finish", "Finish sign-in and close the dialog"},

	// api_key (the API-key entry dialog)
	{KeybindingGroupAPIKey, "submit", []string{"enter", "ctrl+y"}, "enter", "submit", "Submit the API key"},
}

func init() {
	KeybindingCatalog = append(KeybindingCatalog, dialogKeybindings...)
}
