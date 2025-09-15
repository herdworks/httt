package atom

// Package atom defines XML data structures for an Atom feed.
//
// reference: https://www.ietf.org/rfc/rfc4287.txt

// This file is a derivative work of "golang.org/x/tools/blog/atom" and is
// subject to the same copyright and license terms.
//
// golang.org/x/tools/LICENSE
// Copyright 2009 The Go Authors.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
//      notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
//      copyright notice, this list of conditions and the following disclaimer
//      in the documentation and/or other materials provided with the
//      distribution.
//    * Neither the name of Google LLC nor the names of its
//      contributors may be used to endorse or promote products derived from
//      this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
//
// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Adapted from encoding/xml/read_test.go.

import (
	"encoding/xml"
	"regexp"
	"time"
)

type Feed struct {
	XMLName xml.Name  `xml:"http://www.w3.org/2005/Atom feed"`
	XMLNS   string    `xml:"xmlns:hypergraph,attr"`
	Title   string    `xml:"title"`
	ID      string    `xml:"id"`
	Link    []Link    `xml:"link"`
	Updated TimeStamp `xml:"updated"`
	Author  *Person   `xml:"author,omitempty"`
	Entry   []*Entry  `xml:"entry"` // TODO: should this be renamed to "Entries"?
}

func (feed *Feed) Render() (string, error) {
	atomBytes, err := xml.MarshalIndent(feed, "", "\t")
	if err != nil {
		return "", err
	}
	return string(atomBytes), nil
}

type Entry struct {
	Title       string    `xml:"title"`
	ID          string    `xml:"id"`
	Link        []Link    `xml:"link"`
	Published   TimeStamp `xml:"published"`
	Updated     TimeStamp `xml:"updated"`
	Author      *Person   `xml:"author"`
	Contributor []*Person `xml:"contributor"` // extension from golang.org/x/tools/blog/atom
	Summary     *Text     `xml:"summary"`
	Content     *Text     `xml:"content"`
}

type Link struct {
	Rel      string `xml:"rel,attr,omitempty"`
	Href     string `xml:"href,attr"`
	Type     string `xml:"type,attr,omitempty"`
	HrefLang string `xml:"hreflang,attr,omitempty"`
	Title    string `xml:"title,attr,omitempty"`
	Length   uint   `xml:"length,attr,omitempty"`
}

type Person struct {
	Name     string `xml:"name"`
	URI      string `xml:"uri,omitempty"`
	Username string `xml:"hypergraph:username,omitempty"`
	Favicon  string `xml:"hypergraph:favicon,omitempty"`
	Email    string `xml:"email,omitempty"`
	InnerXML string `xml:",innerxml"`
}

type Text struct {
	Type string `xml:"type,attr"`
	Body string `xml:",chardata"`
}

type TimeStamp struct {
	Time     time.Time
	RawValue string
}

func (t *TimeStamp) String() string {
	if !t.Time.IsZero() {
		return t.Time.Format(time.RFC3339)
	}
	ts, err := time.Parse(time.RFC3339, t.RawValue)
	if err != nil {
		return t.RawValue
	}
	return ts.Format(time.RFC3339)
}
func (t *TimeStamp) MarshalXML(encoder *xml.Encoder, start xml.StartElement) error {
	return encoder.EncodeElement(t.String(), start)
}

// AtomDetected demonstrates regexp on []byte.
func AtomDetected(doc []byte) bool {
	var patterns []*regexp.Regexp
	patterns = append(patterns, regexp.MustCompile(`<(feed|entry)\b[^>]*\bxmlns\s*=\s*('http://www\.w3\.org/2005/Atom'|"http://www\.w3\.org/2005/Atom")`))
	for _, pattern := range patterns {
		if pattern.Match(doc) {
			return true
		}
	}
	return false
}
