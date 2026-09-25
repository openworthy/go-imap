package imapclient_test

import (
	"bufio"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

// gmailServer answers each command from script, keyed by the command's text
// without its tag, as Gmail's IMAP server would, and records what it got.
func gmailServer(t *testing.T, script map[string][]string) (*imapclient.Client, *[]string) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	var got []string
	go func() {
		defer serverConn.Close()
		w := bufio.NewWriter(serverConn)
		w.WriteString("* OK [CAPABILITY IMAP4rev1 X-GM-EXT-1] Gimap ready\r\n")
		w.Flush()
		r := bufio.NewReader(serverConn)
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			line = strings.TrimRight(line, "\r\n")
			tag, cmd, _ := strings.Cut(line, " ")
			got = append(got, cmd)
			for _, untagged := range script[cmd] {
				w.WriteString(untagged + "\r\n")
			}
			w.WriteString(tag + " OK Success\r\n")
			w.Flush()
		}
	}()
	c := imapclient.New(clientConn, nil)
	t.Cleanup(func() { c.Close() })
	return c, &got
}

func TestStoreGmailLabels(t *testing.T) {
	c, got := gmailServer(t, nil)
	if !c.Caps().Has(imap.CapGmailExt1) {
		t.Fatalf("X-GM-EXT-1 not advertised")
	}
	uids := imap.UIDSetNum(42, 43)
	for _, store := range []*imap.StoreGmailLabels{
		{Op: imap.StoreFlagsAdd, Silent: true, Labels: []string{"Worth Reading", `\Starred`, "Café"}},
		{Op: imap.StoreFlagsDel, Silent: true, Labels: []string{"Worth Reading"}},
		{Op: imap.StoreFlagsSet, Labels: []string{"Receipts"}},
	} {
		if err := c.StoreGmailLabels(uids, store).Close(); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		`UID STORE 42:43 +X-GM-LABELS.SILENT ("Worth Reading" \Starred "Caf&AOk-")`,
		`UID STORE 42:43 -X-GM-LABELS.SILENT ("Worth Reading")`,
		`UID STORE 42:43 X-GM-LABELS ("Receipts")`,
	}
	if !reflect.DeepEqual(*got, want) {
		t.Fatalf("sent:\n%s\nwant:\n%s", strings.Join(*got, "\n"), strings.Join(want, "\n"))
	}
}

func TestFetchGmailLabels(t *testing.T) {
	c, got := gmailServer(t, map[string][]string{
		"UID FETCH 42:44 (UID X-GM-LABELS)": {
			`* 1 FETCH (X-GM-LABELS (\Inbox \Important "Worth Reading" Caf&AOk-) UID 42)`,
			`* 2 FETCH (X-GM-LABELS () UID 43)`,
			`* 3 FETCH (UID 44 X-GM-LABELS ("a \"quoted\" label"))`,
		},
	})
	msgs, err := c.Fetch(imap.UIDSetNum(42, 43, 44), &imap.FetchOptions{UID: true, GmailLabels: true}).Collect()
	if err != nil {
		t.Fatal(err)
	}
	if len(*got) != 1 || !strings.HasPrefix((*got)[0], "UID FETCH 42:44 (UID X-GM-LABELS)") {
		t.Fatalf("sent %q", *got)
	}
	want := map[imap.UID][]string{
		42: {`\Inbox`, `\Important`, "Worth Reading", "Café"},
		43: {},
		44: {`a "quoted" label`},
	}
	if len(msgs) != 3 {
		t.Fatalf("%d messages", len(msgs))
	}
	for _, m := range msgs {
		if !reflect.DeepEqual(m.GmailLabels, want[m.UID]) {
			t.Errorf("UID %d labels = %q, want %q", m.UID, m.GmailLabels, want[m.UID])
		}
	}
}
