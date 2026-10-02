package kernel

// Units sind die Anzeigeeinheiten eines Nutzers (Schema DisplayUnits, ADR-007).
type Units struct {
	Distance            string // km | mi
	Volume              string // l | gal_us | gal_imp
	Consumption         string // l_per_100km | km_per_l | mpg_us | mpg_uk
	ElectricConsumption string // kwh_per_100km | km_per_kwh | mi_per_kwh
	OilVolume           string // ml | l | qt_us | qt_imp
}

// UnitsFromSettings liest die Anzeigeeinheiten aus den Nutzereinstellungen.
func UnitsFromSettings(st map[string]any) Units {
	u := Units{Distance: "km", Volume: "l", Consumption: "l_per_100km", ElectricConsumption: "kwh_per_100km", OilVolume: "l"}
	du, _ := st["display_units"].(map[string]any)
	pick := func(k string, dst *string, allowed ...string) {
		if v, ok := du[k].(string); ok {
			for _, a := range allowed {
				if v == a {
					*dst = v
				}
			}
		}
	}
	pick("distance", &u.Distance, "km", "mi")
	pick("volume", &u.Volume, "l", "gal_us", "gal_imp")
	pick("consumption", &u.Consumption, "l_per_100km", "km_per_l", "mpg_us", "mpg_uk")
	pick("electric_consumption", &u.ElectricConsumption, "kwh_per_100km", "km_per_kwh", "mi_per_kwh")
	pick("oil_volume", &u.OilVolume, "ml", "l", "qt_us", "qt_imp")
	return u
}

// Display rechnet einen kanonischen Wert in eine Anzeigeeinheit um und rundet.
func Display(canonical int64, unit string, digits int) float64 {
	v, err := FromCanonical(canonical, unit)
	if err != nil {
		return float64(canonical)
	}
	return Round(v, digits)
}

// UnitLabel ist die Beschriftung eines Einheitencodes.
func UnitLabel(u string) string {
	switch u {
	case "gal_us":
		return "gal (US)"
	case "gal_imp":
		return "gal (UK)"
	case "qt_us":
		return "qt (US)"
	case "qt_imp":
		return "qt (UK)"
	}
	return u
}
