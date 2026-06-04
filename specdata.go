package main

import "strings"

// SimCSpec maps a SimC class+spec string combination to a WoW spec ID.
// Class key is the first line identifier (e.g. "demonhunter"),
// spec key is the spec= value (e.g. "devourer").
// Spec IDs match KeystoneLoot's ItemDatabase class/spec arrays.

type SpecInfo struct {
	SpecID  int
	ClassID int
	Name    string
}

// SimCToSpec maps simC class name -> spec name -> SpecInfo
var SimCToSpec = map[string]map[string]SpecInfo{
	"warrior": {
		"arms":       {SpecID: 71, ClassID: 1, Name: "Arms"},
		"fury":       {SpecID: 72, ClassID: 1, Name: "Fury"},
		"protection": {SpecID: 73, ClassID: 1, Name: "Protection"},
	},
	"paladin": {
		"holy":        {SpecID: 65, ClassID: 2, Name: "Holy"},
		"protection":  {SpecID: 66, ClassID: 2, Name: "Protection"},
		"retribution": {SpecID: 70, ClassID: 2, Name: "Retribution"},
	},
	"hunter": {
		"beast_mastery": {SpecID: 253, ClassID: 3, Name: "Beast Mastery"},
		"marksmanship":  {SpecID: 254, ClassID: 3, Name: "Marksmanship"},
		"survival":      {SpecID: 255, ClassID: 3, Name: "Survival"},
	},
	"rogue": {
		"assassination": {SpecID: 259, ClassID: 4, Name: "Assassination"},
		"outlaw":        {SpecID: 260, ClassID: 4, Name: "Outlaw"},
		"subtlety":      {SpecID: 261, ClassID: 4, Name: "Subtlety"},
	},
	"priest": {
		"discipline": {SpecID: 256, ClassID: 5, Name: "Discipline"},
		"holy":       {SpecID: 257, ClassID: 5, Name: "Holy"},
		"shadow":     {SpecID: 258, ClassID: 5, Name: "Shadow"},
	},
	"deathknight": {
		"blood":  {SpecID: 250, ClassID: 6, Name: "Blood"},
		"frost":  {SpecID: 251, ClassID: 6, Name: "Frost"},
		"unholy": {SpecID: 252, ClassID: 6, Name: "Unholy"},
	},
	"shaman": {
		"elemental":   {SpecID: 262, ClassID: 7, Name: "Elemental"},
		"enhancement": {SpecID: 263, ClassID: 7, Name: "Enhancement"},
		"restoration": {SpecID: 264, ClassID: 7, Name: "Restoration"},
	},
	"mage": {
		"arcane": {SpecID: 62, ClassID: 8, Name: "Arcane"},
		"fire":   {SpecID: 63, ClassID: 8, Name: "Fire"},
		"frost":  {SpecID: 64, ClassID: 8, Name: "Frost"},
	},
	"warlock": {
		"affliction":  {SpecID: 265, ClassID: 9, Name: "Affliction"},
		"demonology":  {SpecID: 266, ClassID: 9, Name: "Demonology"},
		"destruction": {SpecID: 267, ClassID: 9, Name: "Destruction"},
	},
	"monk": {
		"brewmaster": {SpecID: 268, ClassID: 10, Name: "Brewmaster"},
		"mistweaver": {SpecID: 269, ClassID: 10, Name: "Mistweaver"},
		"windwalker": {SpecID: 270, ClassID: 10, Name: "Windwalker"},
	},
	"druid": {
		"balance":     {SpecID: 102, ClassID: 11, Name: "Balance"},
		"feral":       {SpecID: 103, ClassID: 11, Name: "Feral"},
		"guardian":    {SpecID: 104, ClassID: 11, Name: "Guardian"},
		"restoration": {SpecID: 105, ClassID: 11, Name: "Restoration"},
	},
	"demonhunter": {
		"havoc":     {SpecID: 577, ClassID: 12, Name: "Havoc"},
		"vengeance": {SpecID: 581, ClassID: 12, Name: "Vengeance"},
		"devourer":  {SpecID: 1480, ClassID: 12, Name: "Devourer"},
	},
	"evoker": {
		"devastation":  {SpecID: 1467, ClassID: 13, Name: "Devastation"},
		"preservation": {SpecID: 1468, ClassID: 13, Name: "Preservation"},
		"augmentation": {SpecID: 1473, ClassID: 13, Name: "Augmentation"},
	},
}

// ParseSimC extracts class, spec name and SpecInfo from a SimC export string.
func ParseSimC(input string) (className, specName string, info SpecInfo, ok bool) {
	lines := splitLines(input)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// First non-comment line: class="charname"
		if className == "" {
			eqIdx := strings.Index(line, "=")
			if eqIdx > 0 {
				className = strings.TrimSpace(line[:eqIdx])
				continue
			}
		}

		// Look for spec=
		if strings.HasPrefix(line, "spec=") {
			specName = strings.TrimSpace(strings.TrimPrefix(line, "spec="))
			continue
		}
	}

	if className == "" || specName == "" {
		return "", "", SpecInfo{}, false
	}

	specs, classOK := SimCToSpec[className]
	if !classOK {
		return className, specName, SpecInfo{}, false
	}

	info, specOK := specs[specName]
	if !specOK {
		return className, specName, SpecInfo{}, false
	}

	return className, specName, info, true
}

func splitLines(s string) []string {
	return strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
}
