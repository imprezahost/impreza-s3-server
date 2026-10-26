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

package cmd

import (
	"reflect"
	"testing"
)

// CVE-2026-42600: the internode ReadMultiple storage-REST endpoint read files outside the configured drive
// roots. It had no caller, so the fix removes it everywhere a peer could reach it: the StorageAPI interface, the
// local drive, the disk-health wrapper and the REST client. Reflection keeps this test compiling on the unpatched
// base, where it fails because the method is still there.
func TestReadMultipleIsGone(t *testing.T) {
	for name, typ := range map[string]reflect.Type{
		"StorageAPI":           reflect.TypeOf((*StorageAPI)(nil)).Elem(),
		"xlStorage":            reflect.TypeOf((*xlStorage)(nil)),
		"xlStorageDiskIDCheck": reflect.TypeOf((*xlStorageDiskIDCheck)(nil)),
		"storageRESTClient":    reflect.TypeOf((*storageRESTClient)(nil)),
	} {
		if _, ok := typ.MethodByName("ReadMultiple"); ok {
			t.Errorf("%s still exposes ReadMultiple", name)
		}
	}
}
