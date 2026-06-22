package model

import (
	"charm.land/bubbles/v2/key"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/ui/common"
)

type KeyMap struct {
	Editor struct {
		AddFile     key.Binding
		SendMessage key.Binding
		OpenEditor  key.Binding
		Newline     key.Binding
		AddImage    key.Binding
		PasteImage  key.Binding
		MentionFile key.Binding
		Commands    key.Binding

		// Attachments key maps
		AttachmentDeleteMode key.Binding
		Escape               key.Binding
		DeleteAllAttachments key.Binding

		// History navigation
		HistoryPrev key.Binding
		HistoryNext key.Binding
	}

	Chat struct {
		NewSession     key.Binding
		AddAttachment  key.Binding
		Cancel         key.Binding
		Tab            key.Binding
		Details        key.Binding
		TogglePills    key.Binding
		PillLeft       key.Binding
		PillRight      key.Binding
		Down           key.Binding
		Up             key.Binding
		UpDown         key.Binding
		DownOneItem    key.Binding
		UpOneItem      key.Binding
		UpDownOneItem  key.Binding
		PageDown       key.Binding
		PageUp         key.Binding
		HalfPageDown   key.Binding
		HalfPageUp     key.Binding
		Home           key.Binding
		End            key.Binding
		Copy           key.Binding
		ClearHighlight key.Binding
		Expand         key.Binding
		ScrollLeft     key.Binding
		ScrollRight    key.Binding
	}

	Initialize struct {
		Yes,
		No,
		Enter,
		Switch key.Binding
	}

	// Global key maps
	Quit       key.Binding
	Help       key.Binding
	Commands   key.Binding
	Models     key.Binding
	Suspend    key.Binding
	Sessions   key.Binding
	Tab        key.Binding
	ToggleYolo key.Binding
}

// DefaultKeyMap builds the keymap, applying any keybinding overrides
// from cfg (options.tui.keybindings.<group>). Every binding's defaults
// and help metadata come from config.KeybindingCatalog, so the keys here
// stay in lockstep with the `crush keybindings` listing. Pass nil for
// pure defaults (tests, headless).
func DefaultKeyMap(cfg *config.Config) KeyMap {
	b := func(group, action string) key.Binding {
		return common.Binding(cfg, group, action)
	}

	var km KeyMap

	// Global
	km.Quit = b(config.KeybindingGroupGlobal, config.KeybindActionQuit)
	km.Help = b(config.KeybindingGroupGlobal, config.KeybindActionHelp)
	km.Commands = b(config.KeybindingGroupGlobal, config.KeybindActionCommands)
	km.Models = b(config.KeybindingGroupGlobal, config.KeybindActionModels)
	km.Suspend = b(config.KeybindingGroupGlobal, config.KeybindActionSuspend)
	km.Sessions = b(config.KeybindingGroupGlobal, config.KeybindActionSessions)
	km.Tab = b(config.KeybindingGroupGlobal, config.KeybindActionTab)
	km.ToggleYolo = b(config.KeybindingGroupGlobal, config.KeybindActionToggleYolo)

	// Editor
	km.Editor.AddFile = b(config.KeybindingGroupEditor, config.KeybindActionEditorAddFile)
	km.Editor.SendMessage = b(config.KeybindingGroupEditor, config.KeybindActionEditorSendMessage)
	km.Editor.OpenEditor = b(config.KeybindingGroupEditor, config.KeybindActionEditorOpenEditor)
	km.Editor.Newline = b(config.KeybindingGroupEditor, config.KeybindActionEditorNewline)
	km.Editor.AddImage = b(config.KeybindingGroupEditor, config.KeybindActionEditorAddImage)
	km.Editor.PasteImage = b(config.KeybindingGroupEditor, config.KeybindActionEditorPasteImage)
	km.Editor.MentionFile = b(config.KeybindingGroupEditor, config.KeybindActionEditorMentionFile)
	km.Editor.Commands = b(config.KeybindingGroupEditor, config.KeybindActionEditorCommands)
	km.Editor.AttachmentDeleteMode = b(config.KeybindingGroupEditor, config.KeybindActionEditorAttachmentDeleteMode)
	km.Editor.Escape = b(config.KeybindingGroupEditor, config.KeybindActionEditorEscape)
	km.Editor.DeleteAllAttachments = b(config.KeybindingGroupEditor, config.KeybindActionEditorDeleteAllAttachments)
	km.Editor.HistoryPrev = b(config.KeybindingGroupEditor, config.KeybindActionEditorHistoryPrev)
	km.Editor.HistoryNext = b(config.KeybindingGroupEditor, config.KeybindActionEditorHistoryNext)

	// Chat
	km.Chat.NewSession = b(config.KeybindingGroupChat, config.KeybindActionChatNewSession)
	km.Chat.AddAttachment = b(config.KeybindingGroupChat, config.KeybindActionChatAddAttachment)
	km.Chat.Cancel = b(config.KeybindingGroupChat, config.KeybindActionChatCancel)
	km.Chat.Tab = b(config.KeybindingGroupChat, config.KeybindActionChatTab)
	km.Chat.Details = b(config.KeybindingGroupChat, config.KeybindActionChatDetails)
	km.Chat.TogglePills = b(config.KeybindingGroupChat, config.KeybindActionChatTogglePills)
	km.Chat.PillLeft = b(config.KeybindingGroupChat, config.KeybindActionChatPillLeft)
	km.Chat.PillRight = b(config.KeybindingGroupChat, config.KeybindActionChatPillRight)
	km.Chat.Down = b(config.KeybindingGroupChat, config.KeybindActionChatDown)
	km.Chat.Up = b(config.KeybindingGroupChat, config.KeybindActionChatUp)
	km.Chat.UpDown = b(config.KeybindingGroupChat, config.KeybindActionChatUpDown)
	km.Chat.DownOneItem = b(config.KeybindingGroupChat, config.KeybindActionChatDownOneItem)
	km.Chat.UpOneItem = b(config.KeybindingGroupChat, config.KeybindActionChatUpOneItem)
	km.Chat.UpDownOneItem = b(config.KeybindingGroupChat, config.KeybindActionChatUpDownOneItem)
	km.Chat.PageDown = b(config.KeybindingGroupChat, config.KeybindActionChatPageDown)
	km.Chat.PageUp = b(config.KeybindingGroupChat, config.KeybindActionChatPageUp)
	km.Chat.HalfPageDown = b(config.KeybindingGroupChat, config.KeybindActionChatHalfPageDown)
	km.Chat.HalfPageUp = b(config.KeybindingGroupChat, config.KeybindActionChatHalfPageUp)
	km.Chat.Home = b(config.KeybindingGroupChat, config.KeybindActionChatHome)
	km.Chat.End = b(config.KeybindingGroupChat, config.KeybindActionChatEnd)
	km.Chat.Copy = b(config.KeybindingGroupChat, config.KeybindActionChatCopy)
	km.Chat.ClearHighlight = b(config.KeybindingGroupChat, config.KeybindActionChatClearHighlight)
	km.Chat.Expand = b(config.KeybindingGroupChat, config.KeybindActionChatExpand)
	km.Chat.ScrollLeft = b(config.KeybindingGroupChat, config.KeybindActionChatScrollLeft)
	km.Chat.ScrollRight = b(config.KeybindingGroupChat, config.KeybindActionChatScrollRight)

	// Initialize
	km.Initialize.Yes = b(config.KeybindingGroupInitialize, config.KeybindActionInitializeYes)
	km.Initialize.No = b(config.KeybindingGroupInitialize, config.KeybindActionInitializeNo)
	km.Initialize.Switch = b(config.KeybindingGroupInitialize, config.KeybindActionInitializeSwitch)
	km.Initialize.Enter = b(config.KeybindingGroupInitialize, config.KeybindActionInitializeEnter)

	return km
}
