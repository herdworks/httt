package opml

import (
	"net/url"
	"os"
	"testing"
	"time"
)

// use the netnewswire default feeds file for test data
// source: https://github.com/Ranchero-Software/NetNewsWire/blob/main/Shared/Importers/DefaultFeeds.opml
var NNWDefaultOPML_sampledata string = `
<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
<head>
	<title>Default Feeds</title>
</head>
<body>
	<outline text="Allen Pike" title="Allen Pike" type="rss" version="RSS" htmlUrl="https://www.allenpike.com/" xmlUrl="https://feeds.allenpike.com/feed/"/>
	<outline text="Colossal" title="Colossal" type="rss" version="RSS" htmlUrl="https://www.thisiscolossal.com/" xmlUrl="https://www.thisiscolossal.com/feed/"/>
	<outline text="Craig Hockenberry" title="Craig Hockenberry" type="rss" version="RSS" htmlUrl="https://furbo.org/" xmlUrl="https://furbo.org/feed/json"/>
	<outline text="Daring Fireball" title="Daring Fireball" type="rss" version="RSS" htmlUrl="https://daringfireball.net/" xmlUrl="https://daringfireball.net/feeds/json"/>
	<outline text="inessential" title="inessential" type="rss" version="RSS" htmlUrl="https://inessential.com/" xmlUrl="https://inessential.com/feed.json"/>
	<outline text="Jason Kottke" title="Jason Kottke" type="rss" version="RSS" htmlUrl="https://kottke.org/" xmlUrl="https://feeds.kottke.org/json"/>
	<outline text="Julia Evans" title="Julia Evans" type="rss" version="RSS" htmlUrl="https://jvns.ca/" xmlUrl="https://jvns.ca/atom.xml"/>
	<outline text="Manton Reece" title="Manton Reece" type="rss" version="RSS" htmlUrl="https://manton.org/" xmlUrl="https://www.manton.org/feed/json"/>
	<outline text="Maurice Parker" title="Maurice Parker" type="rss" version="RSS" htmlUrl="https://vincode.io/" xmlUrl="https://vincode.io/feed.xml"/>
	<outline text="Michael Tsai" title="Michael Tsai" type="rss" version="RSS" htmlUrl="https://mjtsai.com/blog/" xmlUrl="https://mjtsai.com/blog/feed/"/>
	<outline text="NetNewsWire Blog" title="NetNewsWire Blog" type="rss" version="RSS" htmlUrl="https://netnewswire.blog/" xmlUrl="https://netnewswire.blog/feed.json"/>
	<outline text="One Foot Tsunami" title="One Foot Tsunami" type="rss" version="RSS" htmlUrl="https://onefoottsunami.com/" xmlUrl="https://onefoottsunami.com/feed/json/"/>
	<outline text="Scripting News" title="Scripting News" type="rss" version="RSS" htmlUrl="http://scripting.com/" xmlUrl="http://scripting.com/rss.xml"/>
	<outline text="Six Colors" title="Six Colors" type="rss" version="RSS" htmlUrl="https://sixcolors.com/" xmlUrl="https://feedpress.me/sixcolors?type=xml"/>
</body>
</opml>
`

// TestOPML_Load
//
// go test ./...
//
// go test ./... -run TestOPML_Load
func TestOPML_Load(t *testing.T) {
	var sampleDataFile *os.File
	var sampleDataFileURL *url.URL
	var err error

	// open OPML sample data file, write sample data, get file URL
	if sampleDataFile, err = os.CreateTemp("/tmp/", "default.opml"); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(sampleDataFile.Name())
	if _, err := sampleDataFile.Write([]byte(NNWDefaultOPML_sampledata)); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if sampleDataFileURL, err = url.Parse(sampleDataFile.Name()); err != nil {
		t.Fatalf("Failed to parse sampleDataFile URL: %v", err)
	}

	opml := &OPML{URI: sampleDataFileURL}
	if err = opml.Load(); err != nil {
		t.Fatalf("Failed to load OPML: %v", err)
	}
	for _, outline := range opml.Body.Outlines {
		t.Logf("Outline: %s (%s)\n", outline.Title, outline.XMLURL)
	}
	if len(opml.Body.Outlines) != 14 {
		t.Errorf("Expected 16 outlines, got %d", len(opml.Body.Outlines))
	}

}

func TestOPML_DateCreated(t *testing.T) {
    t.Parallel()

    cases := []struct {
        name    string
        layout  string
        dateStr string
    }{
        {
            name:    "RFC822 with zone name",
            layout:  "Mon, 02 Jan 06 15:04 MST",
            dateStr: "Mon, 02 Jan 06 15:04 MST",
        },
        {
            name:    "RFC822 without weekday, zone name",
            layout:  "02 Jan 06 15:04 MST",
            dateStr: "02 Jan 06 15:04 MST",
        },
        {
            name:    "RFC822Z with numeric offset",
            layout:  "Mon, 02 Jan 06 15:04 -0700",
            dateStr: "Mon, 02 Jan 06 15:04 -0700",
        },
        {
            name:    "RFC822Z without weekday, numeric offset",
            layout:  "02 Jan 06 15:04 -0700",
            dateStr: "02 Jan 06 15:04 -0700",
        },
    }

    for _, tc := range cases {
        tc := tc
        t.Run(tc.name, func(t *testing.T) {
			var want time.Time
			var got time.Time
			var err error
            t.Parallel()

            if want, err = time.Parse(tc.layout, tc.dateStr); err != nil {
                t.Fatalf("test setup parse failed for %q: %v", tc.dateStr, err)
			}

            op := &OPML{Head: Head{DateCreated: tc.dateStr, DateModified: tc.dateStr}}

			// DateCreated test
			if got, err = op.DateCreated(); err != nil {
                t.Fatalf("DateCreated() error = %v, want nil", err)
            }
            if !got.Equal(want) {
                t.Fatalf("DateCreated() = %v, want %v", got, want)
            }

			// DateModified test
			if got, err = op.DateModified(); err != nil {
				t.Fatalf("DateModified() error = %v, want nil", err)
			}
			if !got.Equal(want) {
				t.Fatalf("DateModified() = %v, want %v", got, want)
			}
		})
	}
}
