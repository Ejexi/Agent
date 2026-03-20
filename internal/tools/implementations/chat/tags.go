package chat

import (
	"regexp"
	"strings"
)

// TagKind identifies the structured tag type emitted by the DuckOps LLM.
type TagKind string

const (
	// TagThinking — short reasoning update, streamed during analysis.
	// Maps to EventThought on the frontend (thinking block).
	TagThinking TagKind = "thinking"

	// TagApprovalRequired — the LLM wants to run a tool and needs user consent.
	// Maps to EventToolCall (pauses the loop until the user responds).
	TagApprovalRequired TagKind = "approval_required"

	// TagProgress — status update while a tool is running.
	// Maps to EventLog (shown inside the thinking trace as a progress row).
	TagProgress TagKind = "progress"

	// TagFinal — the user-visible answer. Maps to EventResult.
	TagFinal TagKind = "final"

	// TagPlain — raw text between tags (treated as thinking if non-empty).
	TagPlain TagKind = "plain"
)

// ParsedTag is one structured block extracted from an LLM response.
type ParsedTag struct {
	Kind TagKind

	// Content is the raw inner text of the tag.
	Content string

	// Approval fields — populated only when Kind == TagApprovalRequired.
	Tool   string
	Reason string
}

// reDuckOpsBlock matches any of the four DuckOps structured tags.
// Named group "tag" captures the tag name; "body" captures the inner text.
var reDuckOpsBlock = regexp.MustCompile(
	`(?is)<(thinking|approval_required|progress|final)>(.*?)</(?:thinking|approval_required|progress|final)>`,
)

// reToolLine extracts "tool: VALUE" from an approval_required body.
var reToolLine = regexp.MustCompile(`(?i)tool:\s*(.+)`)

// reReasonLine extracts "reason: VALUE" from an approval_required body.
var reReasonLine = regexp.MustCompile(`(?i)reason:\s*(.+)`)

// ParseDuckOpsTags splits raw LLM output into an ordered slice of ParsedTag.
//
// Plain text between tags is preserved as TagPlain blocks so that nothing is
// silently dropped — the caller decides whether to show or discard them.
//
// Example input:
//
//	<thinking>analyzing imports</thinking>
//	<approval_required>tool: semgrep\nreason: static analysis</approval_required>
//	<final>No critical issues found.</final>
func ParseDuckOpsTags(raw string) []ParsedTag {
	var tags []ParsedTag
	lastEnd := 0

	for _, m := range reDuckOpsBlock.FindAllStringSubmatchIndex(raw, -1) {
		// Capture plain text before this tag.
		if m[0] > lastEnd {
			plain := strings.TrimSpace(raw[lastEnd:m[0]])
			if plain != "" {
				tags = append(tags, ParsedTag{Kind: TagPlain, Content: plain})
			}
		}

		tagName := strings.ToLower(raw[m[2]:m[3]])
		body := strings.TrimSpace(raw[m[4]:m[5]])

		pt := ParsedTag{Kind: TagKind(tagName), Content: body}

		if pt.Kind == TagApprovalRequired {
			if mg := reToolLine.FindStringSubmatch(body); len(mg) > 1 {
				pt.Tool = strings.TrimSpace(mg[1])
			}
			if mg := reReasonLine.FindStringSubmatch(body); len(mg) > 1 {
				pt.Reason = strings.TrimSpace(mg[1])
			}
		}

		tags = append(tags, pt)
		lastEnd = m[1]
	}

	// Trailing plain text.
	if lastEnd < len(raw) {
		if tail := strings.TrimSpace(raw[lastEnd:]); tail != "" {
			tags = append(tags, ParsedTag{Kind: TagPlain, Content: tail})
		}
	}

	return tags
}

// IsFinal returns true if the slice contains at least one TagFinal block.
// Used by the agent loop to decide whether to break after processing tags.
func IsFinal(tags []ParsedTag) bool {
	for _, t := range tags {
		if t.Kind == TagFinal {
			return true
		}
	}
	return false
}

// HasApprovalRequest returns true if any tag requires tool approval.
func HasApprovalRequest(tags []ParsedTag) bool {
	for _, t := range tags {
		if t.Kind == TagApprovalRequired {
			return true
		}
	}
	return false
}
