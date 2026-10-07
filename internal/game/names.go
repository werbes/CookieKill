package game

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

func objectName(input string, maximum int) (string, error) {
	if !utf8.ValidString(input) {
		return "", errors.New("Use a readable name.")
	}
	for _, r := range input {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return "", errors.New("Use a name without control or invisible characters.")
		}
	}
	name := strings.Join(strings.Fields(input), " ")
	if n := utf8.RuneCountInString(name); n < 1 || n > maximum {
		return "", fmt.Errorf("Use 1 to %d characters for the name.", maximum)
	}
	return name, nil
}

func (w *World) renameBakery(p *player, input string) error {
	if err := w.requireBakery(p); err != nil {
		return err
	}
	name, err := objectName(input, 32)
	if err != nil {
		return err
	}
	p.profile.BakeryName = name
	w.eventActor("Your bakery is now called "+name+".", "notification", p.x, p.z, p.profile.ID)
	return nil
}

func (w *World) renameCamel(p *player, target, input string) error {
	name, err := objectName(input, 24)
	if err != nil {
		return err
	}
	for i := range p.profile.Camels {
		c := &p.profile.Camels[i]
		if c.ID != target || c.RentalUntil > 0 && c.RentalUntil <= w.now().Unix() {
			continue
		}
		c.Name = name
		w.eventActor("Your camel is now called "+name+".", "notification", p.x, p.z, p.profile.ID)
		return nil
	}
	return errors.New("Choose one of your own camels to name.")
}
