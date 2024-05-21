// Copy from https://github.com/google/gousb
//
// Copyright 2013 Google Inc.  All rights reserved.
// Copyright 2016 the gousb Authors.  All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package usbid

import (
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/harvester/pcidevices/pkg/util/gousb"
)

const (
	// LinuxUsbDotOrg is one source of files in the format used by this package.
	LinuxUsbDotOrg = "http://www.linux-usb.org/usb.ids"
)

var (
	// Vendors stores the vendor and product ID mappings.
	Vendors map[gousb.ID]*Vendor

	// Classes stores the class, subclass and protocol mappings.
	Classes map[gousb.Class]*Class
)

func Setup() {
	ids, cls, err := ParseIDs(strings.NewReader(usbIDListData))
	if err != nil {
		logrus.Errorf("Failed to parse USB ID list: %v", err)
		return
	}

	Vendors = ids
	Classes = cls
}