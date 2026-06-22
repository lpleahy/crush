package styles

import (
	"charm.land/glamour/v2/ansi"
	"charm.land/lipgloss/v2"
)

func Transparent(s Styles) Styles {
	s.Background = nil
	transparentMarkdown(&s.Markdown)
	transparentMarkdown(&s.QuietMarkdown)

	s.Diff.DividerLine = transparentLineStyle(s.Diff.DividerLine)
	s.Diff.MissingLine = transparentLineStyle(s.Diff.MissingLine)
	s.Diff.EqualLine = transparentLineStyle(s.Diff.EqualLine)
	s.Diff.Filename = transparentLineStyle(s.Diff.Filename)

	s.Tool.ContentLine = s.Tool.ContentLine.UnsetBackground()
	s.Tool.ContentTruncation = s.Tool.ContentTruncation.UnsetBackground()
	s.Tool.ContentCodeLine = s.Tool.ContentCodeLine.UnsetBackground()
	s.Tool.ContentCodeTruncation = s.Tool.ContentCodeTruncation.UnsetBackground()
	s.Tool.ContentCodeBg = nil
	s.Tool.ContentBg = s.Tool.ContentBg.UnsetBackground()
	s.Tool.ContentLineNumber = s.Tool.ContentLineNumber.UnsetBackground()
	s.Tool.DiffTruncation = s.Tool.DiffTruncation.UnsetBackground()

	s.Messages.ThinkingBox = s.Messages.ThinkingBox.UnsetBackground()
	transparentDialogBackgrounds(&s)

	s.Completions.Normal = s.Completions.Normal.UnsetBackground()
	s.Attachments.Normal = s.Attachments.Normal.UnsetBackground()

	return s
}

func transparentLineStyle(s diffLineStyle) diffLineStyle {
	s.LineNumber = s.LineNumber.UnsetBackground()
	s.Symbol = s.Symbol.UnsetBackground()
	s.Code = s.Code.UnsetBackground()
	return s
}

type diffLineStyle = struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

func transparentDialogBackgrounds(s *Styles) {
	s.Dialog.ContentPanel = s.Dialog.ContentPanel.UnsetBackground()
	s.Dialog.Permissions.ParamsBg = nil
	s.Dialog.OAuth.UserCodeBg = nil
}

func transparentMarkdown(c *ansi.StyleConfig) {
	transparentBlock(&c.Document)
	transparentBlock(&c.BlockQuote)
	transparentBlock(&c.Heading)
	transparentBlock(&c.H1)
	transparentBlock(&c.H2)
	transparentBlock(&c.H3)
	transparentBlock(&c.H4)
	transparentBlock(&c.H5)
	transparentBlock(&c.H6)
	transparentPrimitive(&c.Strikethrough)
	transparentPrimitive(&c.Emph)
	transparentPrimitive(&c.Strong)
	transparentPrimitive(&c.HorizontalRule)
	transparentPrimitive(&c.Item)
	transparentPrimitive(&c.Enumeration)
	transparentPrimitive(&c.Task.StylePrimitive)
	transparentPrimitive(&c.Link)
	transparentPrimitive(&c.LinkText)
	transparentPrimitive(&c.Image)
	transparentPrimitive(&c.ImageText)
	transparentBlock(&c.Code)
	transparentBlock(&c.CodeBlock.StyleBlock)
	transparentTable(&c.Table)
	transparentPrimitive(&c.DefinitionDescription)
	if c.CodeBlock.Chroma != nil {
		transparentChroma(c.CodeBlock.Chroma)
	}
}

func transparentTable(t *ansi.StyleTable) {
	transparentBlock(&t.StyleBlock)
}

func transparentBlock(b *ansi.StyleBlock) {
	transparentPrimitive(&b.StylePrimitive)
}

func transparentPrimitive(p *ansi.StylePrimitive) {
	p.BackgroundColor = nil
}

func transparentChroma(c *ansi.Chroma) {
	transparentPrimitive(&c.Text)
	transparentPrimitive(&c.Error)
	transparentPrimitive(&c.Comment)
	transparentPrimitive(&c.CommentPreproc)
	transparentPrimitive(&c.Keyword)
	transparentPrimitive(&c.KeywordReserved)
	transparentPrimitive(&c.KeywordNamespace)
	transparentPrimitive(&c.KeywordType)
	transparentPrimitive(&c.Operator)
	transparentPrimitive(&c.Punctuation)
	transparentPrimitive(&c.Name)
	transparentPrimitive(&c.NameBuiltin)
	transparentPrimitive(&c.NameTag)
	transparentPrimitive(&c.NameAttribute)
	transparentPrimitive(&c.NameClass)
	transparentPrimitive(&c.NameConstant)
	transparentPrimitive(&c.NameDecorator)
	transparentPrimitive(&c.NameException)
	transparentPrimitive(&c.NameFunction)
	transparentPrimitive(&c.NameOther)
	transparentPrimitive(&c.Literal)
	transparentPrimitive(&c.LiteralNumber)
	transparentPrimitive(&c.LiteralDate)
	transparentPrimitive(&c.LiteralString)
	transparentPrimitive(&c.LiteralStringEscape)
	transparentPrimitive(&c.GenericDeleted)
	transparentPrimitive(&c.GenericEmph)
	transparentPrimitive(&c.GenericInserted)
	transparentPrimitive(&c.GenericStrong)
	transparentPrimitive(&c.GenericSubheading)
	transparentPrimitive(&c.Background)
}
