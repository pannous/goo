package units

import "fmt"

// Unit represents a physical unit with value and metadata
type Unit struct {
	name             string
	symbol           string
	baseUnit         string
	conversionFactor float64
	category         string
	value            float64
}

// NewUnit creates a new unit instance
func NewUnit(name, symbol, baseUnit string, conversionFactor float64, category string) *Unit {
	return &Unit{
		name:             name,
		symbol:           symbol,
		baseUnit:         baseUnit,
		conversionFactor: conversionFactor,
		category:         category,
		value:            1.0,
	}
}

// withValue creates a copy of the unit with a specific value
func (u *Unit) withValue(val float64) *Unit {
	return &Unit{
		name:             u.name,
		symbol:           u.symbol,
		baseUnit:         u.baseUnit,
		conversionFactor: u.conversionFactor,
		category:         u.category,
		value:            val,
	}
}

// Arithmetic operations (lowercase for internal use)
func (u *Unit) add(other *Unit) *Unit {
	if u.category != other.category {
		panic("Cannot add incompatible units: " + u.category + " + " + other.category)
	}
	
	// Convert both to base units
	thisBase := u.value * u.conversionFactor
	otherBase := other.value * other.conversionFactor
	resultBase := thisBase + otherBase
	
	// Return result in this unit's type
	return u.withValue(resultBase / u.conversionFactor)
}

func (u *Unit) sub(other *Unit) *Unit {
	if u.category != other.category {
		panic("Cannot subtract incompatible units: " + u.category + " - " + other.category)
	}
	
	thisBase := u.value * u.conversionFactor
	otherBase := other.value * other.conversionFactor
	resultBase := thisBase - otherBase
	
	return u.withValue(resultBase / u.conversionFactor)
}

func (u *Unit) mul(scalar float64) *Unit {
	return u.withValue(u.value * scalar)
}

func (u *Unit) div(scalar float64) *Unit {
	return u.withValue(u.value / scalar)
}

// Exported methods for transformer use
func (u *Unit) Add(other *Unit) *Unit {
	return u.add(other)
}

func (u *Unit) Sub(other *Unit) *Unit {
	return u.sub(other)
}

func (u *Unit) Multiply(scalar any) *Unit {
	switch v := scalar.(type) {
	case float64:
		return u.withValue(u.value * v)
	case int:
		return u.withValue(u.value * float64(v))
	case float32:
		return u.withValue(u.value * float64(v))
	default:
		panic(fmt.Sprintf("Cannot multiply unit by %T", scalar))
	}
}

func (u *Unit) Divide(scalar float64) *Unit {
	return u.withValue(u.value / scalar)
}

func (u *Unit) toString() string {
	return fmt.Sprintf("%.3g%s", u.value, u.symbol)
}

// Exported version for external use
func (u *Unit) ToString() string {
	return u.toString()
}

// Getter for the value field
func (u *Unit) Value() float64 {
	return u.value
}

// Equal checks if two units have the same value in their base units
func (u *Unit) Equal(other *Unit) bool {
	if u.category != other.category {
		return false // Cannot compare different categories
	}
	
	// Convert both to base units and compare
	thisBase := u.value * u.conversionFactor
	otherBase := other.value * other.conversionFactor
	
	// Use small epsilon for floating point comparison
	epsilon := 1e-9
	return (thisBase - otherBase) < epsilon && (otherBase - thisBase) < epsilon
}

// Global unit constants for multiplication syntax like 2*km  
var (
	// Distance units
	M   = NewUnit("meter", "m", "m", 1.0, "distance").withValue(1.0) 
	Km  = NewUnit("kilometer", "km", "m", 1000.0, "distance").withValue(1.0)
	Cm  = NewUnit("centimeter", "cm", "m", 0.01, "distance").withValue(1.0)
	Mm  = NewUnit("millimeter", "mm", "m", 0.001, "distance").withValue(1.0)
	
	// Time units
	S   = NewUnit("second", "s", "s", 1.0, "time").withValue(1.0)
	Ms  = NewUnit("millisecond", "ms", "s", 0.001, "time").withValue(1.0) 
	H   = NewUnit("hour", "h", "s", 3600.0, "time").withValue(1.0)
	Min = NewUnit("minute", "min", "s", 60.0, "time").withValue(1.0)
	
	// Frequency units
	Hz_unit = NewUnit("hertz", "Hz", "Hz", 1.0, "frequency").withValue(1.0)
	KHz_unit = NewUnit("kilohertz", "kHz", "Hz", 1000.0, "frequency").withValue(1.0)
	MHz = NewUnit("megahertz", "MHz", "Hz", 1000000.0, "frequency").withValue(1.0)
	GHz = NewUnit("gigahertz", "GHz", "Hz", 1000000000.0, "frequency").withValue(1.0)
)

