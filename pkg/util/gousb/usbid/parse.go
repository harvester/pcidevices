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
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/harvester/pcidevices/pkg/util/gousb"
)

// A Vendor contains the name of the vendor and mappings corresponding to all
// known products by their ID.
type Vendor struct {
	Name    string
	Product map[gousb.ID]*Product
}

// String returns the name of the vendor.
func (v Vendor) String() string {
	return v.Name
}

// A Product contains the name of the product (from a particular vendor) and
// the names of any interfaces that were specified.
type Product struct {
	Name      string
	Interface map[gousb.ID]string
}

// String returns the name of the product.
func (p Product) String() string {
	return p.Name
}

// A Class contains the name of the class and mappings for each subclass.
type Class struct {
	Name     string
	SubClass map[gousb.Class]*SubClass
}

// String returns the name of the class.
func (c Class) String() string {
	return c.Name
}

// A SubClass contains the name of the subclass and any associated protocols.
type SubClass struct {
	Name     string
	Protocol map[gousb.Protocol]string
}

// String returns the name of the SubClass.
func (s SubClass) String() string {
	return s.Name
}

type Parser struct {
	vendor   *Vendor
	device   *Product
	class    *Class
	subclass *SubClass

	vendors map[gousb.ID]*Vendor
	classes map[gousb.Class]*Class
}

func NewParser() *Parser {
	return &Parser{
		vendor:   nil,
		device:   nil,
		class:    nil,
		subclass: nil,

		vendors: make(map[gousb.ID]*Vendor, 2800),
		classes: make(map[gousb.Class]*Class), // TODO(kevlar): count
	}
}

func (p *Parser) split(s string) (kind string, level int, id uint64, name string, err error) {
	pieces := strings.SplitN(s, "  ", 2)
	if len(pieces) != 2 {
		err = fmt.Errorf("malformatted line %q", s)
		return
	}

	// Save the name
	name = pieces[1]

	// Parse out the level
	for len(pieces[0]) > 0 && pieces[0][0] == '\t' {
		level, pieces[0] = level+1, pieces[0][1:]
	}

	// Parse the first piece to see if it has a kind
	first := strings.SplitN(pieces[0], " ", 2)
	if len(first) == 2 {
		kind, pieces[0] = first[0], first[1]
	}

	// Parse the ID
	i, err := strconv.ParseUint(pieces[0], 16, 16)
	if err != nil {
		err = fmt.Errorf("malformatted id %q: %s", pieces[0], err)
		return
	}
	id = i

	return
}

func (p *Parser) parseVendor(level int, raw uint64, name string) error {
	id := gousb.ID(raw)

	switch level {
	case 0:
		p.vendor = &Vendor{
			Name: name,
		}
		p.vendors[id] = p.vendor

	case 1:
		if p.vendor == nil {
			return fmt.Errorf("product line without vendor line")
		}

		p.device = &Product{
			Name: name,
		}
		if p.vendor.Product == nil {
			p.vendor.Product = make(map[gousb.ID]*Product)
		}
		p.vendor.Product[id] = p.device

	case 2:
		if p.device == nil {
			return fmt.Errorf("interface line without device line")
		}

		if p.device.Interface == nil {
			p.device.Interface = make(map[gousb.ID]string)
		}
		p.device.Interface[id] = name

	default:
		return fmt.Errorf("too many levels of nesting for vendor block")
	}

	return nil
}

func (p *Parser) parseClass(level int, id uint64, name string) error {
	switch level {
	case 0:
		p.class = &Class{
			Name: name,
		}
		p.classes[gousb.Class(id)] = p.class

	case 1:
		if p.class == nil {
			return fmt.Errorf("subclass line without class line")
		}

		p.subclass = &SubClass{
			Name: name,
		}
		if p.class.SubClass == nil {
			p.class.SubClass = make(map[gousb.Class]*SubClass)
		}
		p.class.SubClass[gousb.Class(id)] = p.subclass

	case 2:
		if p.subclass == nil {
			return fmt.Errorf("protocol line without subclass line")
		}

		if p.subclass.Protocol == nil {
			p.subclass.Protocol = make(map[gousb.Protocol]string)
		}
		p.subclass.Protocol[gousb.Protocol(id)] = name

	default:
		return fmt.Errorf("too many levels of nesting for class")
	}

	return nil
}

// ParseIDs parses and returns mappings from the given reader.  In general, this
// should not be necessary, as a set of mappings is already embedded in the library.
// If a new or specialized file is obtained, this can be used to retrieve the mappings,
// which can be stored in the global Vendors and Classes map.
func (p *Parser) ParseIDs(r io.Reader) (map[gousb.ID]*Vendor, map[gousb.Class]*Class, error) {
	var kind string

	lines := bufio.NewReaderSize(r, 512)
parseLines:
	for lineno := 0; ; lineno++ {
		b, isPrefix, err := lines.ReadLine()
		switch {
		case err == io.EOF:
			break parseLines
		case err != nil:
			return nil, nil, err
		case isPrefix:
			return nil, nil, fmt.Errorf("line %d: line too long", lineno)
		}
		line := string(b)

		if len(line) == 0 || line[0] == '#' {
			continue
		}

		k, level, id, name, err := p.split(line)
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %s", lineno, err)
		}
		if k != "" {
			kind = k
		}

		switch kind {
		case "":
			err = p.parseVendor(level, id, name)
		case "C":
			err = p.parseClass(level, id, name)
		}
		if err != nil {
			return nil, nil, fmt.Errorf("line %d: %s", lineno, err)
		}
	}

	return p.vendors, p.classes, nil
}
