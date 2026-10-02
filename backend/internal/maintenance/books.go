package maintenance

import (
	"context"
	"embed"
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sh1su/vector/backend/internal/kernel"
	"github.com/sh1su/vector/backend/internal/platform/problem"
	"github.com/sh1su/vector/backend/internal/vehicles"
)

// Wartungsbücher sind Vorlagen mit Herstellerintervallen (books/*.json). Übernehmen legt daraus
// gewöhnliche Wartungsdefinitionen an, die der Nutzer danach frei anpasst.

//go:embed books/*.json
var bookFiles embed.FS

type BookItem struct {
	Key                   string      `json:"key"`
	Title                 string      `json:"title"`
	Description           *string     `json:"description,omitempty"`
	Category              string      `json:"category"`
	IntervalMonths        *int        `json:"interval_months,omitempty"`
	IntervalDistance      *vehicles.Q `json:"interval_distance,omitempty"`
	FirstIntervalMonths   *int        `json:"first_interval_months,omitempty"`
	FirstIntervalDistance *vehicles.Q `json:"first_interval_distance,omitempty"`
}

type Book struct {
	ID      string     `json:"id"`
	Make    string     `json:"make"`
	Model   string     `json:"model"`
	Variant *string    `json:"variant,omitempty"`
	Title   string     `json:"title"`
	Source  string     `json:"source"`
	Items   []BookItem `json:"items"`
}

var books = mustLoadBooks()

func mustLoadBooks() []Book {
	entries, err := bookFiles.ReadDir("books")
	if err != nil {
		panic(err)
	}
	var out []Book
	for _, e := range entries {
		b, err := bookFiles.ReadFile("books/" + e.Name())
		if err != nil {
			panic(err)
		}
		var bk Book
		if err := json.Unmarshal(b, &bk); err != nil {
			panic("Wartungsbuch " + e.Name() + ": " + err.Error())
		}
		out = append(out, bk)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Make+out[i].Model < out[j].Make+out[j].Model })
	return out
}

// Books liefert die Wartungsbücher, optional nur einer Marke.
func Books(mk string) []Book {
	out := []Book{}
	for _, b := range books {
		if mk == "" || strings.EqualFold(strings.TrimSpace(mk), b.Make) {
			out = append(out, b)
		}
	}
	return out
}

func bookByID(id string) (Book, bool) {
	for _, b := range books {
		if b.ID == id {
			return b, true
		}
	}
	return Book{}, false
}

type ApplyInput struct {
	ItemKeys       []string    `json:"item_keys"`
	AnchorDate     *string     `json:"anchor_date"`
	AnchorOdometer *vehicles.Q `json:"anchor_odometer"`
	SinceNew       bool        `json:"since_new"`
}

type ApplyResult struct {
	Created []ItemView
	Skipped []string
}

// bookNS ist der Namensraum für die IDs übernommener Positionen: dieselbe Position am selben
// Fahrzeug erhält immer dieselbe ID, eine Wiederholung legt deshalb nichts doppelt an.
var bookNS = uuid.MustParse("0b7d6f3e-4f0a-4c55-9d3a-6f1f1c0b5e10")

// ApplyBook legt die Positionen eines Wartungsbuchs als Wartungsdefinitionen an. Positionen, deren
// Titel am Fahrzeug schon existiert, werden übersprungen.
func (s *Service) ApplyBook(ctx context.Context, actor kernel.Actor, vehicleID uuid.UUID, bookID string, in ApplyInput) (ApplyResult, error) {
	bk, ok := bookByID(bookID)
	if !ok {
		return ApplyResult{}, problem.NotFound()
	}
	want := map[string]bool{}
	for _, k := range in.ItemKeys {
		want[k] = true
	}
	var anchor *time.Time
	if in.AnchorDate != nil {
		t, err := time.Parse(time.DateOnly, *in.AnchorDate)
		if err != nil {
			return ApplyResult{}, problem.Validation(problem.FieldError{Pointer: "/anchor_date", Code: "invalid_date"})
		}
		anchor = &t
	}
	existing, err := s.ListItems(ctx, actor, vehicleID, false, nil)
	if err != nil {
		return ApplyResult{}, err
	}
	have := map[string]bool{}
	for _, it := range existing {
		have[strings.ToLower(it.Title)] = true
	}
	res := ApplyResult{Created: []ItemView{}, Skipped: []string{}}
	for _, bi := range bk.Items {
		if len(want) > 0 && !want[bi.Key] {
			continue
		}
		if have[strings.ToLower(bi.Title)] {
			res.Skipped = append(res.Skipped, bi.Title)
			continue
		}
		item := ItemInput{Title: bi.Title, Description: bi.Description, Category: bi.Category, ScheduleMode: ModeFromLast,
			IntervalMonths: bi.IntervalMonths, IntervalDistance: bi.IntervalDistance,
			Note: "Aus Wartungsbuch „" + bk.Title + "“. " + bk.Source}
		if anchor != nil {
			a := *anchor
			if in.SinceNew && bi.FirstIntervalMonths != nil && bi.IntervalMonths != nil {
				// Erstes Intervall abweichend: Basis so verschieben, dass die erste Fälligkeit stimmt.
				a = kernel.AddMonths(a, *bi.FirstIntervalMonths-*bi.IntervalMonths)
			}
			d := a.Format(time.DateOnly)
			item.AnchorDate = &d
		}
		if in.AnchorOdometer != nil && bi.IntervalDistance != nil && bi.IntervalDistance.Unit == in.AnchorOdometer.Unit {
			o := *in.AnchorOdometer
			if in.SinceNew && bi.FirstIntervalDistance != nil && bi.FirstIntervalDistance.Unit == o.Unit {
				o.Value += bi.FirstIntervalDistance.Value - bi.IntervalDistance.Value
			}
			item.AnchorOdometer = &o
		}
		id := uuid.NewSHA1(bookNS, []byte(vehicleID.String()+"/"+bk.ID+"/"+bi.Key))
		v, _, err := s.CreateItem(ctx, actor, vehicleID, &id, item)
		if err != nil {
			return ApplyResult{}, err
		}
		res.Created = append(res.Created, v)
	}
	return res, nil
}
