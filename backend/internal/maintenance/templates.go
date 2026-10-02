package maintenance

// Vorlagen für typische Wartungspläne (OP-MA-1). Die Werte sind eigene,
// neu formulierte Richtwerte; sie ersetzen nicht das Serviceheft. Deshalb sind
// sie nicht als Herstellervorgabe markiert (manufacturer_recommended = false,
// AP-10: eine Herstellervorgabe braucht eine Quelle). Der Nutzer passt die
// Intervalle vor dem Übernehmen an und trägt die letzte Erledigung als Anker ein.

// TemplateItem ist eine Position einer Vorlage (Schema MaintenanceTemplateItem).
type TemplateItem struct {
	Key            string `json:"key"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	ScheduleMode   string `json:"schedule_mode"`
	IntervalMonths *int   `json:"interval_months"`
	IntervalKm     *int   `json:"interval_km"`
	Optional       bool   `json:"optional"`
}

// Template ist ein Wartungsplan (Schema MaintenanceTemplate).
type Template struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	AppliesTo   string         `json:"applies_to"`
	BodyTypes   []string       `json:"body_types"`
	Carriers    []string       `json:"energy_carriers"`
	Note        string         `json:"note"`
	Items       []TemplateItem `json:"items"`
}

func n(v int) *int { return &v }

func ti(key, title, cat string, months, km int, desc string) TemplateItem {
	it := TemplateItem{Key: key, Title: title, Category: cat, ScheduleMode: ModeFromLast, Description: desc}
	if months > 0 {
		it.IntervalMonths = n(months)
	}
	if km > 0 {
		it.IntervalKm = n(km)
	}
	return it
}

func opt(it TemplateItem) TemplateItem { it.Optional = true; return it }

const richtwert = "Richtwerte ohne Gewähr. Maßgeblich sind Serviceheft, Wartungsnachweis und die Serviceanzeige (ASSYST PLUS) des Fahrzeugs; " +
	"bei erschwerten Bedingungen (Kurzstrecke, Anhänger, Staub, hohe Zuladung) verkürzen sich die Intervalle."

// Templates ist der Katalog der eingebauten Vorlagen.
var Templates = []Template{
	{
		ID:          "mb-sprinter-vs30-cdi-2019",
		Title:       "Mercedes-Benz Sprinter CDI (Baureihe 907/910, ab 2018)",
		Description: "Diesel-Transporter bzw. Basisfahrzeug für Reisemobile, z. B. Modelljahr 2019 mit OM651/OM654 und Steuerkette.",
		AppliesTo:   "Sprinter 211–519 CDI, Modelljahre 2018–2023",
		BodyTypes:   []string{"van", "camper", "truck"},
		Carriers:    []string{"diesel"},
		Note:        richtwert,
		Items: []TemplateItem{
			ti("service", "Wartung (Service A/B) mit Motoröl und Ölfilter", "service", 24, 40000,
				"Wechselnd Service A und B nach ASSYST PLUS. Motoröl nach MB-Freigabe 229.51/229.52 (z. B. 5W-30)."),
			ti("cabin_filter", "Innenraum-/Staubfilter", "filters", 24, 40000, "Mit jeder Wartung."),
			ti("air_filter", "Luftfilter", "filters", 48, 80000, "Bei Staub früher."),
			ti("fuel_filter", "Kraftstofffilter (Diesel)", "filters", 48, 80000, "Mit Entwässerung des Wasserabscheiders."),
			ti("brake_fluid", "Bremsflüssigkeit", "brakes", 24, 0, "Wechsel unabhängig von der Laufleistung."),
			ti("brakes_check", "Bremsbeläge und -scheiben prüfen", "brakes", 12, 20000, "Sichtprüfung, Verschleißanzeige."),
			ti("coolant", "Kühlmittel", "fluids", 180, 250000, "Langzeitkühlmittel nach MB 325.x."),
			ti("belt", "Keilrippenriemen und Spannrolle", "service", 72, 120000, "Steuerkette: kein regelmäßiger Wechsel vorgesehen."),
			opt(ti("atf", "Automatikgetriebeöl und Filter (7G-/9G-TRONIC)", "fluids", 0, 125000, "Nur bei Automatikgetriebe.")),
			opt(ti("adblue_filter", "AdBlue-Filter (SCR)", "filters", 0, 200000, "Nur bei Euro-6-Motoren mit SCR.")),
			ti("ac_service", "Klimaanlage (Kältemittel, Desinfektion)", "service", 24, 0, ""),
			ti("battery", "Starterbatterie prüfen", "other", 12, 0, "Ruhespannung und Pole."),
			ti("tires_age", "Reifen: Alter und Profil prüfen", "tires", 12, 0, "Reifen älter als 6–8 Jahre ersetzen (DOT-Nummer)."),
			ti("hu_au", "Hauptuntersuchung mit AU (HU/TÜV)", "legal_inspection", 24, 0,
				"Bis 3,5 t: alle 24 Monate (erste HU nach 36 Monaten). Über 3,5 t gelten kürzere Fristen."),
		},
	},
	{
		ID:          "mb-vito-w447-cdi-2019",
		Title:       "Mercedes-Benz Vito / V-Klasse / Marco Polo CDI (W447, ab 2014)",
		Description: "Diesel-Kleinbus, auch Basis von Campern, z. B. Modelljahr 2019 mit OM651/OM654.",
		AppliesTo:   "Vito 110–119 CDI, V 200–300 d, Modelljahre 2014–2023",
		BodyTypes:   []string{"van", "camper", "car"},
		Carriers:    []string{"diesel"},
		Note:        richtwert,
		Items: []TemplateItem{
			ti("service", "Wartung (Service A/B) mit Motoröl und Ölfilter", "service", 24, 40000,
				"Wechselnd Service A und B. Motoröl nach MB-Freigabe 229.51/229.52."),
			ti("cabin_filter", "Innenraum-/Staubfilter", "filters", 24, 40000, ""),
			ti("air_filter", "Luftfilter", "filters", 48, 80000, ""),
			ti("fuel_filter", "Kraftstofffilter (Diesel)", "filters", 48, 80000, ""),
			ti("brake_fluid", "Bremsflüssigkeit", "brakes", 24, 0, ""),
			ti("brakes_check", "Bremsbeläge und -scheiben prüfen", "brakes", 12, 20000, ""),
			ti("coolant", "Kühlmittel", "fluids", 180, 250000, "Langzeitkühlmittel nach MB 325.x."),
			ti("belt", "Keilrippenriemen", "service", 72, 120000, "Steuerkette: kein regelmäßiger Wechsel vorgesehen."),
			opt(ti("atf", "Automatikgetriebeöl und Filter (7G-/9G-TRONIC)", "fluids", 0, 125000, "Nur bei Automatikgetriebe.")),
			ti("ac_service", "Klimaanlage", "service", 24, 0, ""),
			ti("battery", "Starter- und Stützbatterie prüfen", "other", 12, 0, ""),
			ti("tires_age", "Reifen: Alter und Profil prüfen", "tires", 12, 0, ""),
			ti("hu_au", "Hauptuntersuchung mit AU (HU/TÜV)", "legal_inspection", 24, 0, "Erste HU nach 36 Monaten, danach alle 24 Monate."),
		},
	},
	{
		ID:          "camper-body",
		Title:       "Reisemobil-Aufbau (Ergänzung)",
		Description: "Aufbauspezifische Prüfungen für Wohnmobile und Camper, ergänzend zum Plan des Basisfahrzeugs.",
		AppliesTo:   "Reisemobile und Campervans mit Flüssiggasanlage",
		BodyTypes:   []string{"camper", "van"},
		Note:        "Prüffristen in Deutschland; Herstellergarantie auf die Dichtigkeit setzt meist die jährliche Prüfung voraus.",
		Items: []TemplateItem{
			ti("gas_g607", "Gasprüfung nach DVGW G 607", "legal_inspection", 24, 0, "Prüfbescheinigung und Plakette; Voraussetzung für die HU."),
			ti("gas_hoses", "Gasschläuche und Druckregler ersetzen", "other", 120, 0, "Spätestens nach 10 Jahren bzw. laut Aufdruck."),
			ti("leak_test", "Dichtigkeitsprüfung des Aufbaus", "other", 12, 0, "Feuchtigkeitsmessung durch Fachbetrieb."),
			ti("water", "Frischwasseranlage reinigen und desinfizieren", "fluids", 12, 0, "Vor der Saison."),
			ti("leisure_battery", "Aufbaubatterie und Ladegerät prüfen", "other", 12, 0, ""),
			ti("extinguisher", "Feuerlöscher prüfen", "other", 24, 0, ""),
			ti("smoke_co", "Rauch- und CO-Melder testen", "other", 12, 0, ""),
		},
	},
	{
		ID:          "car-generic-de",
		Title:       "PKW allgemein (Deutschland)",
		Description: "Grundplan für Fahrzeuge ohne eigene Vorlage.",
		AppliesTo:   "PKW mit Verbrennungsmotor",
		BodyTypes:   []string{"car", "van"},
		Carriers:    []string{"petrol", "diesel", "lpg"},
		Note:        "Allgemeine Richtwerte; Herstellerintervalle haben Vorrang.",
		Items: []TemplateItem{
			ti("oil", "Ölwechsel mit Filter", "service", 12, 15000, ""),
			ti("inspection", "Inspektion", "service", 24, 30000, ""),
			ti("brake_fluid", "Bremsflüssigkeit", "brakes", 24, 0, ""),
			ti("cabin_filter", "Innenraumfilter", "filters", 12, 15000, ""),
			opt(ti("timing_belt", "Zahnriemen", "service", 72, 120000, "Nur bei Motoren mit Zahnriemen.")),
			ti("hu_au", "Hauptuntersuchung mit AU (HU/TÜV)", "legal_inspection", 24, 0, ""),
		},
	},
}

// TemplateByID sucht eine Vorlage.
func TemplateByID(id string) (Template, bool) {
	for _, t := range Templates {
		if t.ID == id {
			return t, true
		}
	}
	return Template{}, false
}
