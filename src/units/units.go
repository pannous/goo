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
	// Length/Distance units (base: meter)
	M   = NewUnit("meter", "m", "m", 1.0, "length").withValue(1.0) 
	Km  = NewUnit("kilometer", "km", "m", 1000.0, "length").withValue(1.0)
	Cm  = NewUnit("centimeter", "cm", "m", 0.01, "length").withValue(1.0)
	Mm  = NewUnit("millimeter", "mm", "m", 0.001, "length").withValue(1.0)
	Um  = NewUnit("micrometer", "μm", "m", 1e-6, "length").withValue(1.0)
	Nm  = NewUnit("nanometer", "nm", "m", 1e-9, "length").withValue(1.0)
	Pm  = NewUnit("picometer", "pm", "m", 1e-12, "length").withValue(1.0)
	Fm  = NewUnit("femtometer", "fm", "m", 1e-15, "length").withValue(1.0)
	
	// Imperial length units
	Inch = NewUnit("inch", "in", "m", 0.0254, "length").withValue(1.0)
	Ft   = NewUnit("foot", "ft", "m", 0.3048, "length").withValue(1.0)
	Yard = NewUnit("yard", "yd", "m", 0.9144, "length").withValue(1.0)
	Mile = NewUnit("mile", "mi", "m", 1609.344, "length").withValue(1.0)
	
	// Astronomical units
	NauticalMile = NewUnit("nautical mile", "nmi", "m", 1852.0, "length").withValue(1.0)
	AU           = NewUnit("astronomical unit", "AU", "m", 1.495978707e11, "length").withValue(1.0)
	LightYear    = NewUnit("light year", "ly", "m", 9.4607304725808e15, "length").withValue(1.0)
	Parsec       = NewUnit("parsec", "pc", "m", 3.0856775814913673e16, "length").withValue(1.0)
	
	// Time units (base: second)
	S   = NewUnit("second", "s", "s", 1.0, "time").withValue(1.0)
	Ms  = NewUnit("millisecond", "ms", "s", 0.001, "time").withValue(1.0) 
	Us  = NewUnit("microsecond", "μs", "s", 1e-6, "time").withValue(1.0)
	Ns  = NewUnit("nanosecond", "ns", "s", 1e-9, "time").withValue(1.0)
	Ps  = NewUnit("picosecond", "ps", "s", 1e-12, "time").withValue(1.0)
	Min = NewUnit("minute", "min", "s", 60.0, "time").withValue(1.0)
	H   = NewUnit("hour", "h", "s", 3600.0, "time").withValue(1.0)
	Day = NewUnit("day", "d", "s", 86400.0, "time").withValue(1.0)
	Week = NewUnit("week", "wk", "s", 604800.0, "time").withValue(1.0)
	Month = NewUnit("month", "mo", "s", 2629746.0, "time").withValue(1.0) // average month
	Year = NewUnit("year", "yr", "s", 31556952.0, "time").withValue(1.0) // average year
	
	// Mass units (base: kilogram)
	Kg = NewUnit("kilogram", "kg", "kg", 1.0, "mass").withValue(1.0)
	G  = NewUnit("gram", "g", "kg", 0.001, "mass").withValue(1.0)
	Mg = NewUnit("milligram", "mg", "kg", 1e-6, "mass").withValue(1.0)
	Ug = NewUnit("microgram", "μg", "kg", 1e-9, "mass").withValue(1.0)
	Ton = NewUnit("metric ton", "t", "kg", 1000.0, "mass").withValue(1.0)
	
	// Imperial mass units
	Lb = NewUnit("pound", "lb", "kg", 0.45359237, "mass").withValue(1.0)
	Oz = NewUnit("ounce", "oz", "kg", 0.028349523125, "mass").withValue(1.0)
	Stone = NewUnit("stone", "st", "kg", 6.35029318, "mass").withValue(1.0)
	
	// Scientific mass units
	Amu = NewUnit("atomic mass unit", "u", "kg", 1.66053906660e-27, "mass").withValue(1.0)
	
	// Temperature units (base: kelvin) - Note: these need special conversion handling
	K = NewUnit("kelvin", "K", "K", 1.0, "temperature").withValue(1.0)
	C = NewUnit("celsius", "°C", "K", 1.0, "temperature").withValue(1.0) // Special: C = K - 273.15
	F = NewUnit("fahrenheit", "°F", "K", 5.0/9.0, "temperature").withValue(1.0) // Special: F = (K - 273.15) * 9/5 + 32
	R = NewUnit("rankine", "°R", "K", 5.0/9.0, "temperature").withValue(1.0) // R = K * 5/9
	
	// Energy units (base: joule)
	J = NewUnit("joule", "J", "J", 1.0, "energy").withValue(1.0)
	KJ = NewUnit("kilojoule", "kJ", "J", 1000.0, "energy").withValue(1.0)
	MJ = NewUnit("megajoule", "MJ", "J", 1e6, "energy").withValue(1.0)
	Cal = NewUnit("calorie", "cal", "J", 4.184, "energy").withValue(1.0)
	Kcal = NewUnit("kilocalorie", "kcal", "J", 4184.0, "energy").withValue(1.0)
	BTU = NewUnit("british thermal unit", "BTU", "J", 1055.056, "energy").withValue(1.0)
	KWh = NewUnit("kilowatt hour", "kWh", "J", 3.6e6, "energy").withValue(1.0)
	EV = NewUnit("electron volt", "eV", "J", 1.602176634e-19, "energy").withValue(1.0)
	KeV = NewUnit("kiloelectron volt", "keV", "J", 1.602176634e-16, "energy").withValue(1.0)
	MeV = NewUnit("megaelectron volt", "MeV", "J", 1.602176634e-13, "energy").withValue(1.0)
	
	// Power units (base: watt)
	W = NewUnit("watt", "W", "W", 1.0, "power").withValue(1.0)
	KW = NewUnit("kilowatt", "kW", "W", 1000.0, "power").withValue(1.0)
	MW = NewUnit("megawatt", "MW", "W", 1e6, "power").withValue(1.0)
	GW = NewUnit("gigawatt", "GW", "W", 1e9, "power").withValue(1.0)
	HP = NewUnit("horsepower", "hp", "W", 745.7, "power").withValue(1.0)
	BTUh = NewUnit("BTU per hour", "BTU/h", "W", 0.29307107, "power").withValue(1.0)
	
	// Pressure units (base: pascal)
	Pa = NewUnit("pascal", "Pa", "Pa", 1.0, "pressure").withValue(1.0)
	KPa = NewUnit("kilopascal", "kPa", "Pa", 1000.0, "pressure").withValue(1.0)
	MPa = NewUnit("megapascal", "MPa", "Pa", 1e6, "pressure").withValue(1.0)
	Bar = NewUnit("bar", "bar", "Pa", 1e5, "pressure").withValue(1.0)
	Atm = NewUnit("atmosphere", "atm", "Pa", 101325.0, "pressure").withValue(1.0)
	Psi = NewUnit("pounds per square inch", "psi", "Pa", 6894.757, "pressure").withValue(1.0)
	MmHg = NewUnit("millimeter of mercury", "mmHg", "Pa", 133.322, "pressure").withValue(1.0)
	Torr = NewUnit("torr", "torr", "Pa", 133.322, "pressure").withValue(1.0)
	
	// Electric units (base SI units)
	A = NewUnit("ampere", "A", "A", 1.0, "current").withValue(1.0)
	V = NewUnit("volt", "V", "V", 1.0, "voltage").withValue(1.0)
	Ohm = NewUnit("ohm", "Ω", "Ω", 1.0, "resistance").withValue(1.0)
	F_unit = NewUnit("farad", "F", "F", 1.0, "capacitance").withValue(1.0)
	H_unit = NewUnit("henry", "H", "H", 1.0, "inductance").withValue(1.0)
	Wb = NewUnit("weber", "Wb", "Wb", 1.0, "magnetic_flux").withValue(1.0)
	T_unit = NewUnit("tesla", "T", "T", 1.0, "magnetic_field").withValue(1.0)
	
	// Frequency units (base: hertz)
	Hz_unit = NewUnit("hertz", "Hz", "Hz", 1.0, "frequency").withValue(1.0)
	KHz_unit = NewUnit("kilohertz", "kHz", "Hz", 1000.0, "frequency").withValue(1.0)
	MHz = NewUnit("megahertz", "MHz", "Hz", 1000000.0, "frequency").withValue(1.0)
	GHz = NewUnit("gigahertz", "GHz", "Hz", 1000000000.0, "frequency").withValue(1.0)
	THz = NewUnit("terahertz", "THz", "Hz", 1e12, "frequency").withValue(1.0)
	Rpm = NewUnit("revolutions per minute", "rpm", "Hz", 1.0/60.0, "frequency").withValue(1.0)
	
	// Area units (derived from length²)
	M2 = NewUnit("square meter", "m²", "m²", 1.0, "area").withValue(1.0)
	Km2 = NewUnit("square kilometer", "km²", "m²", 1e6, "area").withValue(1.0)
	Cm2 = NewUnit("square centimeter", "cm²", "m²", 1e-4, "area").withValue(1.0)
	Mm2 = NewUnit("square millimeter", "mm²", "m²", 1e-6, "area").withValue(1.0)
	Hectare = NewUnit("hectare", "ha", "m²", 1e4, "area").withValue(1.0)
	Acre = NewUnit("acre", "ac", "m²", 4046.856, "area").withValue(1.0)
	SqFt = NewUnit("square foot", "ft²", "m²", 0.092903, "area").withValue(1.0)
	SqIn = NewUnit("square inch", "in²", "m²", 0.00064516, "area").withValue(1.0)
	
	// Volume units (derived from length³)
	M3 = NewUnit("cubic meter", "m³", "m³", 1.0, "volume").withValue(1.0)
	L = NewUnit("liter", "L", "m³", 0.001, "volume").withValue(1.0)
	ML = NewUnit("milliliter", "mL", "m³", 1e-6, "volume").withValue(1.0)
	Gallon = NewUnit("gallon", "gal", "m³", 0.003785412, "volume").withValue(1.0)
	Quart = NewUnit("quart", "qt", "m³", 0.000946353, "volume").withValue(1.0)
	Pint = NewUnit("pint", "pt", "m³", 0.000473176, "volume").withValue(1.0)
	Cup = NewUnit("cup", "cup", "m³", 0.000236588, "volume").withValue(1.0)
	FlOz = NewUnit("fluid ounce", "fl oz", "m³", 2.95735e-5, "volume").withValue(1.0)
	Barrel = NewUnit("barrel", "bbl", "m³", 0.158987, "volume").withValue(1.0)
	
	// Velocity units (derived: m/s)
	Mps = NewUnit("meters per second", "m/s", "m/s", 1.0, "velocity").withValue(1.0)
	Kmh = NewUnit("kilometers per hour", "km/h", "m/s", 1.0/3.6, "velocity").withValue(1.0)
	Mph = NewUnit("miles per hour", "mph", "m/s", 0.44704, "velocity").withValue(1.0)
	Knot = NewUnit("knot", "kn", "m/s", 0.514444, "velocity").withValue(1.0)
	Fps = NewUnit("feet per second", "ft/s", "m/s", 0.3048, "velocity").withValue(1.0)
	
	// Acceleration units (derived: m/s²)
	Mps2 = NewUnit("meters per second squared", "m/s²", "m/s²", 1.0, "acceleration").withValue(1.0)
	G_force = NewUnit("g-force", "g", "m/s²", 9.80665, "acceleration").withValue(1.0)
	
	// Angle units (base: radian)
	Rad = NewUnit("radian", "rad", "rad", 1.0, "angle").withValue(1.0)
	Deg = NewUnit("degree", "°", "rad", 0.017453292519943295, "angle").withValue(1.0)
	Grad = NewUnit("gradian", "grad", "rad", 0.015707963267948967, "angle").withValue(1.0)
	Turn = NewUnit("turn", "turn", "rad", 6.283185307179586, "angle").withValue(1.0)
)

