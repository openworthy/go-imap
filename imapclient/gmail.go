package imapclient

import (
	"fmt"
	"strings"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/internal/imapwire"
)

// StoreGmailLabels sends a STORE command altering Gmail labels
// (X-GM-LABELS; requires X-GM-EXT-1).
//
// Unless StoreGmailLabels.Silent is set, the server returns the updated
// labels, which a FetchCommand reads as FetchItemDataGmailLabels.
func (c *Client) StoreGmailLabels(numSet imap.NumSet, store *imap.StoreGmailLabels) *FetchCommand {
	cmd := &FetchCommand{
		numSet: numSet,
		msgs:   make(chan *FetchMessageData, 128),
	}
	enc := c.beginCommand(uidCmdName("STORE", imapwire.NumSetKind(numSet)), cmd)
	enc.SP().NumSet(numSet).SP()
	switch store.Op {
	case imap.StoreFlagsSet:
		// nothing to do
	case imap.StoreFlagsAdd:
		enc.Special('+')
	case imap.StoreFlagsDel:
		enc.Special('-')
	default:
		panic(fmt.Errorf("imapclient: unknown store flags op: %v", store.Op))
	}
	enc.Atom("X-GM-LABELS")
	if store.Silent {
		enc.Atom(".SILENT")
	}
	enc.SP().List(len(store.Labels), func(i int) {
		writeGmailLabel(enc.Encoder, store.Labels[i])
	})
	enc.end()
	return cmd
}

// writeGmailLabel writes a system label as a flag-like atom and a user
// label like a mailbox name (modified UTF-7), as Gmail expects.
func writeGmailLabel(enc *imapwire.Encoder, label string) {
	if strings.HasPrefix(label, `\`) {
		enc.Flag(imap.Flag(label))
	} else {
		enc.Mailbox(label)
	}
}

// FetchItemDataGmailLabels holds data returned by FETCH X-GM-LABELS.
type FetchItemDataGmailLabels struct {
	Labels []string
}

func (FetchItemDataGmailLabels) fetchItemData() {}

// readGmailLabels reads a label list: system labels are flag-like atoms
// ("\Inbox"), user labels are astrings in modified UTF-7.
func readGmailLabels(dec *imapwire.Decoder) ([]string, error) {
	labels := []string{}
	err := dec.ExpectList(func() error {
		dec.SP() // tolerate a stray space, as flag lists do
		if dec.Special('\\') {
			var name string
			if !dec.ExpectAtom(&name) {
				return fmt.Errorf("in gmail label: %w", dec.Err())
			}
			labels = append(labels, `\`+name)
			return nil
		}
		var name string
		if !dec.ExpectMailbox(&name) {
			return fmt.Errorf("in gmail label: %w", dec.Err())
		}
		labels = append(labels, name)
		return nil
	})
	return labels, err
}
