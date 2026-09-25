package imap

// CapGmailExt1 indicates support for Gmail's IMAP extensions (X-GM-EXT-1):
// X-GM-LABELS, X-GM-MSGID, X-GM-THRID and X-GM-RAW. See
// https://developers.google.com/workspace/gmail/imap/imap-extensions
const CapGmailExt1 Cap = "X-GM-EXT-1"

// StoreGmailLabels alters the Gmail labels of messages (requires
// X-GM-EXT-1).
//
// A label is either a user label's name, in UTF-8 ("Receipts"), or a system
// label, which starts with a backslash ("\\Important", "\\Starred").
type StoreGmailLabels struct {
	Op     StoreFlagsOp
	Silent bool
	Labels []string
}
