package imapclient_test

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

// Body structures servers really send that break RFC 3501's grammar. One
// such message must not make a whole FETCH fail: its structure is read as
// far as it goes.
var lenientStructures = map[string]string{
	// A multipart with no parts (emersion/go-imap#701).
	"empty multipart": `(("ALTERNATIVE" ("BOUNDARY" "e7e3") NIL NIL)("APPLICATION" "PDF" NIL NIL NIL "BASE64" 1476806 NIL ("ATTACHMENT" ("FILENAME" "a.pdf")) NIL) "MIXED" ("BOUNDARY" "7b1f") NIL NIL)`,
	// message/global without the envelope, body and lines of a message/rfc822
	// (emersion/go-imap#678).
	"message/global without envelope": `(("text" "plain" ("charset" "UTF-8") NIL NIL "7bit" 5 1 NIL NIL NIL NIL)("message" "global" ("name" "x.dat") NIL NIL "7bit" 6726 NIL ("attachment" ("filename" "x.dat")) NIL NIL) "mixed" ("boundary" "b") NIL NIL NIL)`,
	// message/rfc822 whose line count is missing.
	"message/rfc822 without lines": `(("text" "plain" NIL NIL NIL "7bit" 5 1)("message" "rfc822" NIL NIL NIL "7bit" 300 (NIL "s" NIL NIL NIL NIL NIL NIL NIL NIL) ("text" "plain" NIL NIL NIL "7bit" 5 1)) "mixed")`,
}

func TestLenientBodyStructure(t *testing.T) {
	for name, bs := range lenientStructures {
		t.Run(name, func(t *testing.T) {
			c, _ := gmailServer(t, map[string][]string{
				"UID FETCH 7 (UID BODYSTRUCTURE)": {"* 1 FETCH (UID 7 BODYSTRUCTURE " + bs + ")"},
			})
			msgs, err := c.Fetch(imap.UIDSetNum(7), &imap.FetchOptions{UID: true, BodyStructure: &imap.FetchItemBodyStructure{Extended: true}}).Collect()
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if len(msgs) != 1 || msgs[0].UID != 7 {
				t.Fatalf("messages: %+v", msgs)
			}
			mp, ok := msgs[0].BodyStructure.(*imap.BodyStructureMultiPart)
			if !ok || len(mp.Children) != 2 {
				t.Fatalf("the rest of the structure must survive: %#v", msgs[0].BodyStructure)
			}
		})
	}
}

func fetchStructure(t *testing.T, bs string) imap.BodyStructure {
	t.Helper()
	c, _ := gmailServer(t, map[string][]string{
		"UID FETCH 7 (UID BODYSTRUCTURE)": {"* 1 FETCH (UID 7 BODYSTRUCTURE " + bs + ")"},
	})
	msgs, err := c.Fetch(imap.UIDSetNum(7), &imap.FetchOptions{UID: true, BodyStructure: &imap.FetchItemBodyStructure{Extended: true}}).Collect()
	if err != nil || len(msgs) != 1 {
		t.Fatalf("fetch: %v %+v", err, msgs)
	}
	return msgs[0].BodyStructure
}

// What is present is still read in full: a well-formed attached message,
// and one whose line count alone is missing.
func TestBodyStructureMessagePart(t *testing.T) {
	for name, tc := range map[string]struct {
		bs    string
		lines int64
		ext   bool
	}{
		"complete":      {`(("text" "plain" NIL NIL NIL "7bit" 5 1)("message" "rfc822" NIL NIL NIL "7bit" 300 (NIL "s" NIL NIL NIL NIL NIL NIL NIL NIL) ("text" "plain" NIL NIL NIL "7bit" 5 1) 12 NIL ("attachment" ("filename" "fwd.eml")) NIL NIL) "mixed")`, 12, true},
		"without lines": {`(("text" "plain" NIL NIL NIL "7bit" 5 1)("message" "rfc822" NIL NIL NIL "7bit" 300 (NIL "s" NIL NIL NIL NIL NIL NIL NIL NIL) ("text" "plain" NIL NIL NIL "7bit" 5 1) NIL ("attachment" ("filename" "fwd.eml")) NIL NIL) "mixed")`, 0, true},
	} {
		t.Run(name, func(t *testing.T) {
			mp, ok := fetchStructure(t, tc.bs).(*imap.BodyStructureMultiPart)
			if !ok || len(mp.Children) != 2 {
				t.Fatalf("structure: %#v", mp)
			}
			part := mp.Children[1].(*imap.BodyStructureSinglePart)
			m := part.MessageRFC822
			if m == nil || m.Envelope == nil || m.Envelope.Subject != "s" || m.BodyStructure == nil || m.NumLines != tc.lines {
				t.Fatalf("attached message: %#v", m)
			}
			if tc.ext != (part.Filename() == "fwd.eml") {
				t.Fatalf("extension data: %#v", part.Extended)
			}
		})
	}
}
