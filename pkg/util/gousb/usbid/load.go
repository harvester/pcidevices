/* This file was part of the google/gousb project, copied to this project
 * to get around private package issues.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2024 SUSE, LLC.
 *
 */

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

func init() {
	ids, cls, err := NewParser().ParseIDs(strings.NewReader(usbIDListData))
	if err != nil {
		logrus.Errorf("Failed to parse USB ID list: %v", err)
		return
	}

	Vendors = ids
	Classes = cls
}
