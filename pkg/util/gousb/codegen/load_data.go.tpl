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

import "time"

// LastUpdate stores the latest time that the library was updated.
//
// The baked-in data was last generated:
//   {{.Generated}}
var LastUpdate = time.Unix(0, {{.Generated.UnixNano}})

const usbIDListData = `{{printf "%s" .Data}}`