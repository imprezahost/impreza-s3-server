// Copyright (c) 2015-2026 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package csv

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio/internal/s3select/sql"
)

// CVE-2026-39414: a CSV record longer than the S3 Select limit (1 MiB) must be refused with
// OverMaxRecordSize instead of being buffered without bound. The literal 1<<20 keeps this test compiling on the
// unpatched base, where it fails on behaviour (the oversized record is read as an ordinary row).
const selectRecordLimit = 1 << 20

func readAllCSVRecords(t *testing.T, content string) error {
	t.Helper()
	r, err := NewReader(io.NopCloser(strings.NewReader(content)), &ReaderArgs{
		FileHeaderInfo:             none,
		RecordDelimiter:            defaultRecordDelimiter,
		FieldDelimiter:             defaultFieldDelimiter,
		QuoteCharacter:             defaultQuoteCharacter,
		QuoteEscapeCharacter:       defaultQuoteEscapeCharacter,
		CommentCharacter:           defaultCommentCharacter,
		AllowQuotedRecordDelimiter: false,
		unmarshaled:                true,
	})
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()
	var record sql.Record
	for {
		record, err = r.Read(record)
		if err != nil {
			return err
		}
	}
}

func TestCSVRejectsOversizedRecord(t *testing.T) {
	big := strings.Repeat("a", selectRecordLimit+1)
	err := readAllCSVRecords(t, "1,2,3\n"+big+"\nx,y,z\n")
	var se interface{ ErrorCode() string }
	if err == nil || errors.Is(err, io.EOF) || !errors.As(err, &se) || se.ErrorCode() != "OverMaxRecordSize" {
		t.Fatalf("an oversized CSV record was not refused with OverMaxRecordSize: %v", err)
	}
}

func TestCSVAcceptsARecordAtTheLimit(t *testing.T) {
	atLimit := strings.Repeat("a", selectRecordLimit)
	if err := readAllCSVRecords(t, "1,2,3\n"+atLimit+"\nx,y,z\n"); !errors.Is(err, io.EOF) {
		t.Fatalf("a record of exactly 1 MiB was refused: %v", err)
	}
}
