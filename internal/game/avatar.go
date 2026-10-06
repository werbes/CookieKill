package game

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

func validChoice(s string, choices ...string) bool {
	for _, v := range choices {
		if v == s {
			return true
		}
	}
	return false
}
func validColor(s string) bool {
	return validChoice(s, "teal", "blue", "red", "purple", "sand", "black", "white")
}
func validAvatar(a Avatar) bool {
	return a.Skin >= 0 && a.Skin < 5 && validChoice(a.Shirt, "tee", "hoodie", "tank") && validChoice(a.Pants, "trousers", "shorts") && validColor(a.ShirtColor) && validColor(a.PantsColor)
}
func normalizeIdentity(p *Profile) {
	if p.CallName == "" {
		p.CallName = p.Name
	}
	if p.CallName == "" {
		p.CallName = "Cookie explorer"
	}
	p.Name = p.CallName
	if p.Username == "" {
		var b strings.Builder
		for _, c := range strings.ToLower(p.Name) {
			if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
				b.WriteRune(c)
			}
			if b.Len() >= 14 {
				break
			}
		}
		if b.Len() == 0 {
			b.WriteString("baker")
		}
		sum := sha256.Sum256([]byte(p.ID))
		p.Username = fmt.Sprintf("%s_%x", b.String(), sum[:5])
	}
	if !validAvatar(p.Avatar) {
		p.Avatar = Avatar{Skin: 2, Shirt: "tee", Pants: "trousers", ShirtColor: "teal", PantsColor: "sand"}
	}
	if p.Discovered == nil {
		p.Discovered = map[string]bool{}
	}
	if p.Hats == nil {
		p.Hats = map[string]bool{}
	}
	if p.Equipment == nil {
		p.Equipment = map[string]bool{}
	}
	if p.Paint == nil {
		p.Paint = map[string]string{}
	}
	awardHats(p)
	if p.Avatar.Hat != "" && !p.Hats[p.Avatar.Hat] {
		p.Avatar.Hat = ""
	}
}
func awardHats(p *Profile) {
	if len(p.Discovered) > 0 {
		p.Hats["chef"] = true
	}
	if p.Cleaned >= 10 {
		p.Hats["recycler"] = true
	}
	if p.Rescues >= 3 {
		p.Hats["ocean"] = true
	}
	if p.Kills >= 5 {
		p.Hats["champion"] = true
	}
	if p.Bakery || p.Home != nil {
		p.Hats["builder"] = true
	}
}
func (w *World) customize(p *player, a Action) error {
	name := strings.TrimSpace(a.CallName)
	if name != "" && name != p.profile.CallName {
		if len([]rune(name)) < 2 || len([]rune(name)) > 24 {
			return errors.New("Use 2 to 24 characters for your call name.")
		}
		for _, c := range name {
			if unicode.IsControl(c) || c == '<' || c == '>' {
				return errors.New("Use plain text for your call name.")
			}
		}
		if p.profile.CallNameChangedAt > 0 && w.now().Unix()-p.profile.CallNameChangedAt < 3600 {
			return errors.New("Your call name can change once every hour.")
		}
	}
	if a.Avatar != nil {
		if !validAvatar(*a.Avatar) {
			return errors.New("Choose one of the available avatar styles and colors.")
		}
		if a.Avatar.Hat != "" && !p.profile.Hats[a.Avatar.Hat] {
			return errors.New("Earn that hat's achievement before wearing it.")
		}
	}
	if name != "" && name != p.profile.CallName {
		p.profile.CallName = name
		p.profile.Name = name
		p.profile.CallNameChangedAt = w.now().Unix()
	}
	if a.Avatar != nil {
		p.profile.Avatar = *a.Avatar
	}
	if p.profile.Home != nil {
		p.profile.Home.CallName = p.profile.CallName
	}
	return nil
}
