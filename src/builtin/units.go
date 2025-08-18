package builtin

import (
	"fmt"
	"strings"
)

type Unit struct {
	Value float64
	Type  string
	Scale float64  // conversion factor to base unit
}

type UnitRegistry struct {
	units map[string]*UnitInfo
}

type UnitInfo struct {
	BaseUnit string
	Scale    float64
	Category string
}

var DefaultRegistry = NewUnitRegistry()

func NewUnitRegistry() *UnitRegistry {
	r := &UnitRegistry{
		units: make(map[string]*UnitInfo),
	}
	
	// Register all unit categories
	r.registerDistanceUnits()
	r.registerTimeUnits()
	r.registerFrequencyUnits()
	r.registerMassUnits()
	r.registerTemperatureUnits()
	r.registerPressureUnits()
	r.registerEnergyUnits()
	r.registerPowerUnits()
	r.registerElectricUnits()
	r.registerAreaUnits()
	r.registerVolumeUnits()
	r.registerVelocityUnits()
	r.registerAccelerationUnits()
	r.registerAngleUnits()
	
	return r
}

func (r *UnitRegistry) registerDistanceUnits() {
	r.Register("m", "m", 1.0, "distance")
	r.Register("km", "m", 1000.0, "distance")
	r.Register("cm", "m", 0.01, "distance")
	r.Register("mm", "m", 0.001, "distance")
	r.Register("in", "m", 0.0254, "distance")
	r.Register("ft", "m", 0.3048, "distance")
	r.Register("mi", "m", 1609.34, "distance")
}

func (r *UnitRegistry) registerTimeUnits() {
	r.Register("s", "s", 1.0, "time")
	r.Register("ms", "s", 0.001, "time")
	r.Register("us", "s", 0.000001, "time")
	r.Register("ns", "s", 0.000000001, "time")
	r.Register("min", "s", 60.0, "time")
	r.Register("h", "s", 3600.0, "time")
	r.Register("d", "s", 86400.0, "time")
}

func (r *UnitRegistry) registerFrequencyUnits() {
	r.Register("Hz", "Hz", 1.0, "frequency")
	r.Register("kHz", "Hz", 1000.0, "frequency")
	r.Register("MHz", "Hz", 1000000.0, "frequency")
	r.Register("GHz", "Hz", 1000000000.0, "frequency")
	r.Register("THz", "Hz", 1000000000000.0, "frequency")
	r.Register("rpm", "Hz", 1.0/60.0, "frequency") // revolutions per minute
}

func (r *UnitRegistry) registerMassUnits() {
	r.Register("g", "kg", 0.001, "mass")
	r.Register("kg", "kg", 1.0, "mass")
	r.Register("mg", "kg", 0.000001, "mass")
	r.Register("μg", "kg", 0.000000001, "mass")
	r.Register("lb", "kg", 0.453592, "mass")
	r.Register("oz", "kg", 0.0283495, "mass")
	r.Register("st", "kg", 6.35029, "mass") // stone
	r.Register("t", "kg", 1000.0, "mass") // metric ton
	r.Register("u", "kg", 1.66054e-27, "mass") // atomic mass unit
}

func (r *UnitRegistry) registerTemperatureUnits() {
	r.Register("K", "K", 1.0, "temperature")
	r.Register("°C", "K", 1.0, "temperature") // Note: needs offset +273.15
	r.Register("°F", "K", 5.0/9.0, "temperature") // Note: needs (°F-32)*5/9 + 273.15
	r.Register("°R", "K", 5.0/9.0, "temperature") // Rankine
}

func (r *UnitRegistry) registerPressureUnits() {
	r.Register("Pa", "Pa", 1.0, "pressure")
	r.Register("kPa", "Pa", 1000.0, "pressure")
	r.Register("MPa", "Pa", 1000000.0, "pressure")
	r.Register("bar", "Pa", 100000.0, "pressure")
	r.Register("atm", "Pa", 101325.0, "pressure")
	r.Register("psi", "Pa", 6894.76, "pressure")
	r.Register("mmHg", "Pa", 133.322, "pressure")
	r.Register("torr", "Pa", 133.322, "pressure")
}

func (r *UnitRegistry) registerEnergyUnits() {
	r.Register("J", "J", 1.0, "energy")
	r.Register("kJ", "J", 1000.0, "energy")
	r.Register("MJ", "J", 1000000.0, "energy")
	r.Register("cal", "J", 4.184, "energy")
	r.Register("kcal", "J", 4184.0, "energy")
	r.Register("BTU", "J", 1055.06, "energy")
	r.Register("eV", "J", 1.60218e-19, "energy")
	r.Register("keV", "J", 1.60218e-16, "energy")
	r.Register("MeV", "J", 1.60218e-13, "energy")
	r.Register("kWh", "J", 3600000.0, "energy")
}

func (r *UnitRegistry) registerPowerUnits() {
	r.Register("W", "W", 1.0, "power")
	r.Register("kW", "W", 1000.0, "power")
	r.Register("MW", "W", 1000000.0, "power")
	r.Register("GW", "W", 1000000000.0, "power")
	r.Register("hp", "W", 745.7, "power") // mechanical horsepower
	r.Register("BTU/h", "W", 0.293071, "power")
}

func (r *UnitRegistry) registerElectricUnits() {
	r.Register("A", "A", 1.0, "current")
	r.Register("V", "V", 1.0, "voltage")
	r.Register("Ω", "Ω", 1.0, "resistance")
	r.Register("F", "F", 1.0, "capacitance")
	r.Register("H", "H", 1.0, "inductance")
	r.Register("Wb", "Wb", 1.0, "magnetic_flux")
	r.Register("T", "T", 1.0, "magnetic_field")
}

func (r *UnitRegistry) registerAreaUnits() {
	r.Register("m²", "m²", 1.0, "area")
	r.Register("km²", "m²", 1000000.0, "area")
	r.Register("cm²", "m²", 0.0001, "area")
	r.Register("mm²", "m²", 0.000001, "area")
	r.Register("ha", "m²", 10000.0, "area") // hectare
	r.Register("ac", "m²", 4046.86, "area") // acre
	r.Register("ft²", "m²", 0.092903, "area")
	r.Register("in²", "m²", 0.00064516, "area")
	// ASCII alternatives
	r.Register("sqm", "m²", 1.0, "area")
}

func (r *UnitRegistry) registerVolumeUnits() {
	r.Register("m³", "m³", 1.0, "volume")
	r.Register("L", "m³", 0.001, "volume")
	r.Register("mL", "m³", 0.000001, "volume")
	r.Register("gal", "m³", 0.00378541, "volume") // US gallon
	r.Register("qt", "m³", 0.000946353, "volume") // US quart
	r.Register("pt", "m³", 0.000473176, "volume") // US pint
	r.Register("cup", "m³", 0.000236588, "volume") // US cup
	r.Register("fl oz", "m³", 2.95735e-05, "volume") // US fluid ounce
	r.Register("bbl", "m³", 0.158987, "volume") // barrel
	// ASCII alternative
	r.Register("cbm", "m³", 1.0, "volume")
}

func (r *UnitRegistry) registerVelocityUnits() {
	r.Register("m/s", "m/s", 1.0, "velocity")
	r.Register("km/h", "m/s", 1.0/3.6, "velocity")
	r.Register("mph", "m/s", 0.44704, "velocity")
	r.Register("kn", "m/s", 0.514444, "velocity") // knot
	r.Register("ft/s", "m/s", 0.3048, "velocity")
}

func (r *UnitRegistry) registerAccelerationUnits() {
	r.Register("m/s²", "m/s²", 1.0, "acceleration")
	r.Register("mps2", "m/s²", 1.0, "acceleration") // ASCII alternative
	r.Register("gf", "m/s²", 9.80665, "acceleration") // g-force
}

func (r *UnitRegistry) registerAngleUnits() {
	r.Register("rad", "rad", 1.0, "angle")
	r.Register("°", "rad", 0.0174533, "angle") // degrees to radians
	r.Register("deg", "rad", 0.0174533, "angle")
	r.Register("grad", "rad", 0.0157080, "angle") // gradians
	r.Register("turn", "rad", 6.28319, "angle") // full turn
}

func (r *UnitRegistry) Register(unitName, baseUnit string, scale float64, category string) {
	r.units[unitName] = &UnitInfo{
		BaseUnit: baseUnit,
		Scale:    scale,
		Category: category,
	}
}

func (r *UnitRegistry) GetUnit(name string) (*UnitInfo, bool) {
	info, exists := r.units[name]
	return info, exists
}

func NewUnit(value float64, unitName string) *Unit {
	info, exists := DefaultRegistry.GetUnit(unitName)
	if !exists {
		panic(fmt.Sprintf("Unknown unit: %s", unitName))
	}
	
	return &Unit{
		Value: value,
		Type:  unitName,
		Scale: info.Scale,
	}
}

func (u *Unit) Add(other *Unit) *Unit {
	if !u.isCompatible(other) {
		panic(fmt.Sprintf("Cannot add incompatible units: %s + %s", u.Type, other.Type))
	}
	
	// Convert both to base units, add, then convert back to first unit
	thisBase := u.Value * u.Scale
	otherBase := other.Value * other.Scale
	resultBase := thisBase + otherBase
	
	return &Unit{
		Value: resultBase / u.Scale,
		Type:  u.Type,
		Scale: u.Scale,
	}
}

func (u *Unit) Sub(other *Unit) *Unit {
	if !u.isCompatible(other) {
		panic(fmt.Sprintf("Cannot subtract incompatible units: %s - %s", u.Type, other.Type))
	}
	
	thisBase := u.Value * u.Scale
	otherBase := other.Value * other.Scale
	resultBase := thisBase - otherBase
	
	return &Unit{
		Value: resultBase / u.Scale,
		Type:  u.Type,
		Scale: u.Scale,
	}
}

func (u *Unit) Mul(other *Unit) *Unit {
	// For multiplication, create compound unit
	newType := u.Type + "*" + other.Type
	newScale := u.Scale * other.Scale
	
	return &Unit{
		Value: u.Value * other.Value,
		Type:  newType,
		Scale: newScale,
	}
}

func (u *Unit) Div(other *Unit) *Unit {
	newType := u.Type + "/" + other.Type
	newScale := u.Scale / other.Scale
	
	return &Unit{
		Value: u.Value / other.Value,
		Type:  newType,
		Scale: newScale,
	}
}

func (u *Unit) isCompatible(other *Unit) bool {
	uInfo, uExists := DefaultRegistry.GetUnit(u.Type)
	oInfo, oExists := DefaultRegistry.GetUnit(other.Type)
	
	if !uExists || !oExists {
		return false
	}
	
	return uInfo.Category == oInfo.Category
}

func (u *Unit) ConvertTo(targetUnit string) *Unit {
	targetInfo, exists := DefaultRegistry.GetUnit(targetUnit)
	if !exists {
		panic(fmt.Sprintf("Unknown target unit: %s", targetUnit))
	}
	
	if !u.isCompatibleWith(targetUnit) {
		panic(fmt.Sprintf("Cannot convert %s to %s", u.Type, targetUnit))
	}
	
	// Convert to base units, then to target
	baseValue := u.Value * u.Scale
	targetValue := baseValue / targetInfo.Scale
	
	return &Unit{
		Value: targetValue,
		Type:  targetUnit,
		Scale: targetInfo.Scale,
	}
}

func (u *Unit) isCompatibleWith(unitName string) bool {
	uInfo, uExists := DefaultRegistry.GetUnit(u.Type)
	tInfo, tExists := DefaultRegistry.GetUnit(unitName)
	
	if !uExists || !tExists {
		return false
	}
	
	return uInfo.Category == tInfo.Category
}

func (u *Unit) String() string {
	return fmt.Sprintf("%.3g%s", u.Value, u.Type)
}

// Available returns a list of all supported units organized by category
func Available() map[string][]string {
	categories := make(map[string][]string)
	
	for unitName, info := range DefaultRegistry.units {
		categories[info.Category] = append(categories[info.Category], unitName)
	}
	
	return categories
}

// AvailableFlat returns a flat list of all supported unit names
func AvailableFlat() []string {
	var units []string
	for unitName := range DefaultRegistry.units {
		units = append(units, unitName)
	}
	return units
}

// ListByCategory prints all units organized by category
func ListByCategory() {
	categories := Available()
	for category, units := range categories {
		fmt.Printf("%s units: %s\n", strings.Title(category), strings.Join(units, ", "))
	}
}

// Common unit synonyms for easier access
var (
	// Distance synonyms
	M     = func(value float64) *Unit { return NewUnit(value, "m") }
	Meter = func(value float64) *Unit { return NewUnit(value, "m") }
	KM    = func(value float64) *Unit { return NewUnit(value, "km") }
	CM    = func(value float64) *Unit { return NewUnit(value, "cm") }
	MM    = func(value float64) *Unit { return NewUnit(value, "mm") }
	
	// Time synonyms
	S      = func(value float64) *Unit { return NewUnit(value, "s") }
	Second = func(value float64) *Unit { return NewUnit(value, "s") }
	MS     = func(value float64) *Unit { return NewUnit(value, "ms") }
	Min    = func(value float64) *Unit { return NewUnit(value, "min") }
	Hour   = func(value float64) *Unit { return NewUnit(value, "h") }
	
	// Mass synonyms
	G    = func(value float64) *Unit { return NewUnit(value, "g") }
	Gram = func(value float64) *Unit { return NewUnit(value, "g") }
	KG   = func(value float64) *Unit { return NewUnit(value, "kg") }
	
	// Temperature synonyms
	Kelvin  = func(value float64) *Unit { return NewUnit(value, "K") }
	Celsius = func(value float64) *Unit { return NewUnit(value, "°C") }
	
	// Pressure synonyms
	Pascal = func(value float64) *Unit { return NewUnit(value, "Pa") }
	Bar    = func(value float64) *Unit { return NewUnit(value, "bar") }
	ATM    = func(value float64) *Unit { return NewUnit(value, "atm") }
	
	// Energy synonyms
	Joule = func(value float64) *Unit { return NewUnit(value, "J") }
	Cal   = func(value float64) *Unit { return NewUnit(value, "cal") }
	
	// Power synonyms
	Watt = func(value float64) *Unit { return NewUnit(value, "W") }
	HP   = func(value float64) *Unit { return NewUnit(value, "hp") }
	
	// Electric synonyms
	Amp  = func(value float64) *Unit { return NewUnit(value, "A") }
	Volt = func(value float64) *Unit { return NewUnit(value, "V") }
	Ohm  = func(value float64) *Unit { return NewUnit(value, "Ω") }
	
	// Area synonyms
	M2 = func(value float64) *Unit { return NewUnit(value, "m²") }
	
	// Volume synonyms
	M3    = func(value float64) *Unit { return NewUnit(value, "m³") }
	Liter = func(value float64) *Unit { return NewUnit(value, "L") }
	
	// Angle synonyms
	Radian = func(value float64) *Unit { return NewUnit(value, "rad") }
	Degree = func(value float64) *Unit { return NewUnit(value, "°") }
)