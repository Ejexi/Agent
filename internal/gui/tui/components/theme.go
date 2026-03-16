package components

// duckOpsBaseTheme is a customized version of the Glamour dark theme, modified to elegantly
// render markdown tables with cyan borders and clear spacing, matching advanced TUI standards.
const duckOpsBaseTheme = `{
  "document": { "block_prefix": "\n", "block_suffix": "\n", "color": "252", "margin": 2 },
  "block_quote": { "indent": 1, "indent_token": "│ " },
  "paragraph": {},
  "list": { "level_indent": 2 },
  "heading": { "block_suffix": "\n", "color": "39", "bold": true },
  "h1": { "prefix": " ", "suffix": " ", "color": "228", "background_color": "63", "bold": true },
  "h2": { "prefix": "## " },
  "h3": { "prefix": "### " },
  "h4": { "prefix": "#### " },
  "h5": { "prefix": "##### " },
  "h6": { "prefix": "###### ", "color": "35", "bold": false },
  "text": {},
  "strikethrough": { "crossed_out": true },
  "emph": { "italic": true },
  "strong": { "bold": true },
  "hr": { "color": "240", "format": "\n--------\n" },
  "item": { "block_prefix": "• " },
  "enumeration": { "block_prefix": ". " },
  "task": { "ticked": "[✓] ", "unticked": "[ ] " },
  "link": { "color": "30", "underline": true },
  "link_text": { "color": "35", "bold": true },
  "image": { "color": "212", "underline": true },
  "image_text": { "color": "243", "format": "Image: {{.text}} →" },
  "code": { "prefix": " ", "suffix": " ", "color": "203", "background_color": "236" },
  "code_block": {
    "color": "244",
    "margin": 2,
    "chroma": {
      "text": { "color": "#C4C4C4" },
      "error": { "color": "#F1F1F1", "background_color": "#F05B5B" }
    }
  },
  "table": {
    "center_separator": "┼",
    "column_separator": "│",
    "row_separator": "─"
  },
  "definition_list": {},
  "definition_term": {},
  "definition_description": { "block_prefix": "\n🠶 " },
  "html_block": {},
  "html_span": {}
}
`

// GetDuckOpsTheme configured glamour to elegantly format AI markdown responses.
func GetDuckOpsTheme() []byte {
	return []byte(duckOpsBaseTheme)
}
