package main

// Where each item comes from, read from AllTheThings' WoW Forever database (MIT). Its .lua files are
// run with every helper stubbed to record the call, and each item gets a category from the calls
// around it (instance, quest, vendor, reputation, profession, PvP, world drop).

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type sourceItem struct {
	ID       int      `json:"id"`
	From     string   `json:"from,omitempty"`     // boss, vendor, quest or mob
	Side     string   `json:"side,omitempty"`     // horde or alliance
	Standing string   `json:"standing,omitempty"` // reputation needed
	Classes  []string `json:"classes,omitempty"`
	// The level a quest can be done from; its rewards often need no level of their own.
	Level int `json:"level,omitempty"`
}

type sourceGroup struct {
	Category string       `json:"category"` // dungeon, raid, crafted, reputation, pvp, vendor, quest, world
	Name     string       `json:"name"`
	Zone     string       `json:"zone,omitempty"`
	Level    int          `json:"level,omitempty"` // an instance's entry level
	New      bool         `json:"new,omitempty"`   // added in Forever
	Items    []sourceItem `json:"items"`
}

// One recorded call, e.g. i(123), n(VENDORS, {...}), q(1, {...}).
type attNode struct {
	fn     string
	id     int    // numeric argument
	sym    string // symbolic argument, e.g. QUESTS or ROOTS.Instances
	name   string // the source comment after the call
	fields *lua.LTable
	kids   []*attNode
}

var (
	// X(arg, {	-- Name  →  X(arg, { __name = "Name",
	tableComment = regexp.MustCompile(`\b([a-zA-Z_]\w*)\(([\w.]+), \{\s*--\s*(.+)$`)
	// X(123),	-- Name  →  X(123, { __name = "Name" }),
	leafComment = regexp.MustCompile(`\b([a-zA-Z_]\w*)\((\d+)\)(,?)\s*--\s*(.+)$`)
)

// Moves each call's trailing comment, which is where ATT keeps names, into the call's table.
func nameComments(src string) string {
	quote := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if m := tableComment.FindStringSubmatchIndex(line); m != nil && line[m[2]:m[3]] != "i" {
			brace := m[0] + strings.Index(line[m[0]:], "{")
			lines[i] = line[:brace+1] + ` __name = "` + quote.Replace(strings.TrimSpace(line[m[6]:m[7]])) + `",`
		} else if m := leafComment.FindStringSubmatchIndex(line); m != nil && line[m[2]:m[3]] != "i" {
			lines[i] = line[:m[0]] + line[m[2]:m[3]] + "(" + line[m[4]:m[5]] + `, { __name = "` +
				quote.Replace(strings.TrimSpace(line[m[8]:m[9]])) + `" })` + line[m[6]:m[7]]
		}
	}
	return strings.Join(lines, "\n")
}

var rootFns = map[string]bool{"root": true, "maproot": true, "instanceroot": true, "profession": true, "battleground": true}

// Runs every .lua file under dir and returns the root calls. Unknown globals resolve to symbols that
// record a call when called, so ATT's helpers need no definitions.
func runATT(dir string, mapsFile string) ([]*attNode, error) {
	L := lua.NewState()
	defer L.Close()
	var roots []*lua.LTable
	symMeta, callMeta := L.NewTable(), L.NewTable()
	newSym := func(name string) *lua.LTable {
		t := L.NewTable()
		t.RawSetString("__sym", lua.LString(name))
		L.SetMetatable(t, symMeta)
		return t
	}
	L.SetField(symMeta, "__index", L.NewFunction(func(L *lua.LState) int {
		L.Push(newSym(L.CheckTable(1).RawGetString("__sym").String() + "." + L.Get(2).String()))
		return 1
	}))
	L.SetField(symMeta, "__call", L.NewFunction(func(L *lua.LState) int {
		fn := L.CheckTable(1).RawGetString("__sym").String()
		call := L.NewTable()
		call.RawSetString("__fn", lua.LString(fn))
		args := L.NewTable()
		for i := 2; i <= L.GetTop(); i++ {
			args.RawSetInt(i-1, L.Get(i))
		}
		call.RawSetString("__args", args)
		L.SetMetatable(call, callMeta)
		if rootFns[fn] {
			roots = append(roots, call)
		}
		L.Push(call)
		return 1
	}))
	// Arithmetic and concatenation on a symbol or call give a symbol; the result is never an item source.
	for _, op := range []string{"__concat", "__add", "__sub", "__mul", "__div", "__mod", "__unm", "__pow"} {
		f := L.NewFunction(func(L *lua.LState) int {
			L.Push(newSym("?"))
			return 1
		})
		L.SetField(symMeta, op, f)
		L.SetField(callMeta, op, f)
	}
	globalMeta := L.NewTable()
	L.SetField(globalMeta, "__index", L.NewFunction(func(L *lua.LState) int {
		L.Push(newSym(L.CheckString(2)))
		return 1
	}))
	L.SetGlobal("MAP", L.NewTable())
	L.SetMetatable(L.G.Global, globalMeta)
	if err := L.DoFile(mapsFile); err != nil { // MAP.X → map ids, named through UiMap
		return nil, fmt.Errorf("%s: %w", mapsFile, err)
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && path != dir && (strings.HasPrefix(d.Name(), ".") || d.Name() == "profession db") {
			return filepath.SkipDir // .config, .nyi, .hqt; profession db maps recipes to spells
		}
		if !d.IsDir() && strings.HasSuffix(path, ".lua") && !strings.HasPrefix(d.Name(), ".") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		if err := L.DoString(nameComments(string(src))); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
	}
	seen := map[*lua.LTable]*attNode{}
	inner := map[*attNode]bool{} // e.g. maproot(...) inside root(ROOTS.Zones, ...)
	var mark func(n *attNode)
	mark = func(n *attNode) {
		for _, k := range n.kids {
			if !inner[k] {
				inner[k] = true
				mark(k)
			}
		}
	}
	all := make([]*attNode, 0, len(roots))
	for _, r := range roots {
		n := toNode(r, seen)
		all = append(all, n)
		mark(n)
	}
	nodes := all[:0]
	for _, n := range all {
		if !inner[n] {
			nodes = append(nodes, n)
		}
	}
	return nodes, nil
}

func symName(v lua.LValue) string {
	if t, ok := v.(*lua.LTable); ok {
		if s, ok := t.RawGetString("__sym").(lua.LString); ok {
			return string(s)
		}
	}
	return ""
}

func isCall(v lua.LValue) bool {
	t, ok := v.(*lua.LTable)
	return ok && t.RawGetString("__fn") != lua.LNil
}

// A plain table's children: its array part and its groups.
func tableKids(t *lua.LTable, seen map[*lua.LTable]*attNode) []*attNode {
	var kids []*attNode
	add := func(v lua.LValue) {
		sub, ok := v.(*lua.LTable)
		switch {
		case isCall(v):
			kids = append(kids, toNode(sub, seen))
		case ok && sub.RawGetString("allianceQuestData") != lua.LNil:
			// A quest with a version per side: { allianceQuestData = q(...), hordeQuestData = q(...), groups = ... }
			a := toNode(sub.RawGetString("allianceQuestData").(*lua.LTable), seen)
			kids = append(kids, &attNode{fn: "q", name: strings.TrimSuffix(a.name, " (A)"), fields: sub, kids: tableKids(sub, seen)})
		case ok && symName(sub) == "":
			kids = append(kids, tableKids(sub, seen)...)
		}
	}
	for i := 1; i <= t.Len(); i++ {
		add(t.RawGetInt(i))
	}
	if g := t.RawGetString("groups"); g != lua.LNil {
		add(g)
	}
	return kids
}

func toNode(call *lua.LTable, seen map[*lua.LTable]*attNode) *attNode {
	if n := seen[call]; n != nil {
		return n
	}
	n := &attNode{fn: call.RawGetString("__fn").String()}
	seen[call] = n
	args := call.RawGetString("__args").(*lua.LTable)
	for i := 1; i <= args.MaxN(); i++ {
		switch v := args.RawGetInt(i).(type) {
		case lua.LNumber:
			n.id = int(v) // the last number wins: maproot(MAP.KALIMDOR, MAP.THE_BARRENS, ...)
		case lua.LString:
			if n.sym == "" {
				n.sym = string(v)
			}
		case *lua.LTable:
			switch {
			case symName(v) != "":
				if n.sym == "" {
					n.sym = symName(v)
				}
			case isCall(v):
				kid := toNode(v, seen)
				if kid.fn == "createHeader" && n.name == "" {
					n.name = headerName(kid)
				} else {
					n.kids = append(n.kids, kid)
				}
			default:
				if n.fields == nil {
					n.fields = v
				}
				if name, ok := v.RawGetString("__name").(lua.LString); ok && n.name == "" {
					n.name = string(name)
				}
				n.kids = append(n.kids, tableKids(v, seen)...)
			}
		}
	}
	return n
}

func headerName(h *attNode) string {
	if h.fields == nil {
		return h.name
	}
	for _, k := range []string{"readable", "text"} {
		switch v := h.fields.RawGetString(k).(type) {
		case lua.LString:
			return string(v)
		case *lua.LTable:
			if en, ok := v.RawGetString("en").(lua.LString); ok {
				return string(en)
			}
		}
	}
	return h.name
}

func (n *attNode) field(k string) lua.LValue {
	if n.fields == nil {
		return lua.LNil
	}
	return n.fields.RawGetString(k)
}

var smallWords = map[string]bool{"of": true, "the": true, "and": true}

// FACTION_BROOD_OF_NOZDORMU → Brood of Nozdormu
func humanize(sym string) string {
	sym = sym[strings.LastIndex(sym, ".")+1:]
	sym = strings.TrimPrefix(sym, "FACTION_")
	words := strings.Split(strings.ToLower(sym), "_")
	for i, w := range words {
		if w != "" && (i == 0 || !smallWords[w]) {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

// Calls whose items are not obtainable or not gear sources.
var skipFns = map[string]bool{"ach": true, "achWithRep": true, "r": true, "recipe": true, "objective": true, "header": true, "title": true, "fp": true}
var skipHeaders = map[string]bool{"ACHIEVEMENTS": true, "FLIGHT_PATHS": true, "HIDDEN_QUESTS": true}
var skipRoots = map[string]bool{
	"ROOTS.HiddenQuestTriggers": true, "ROOTS.NeverImplemented": true, "ROOTS.Unsorted": true, "ROOTS.Secrets": true,
	"ROOTS.Promotions": true, "ROOTS.Holidays": true, "ROOTS.WorldEvents": true, "ROOTS.ExpansionFeatures": true,
}

// Labels for items an instance or zone lists outside a boss.
var headerLabels = map[string]string{
	"ZONE_DROPS": "Trash", "COMMON_BOSS_DROPS": "Shared boss drops", "TREASURES": "Treasures", "RARES": "Rares",
	"SPECIAL": "Special", "REWARDS": "Rewards",
}

type srcCtx struct {
	root, zone, from, profession, pvp string
	instance                          *sourceGroup
	quest, vendor                     bool
	side, faction, standing           string
	classes                           []string
	level                             int
}

type sourceBuilder struct {
	gear         map[int]bool
	mapNames     map[int]string
	factionNames map[int]string
	groups       []*sourceGroup
	byKey        map[string]*sourceGroup
	seen         map[string]bool
}

func (b *sourceBuilder) group(category, name, zone string) *sourceGroup {
	key := category + "\x00" + name
	if g := b.byKey[key]; g != nil {
		return g
	}
	g := &sourceGroup{Category: category, Name: name, Zone: zone, Items: []sourceItem{}}
	b.byKey[key] = g
	b.groups = append(b.groups, g)
	return g
}

func sideOf(v lua.LValue) string {
	s := strings.ToUpper(symName(v))
	switch {
	case strings.Contains(s, "HORDE"):
		return "horde"
	case strings.Contains(s, "ALLIANCE"):
		return "alliance"
	}
	return ""
}

func (b *sourceBuilder) walk(n *attNode, c srcCtx) {
	if skipFns[n.fn] || n.field("u") != lua.LNil || n.field("isBreadcrumb") == lua.LTrue && n.fn != "q" {
		return
	}
	if n.fn == "n" && skipHeaders[n.sym] {
		return
	}
	if s := sideOf(n.field("races")); s != "" {
		c.side = s
	}
	if cls, ok := n.field("classes").(*lua.LTable); ok {
		c.classes = nil
		for i := 1; i <= cls.Len(); i++ {
			if s := symName(cls.RawGetInt(i)); s != "" {
				c.classes = append(c.classes, strings.ToLower(s))
			}
		}
	}
	if rep, ok := n.field("minReputation").(*lua.LTable); ok {
		c.faction = humanize(symName(rep.RawGetInt(1)))
		if id, ok := rep.RawGetInt(1).(lua.LNumber); ok {
			c.faction = b.factionNames[int(id)]
		}
		c.standing = strings.ToLower(symName(rep.RawGetInt(2)))
	}
	switch n.fn {
	case "root":
		if skipRoots[n.sym] {
			return
		}
		c.root = n.sym
	case "battleground":
		c.root, c.zone = "battleground", b.mapNames[n.id]
	case "maproot", "m":
		if name := b.mapNames[n.id]; name != "" {
			c.zone = name
		}
	case "instanceroot", "inst":
		cat := "dungeon"
		if n.field("isRaid") == lua.LTrue {
			cat = "raid"
		}
		g := b.group(cat, n.name, "")
		if lvl, ok := n.field("lvl").(lua.LNumber); ok {
			g.Level = int(lvl)
		}
		g.New = strings.Contains(luaString(n.field("timeline")), "ADDED_1_60_1")
		c.instance, c.zone, c.from = g, n.name, ""
	case "q":
		c.quest, c.from = true, n.name
		// lvl is a number or a { min, max } range.
		switch lvl := n.field("lvl").(type) {
		case lua.LNumber:
			c.level = int(lvl)
		case *lua.LTable:
			if lo, ok := lvl.RawGetInt(1).(lua.LNumber); ok {
				c.level = int(lo)
			}
		}
	case "prof", "profession":
		if c.profession == "" {
			c.profession = humanize(n.sym)
		}
	case "e":
		c.from = n.name
	case "o", "container":
		if !c.quest {
			c.from = n.name // a chest, e.g. the Gordok Tribute Chest
		}
	case "n":
		switch {
		case n.sym == "VENDORS":
			c.vendor = true
		case n.sym == "PREMIER_PVP_GEAR":
			c.pvp = "Premier PvP gear"
		case n.id > 0 && !c.quest:
			c.from = n.name
		case headerLabels[n.sym] != "":
			c.from = headerLabels[n.sym]
		case n.sym == "FACTION_HEADER_HORDE":
			c.side = "horde"
		case n.sym == "FACTION_HEADER_ALLIANCE":
			c.side = "alliance"
		case n.sym == "" && n.name != "" && c.from == "":
			c.from = n.name // a wing, e.g. Dire Maul's Warpwood Quarter
		}
	case "i":
		b.add(n.id, c)
	}
	for _, k := range n.kids {
		b.walk(k, c)
	}
}

func luaString(v lua.LValue) string {
	var sb strings.Builder
	var walk func(lua.LValue)
	walk = func(v lua.LValue) {
		if s := symName(v); s != "" {
			sb.WriteString(s + " ")
		} else if t, ok := v.(*lua.LTable); ok {
			t.ForEach(func(_, x lua.LValue) { walk(x) })
		} else {
			sb.WriteString(v.String() + " ")
		}
	}
	walk(v)
	return sb.String()
}

func (b *sourceBuilder) add(id int, c srcCtx) {
	if !b.gear[id] {
		return
	}
	var g *sourceGroup
	switch {
	case c.faction != "":
		g = b.group("reputation", c.faction, "")
	case c.quest:
		name := c.zone
		if name == "" {
			name = "Other quests"
		}
		g = b.group("quest", name, "")
	case c.vendor && c.root == "battleground":
		g = b.group("pvp", c.from, c.zone)
	case c.vendor:
		g = b.group("vendor", c.from, c.zone)
	case c.root == "ROOTS.Craftables" && c.profession != "":
		g = b.group("crafted", c.profession, "")
	case c.root == "ROOTS.PVP":
		name := c.pvp
		if name == "" {
			name = "PvP rank rewards"
		}
		g = b.group("pvp", name, "")
	case c.instance != nil:
		g = c.instance
	case c.root == "ROOTS.WorldDrops":
		g = b.group("world", "World drops", "")
	case c.zone != "":
		g = b.group("world", c.zone, "")
	default:
		return
	}
	it := sourceItem{ID: id, Side: c.side, Standing: c.standing, Classes: c.classes}
	if c.quest {
		it.Level = c.level
	}
	if g.Category != "vendor" && g.Category != "crafted" && g.Category != "pvp" {
		it.From = c.from
	}
	key := fmt.Sprintf("%s\x00%s\x00%d\x00%s\x00%s", g.Category, g.Name, id, it.From, it.Side)
	if b.seen[key] {
		return
	}
	b.seen[key] = true
	g.Items = append(g.Items, it)
}

// Builds the source groups from an ATT checkout's Forever database; only the items in gear are kept.
func buildSources(attDir string, gear map[int]bool, mapNames, factionNames map[int]string) ([]*sourceGroup, error) {
	db := filepath.Join(attDir, ".contrib/.db/forever")
	roots, err := runATT(db, filepath.Join(db, ".config/constants/maps.lua"))
	if err != nil {
		return nil, err
	}
	b := &sourceBuilder{gear: gear, mapNames: mapNames, factionNames: factionNames, byKey: map[string]*sourceGroup{}, seen: map[string]bool{}}
	for _, r := range roots {
		b.walk(r, srcCtx{})
	}
	// The city honor vendors sell what the PvP section lists by rank; keep it there only.
	pvp := map[int]bool{}
	for _, g := range b.groups {
		if g.Category == "pvp" {
			for _, it := range g.Items {
				pvp[it.ID] = true
			}
		}
	}
	groups := b.groups[:0]
	for _, g := range b.groups {
		if g.Category == "vendor" {
			items := g.Items[:0]
			for _, it := range g.Items {
				if !pvp[it.ID] {
					items = append(items, it)
				}
			}
			if g.Items = items; len(items) == 0 {
				continue
			}
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// ID → Name_lang of a client table, e.g. UiMap or Faction.
func loadNames(path string) (map[int]string, error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	out := make(map[int]string, len(rows))
	for _, r := range rows {
		out[atoi(r["ID"])] = r["Name_lang"]
	}
	return out, nil
}
