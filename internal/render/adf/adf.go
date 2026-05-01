// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

// Package adf builds Atlassian Document Format (ADF) documents for Confluence.
package adf

// Node types in the ADF spec.
const (
	TypeDoc         = "doc"
	TypeParagraph   = "paragraph"
	TypeHeading     = "heading"
	TypeText        = "text"
	TypeBulletList  = "bulletList"
	TypeOrderedList = "orderedList"
	TypeListItem    = "listItem"
	TypeTable       = "table"
	TypeTableRow    = "tableRow"
	TypeTableHeader = "tableHeader"
	TypeTableCell   = "tableCell"
	TypeCodeBlock   = "codeBlock"
	TypeBlockquote  = "blockquote"
	TypePanel       = "panel"
	TypeExpand      = "expand"
	TypeRule        = "rule"
	TypeTaskList    = "taskList"
	TypeTaskItem    = "taskItem"
	TypeExtension   = "extension"
	TypeInlineCard  = "inlineCard"
	TypeStatus      = "status"
	TypeHardBreak   = "hardBreak"
)

// Mark types in the ADF spec.
const (
	MarkStrong    = "strong"
	MarkEm        = "em"
	MarkCode      = "code"
	MarkLink      = "link"
	MarkTextColor = "textColor"
)

// Panel types supported by Confluence.
const (
	PanelInfo    = "info"
	PanelNote    = "note"
	PanelWarning = "warning"
	PanelSuccess = "success"
	PanelError   = "error"
)
