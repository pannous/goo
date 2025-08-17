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
	
	// Register common units
	r.registerDistanceUnits()
	r.registerTimeUnits()
	r.registerFrequencyUnits()
	
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