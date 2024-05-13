package gousb

// ID represents a vendor or product ID.
type ID uint16

// Class represents a USB-IF (Implementers Forum) class or subclass code.
type Class uint8

// Protocol is the interface class protocol, qualified by the values
// of interface class and subclass.
type Protocol uint8

// DeviceDesc is a representation of a USB device descriptor.
type DeviceDesc struct {
	// Protocol information
	Class                Class    // The class of this device
	SubClass             Class    // The sub-class (within the class) of this device
	Protocol             Protocol // The protocol (within the sub-class) of this device
	MaxControlPacketSize int      // Maximum size of the control transfer

	// Product information
	Vendor  ID // The Vendor identifier
	Product ID // The Product identifier
}

// InterfaceSetting contains information about a USB interface with a particular
// alternate setting, extracted from the descriptor.
type InterfaceSetting struct {
	// Number is the number of this interface, the same as in InterfaceDesc.
	Number int
	// Alternate is the number of this alternate setting.
	Alternate int
	// Class is the USB-IF (Implementers Forum) class code, as defined by the USB spec.
	Class Class
	// SubClass is the USB-IF (Implementers Forum) subclass code, as defined by the USB spec.
	SubClass Class
	// Protocol is USB protocol code, as defined by the USB spe.c
	Protocol Protocol

	iInterface int // index of a string descriptor describing this interface.
}
