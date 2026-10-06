package server

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"cookiekill/internal/game"
)

// stateEncoder belongs exclusively to a connection's writer. Its baseline is
// advanced only after a successful write, never when a snapshot is queued.
type stateEncoder struct {
	delta    bool
	previous *game.Snapshot
	sequence uint64
}

func (e *stateEncoder) encode(snapshot game.Snapshot) ([]byte, error) {
	if !e.delta || e.previous == nil {
		if e.previous != nil {
			snapshot.Layout = nil
		}
		var sequence uint64
		if e.delta {
			sequence = e.sequence + 1
		}
		return json.Marshal(struct {
			Type string `json:"type"`
			Seq  uint64 `json:"seq,omitempty"`
			game.Snapshot
		}{"snapshot", sequence, snapshot})
	}

	previous := e.previous
	frame := map[string]any{
		"type": "delta", "seq": e.sequence + 1, "base": e.sequence,
		"time": snapshot.Time, "serverTime": snapshot.ServerTime,
	}
	if patch := changedFields(&previous.Me, &snapshot.Me); len(patch) != 0 {
		frame["me"] = patch
	}
	addEntityChanges(frame, "players", previous.Players, snapshot.Players, "id", func(p game.PlayerView) string { return p.ID })
	addEntityChanges(frame, "nodes", previous.Nodes, snapshot.Nodes, "id", func(n game.Node) string { return n.ID })
	addEntityChanges(frame, "animals", previous.Animals, snapshot.Animals, "id", func(a game.Animal) string { return a.ID })
	addEntityChanges(frame, "projectiles", previous.Projectiles, snapshot.Projectiles, "id", func(p game.Projectile) string { return p.ID })
	addEntityChanges(frame, "homes", previous.Homes, snapshot.Homes, "owner", func(h game.Home) string { return h.Owner })
	if !reflect.DeepEqual(previous.Events, snapshot.Events) {
		frame["events"] = snapshot.Events
	}
	if !reflect.DeepEqual(previous.Recipes, snapshot.Recipes) {
		frame["recipes"] = snapshot.Recipes
	}
	return json.Marshal(frame)
}

func (e *stateEncoder) commit(snapshot game.Snapshot) {
	// Layout is immutable and only needed in the initial frame.
	snapshot.Layout = nil
	e.previous = &snapshot
	e.sequence++
}

type entityChanges struct {
	Upsert []any    `json:"upsert,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

func addEntityChanges[T any](frame map[string]any, name string, previous, current []T, key string, id func(T) string) {
	before := make(map[string]int, len(previous))
	for i, entity := range previous {
		before[id(entity)] = i
	}
	var changes entityChanges
	for i, entity := range current {
		entityID := id(entity)
		previousIndex, exists := before[entityID]
		if !exists {
			changes.Upsert = append(changes.Upsert, entity)
		} else if patch := changedFields(&previous[previousIndex], &current[i]); len(patch) != 0 {
			patch[key] = entityID
			changes.Upsert = append(changes.Upsert, patch)
		}
		delete(before, entityID)
	}
	// Preserve source order for deterministic output, including removals.
	for _, entity := range previous {
		if _, exists := before[id(entity)]; exists {
			changes.Remove = append(changes.Remove, id(entity))
		}
	}
	if len(changes.Upsert) != 0 || len(changes.Remove) != 0 {
		frame[name] = changes
	}
}

// Patches replace each changed JSON field as a whole. In particular, maps and
// arrays are replaced, not recursively merged, so deleted inventory/buff keys
// disappear. Zero values and nil pointers must be sent even for omitempty fields.
func changedFields(previous, current any) map[string]any {
	old, value := reflect.ValueOf(previous), reflect.ValueOf(current)
	if value.Kind() == reflect.Pointer {
		old, value = old.Elem(), value.Elem()
	}
	var patch map[string]any
	for _, field := range jsonFields(value.Type()) {
		before, after := old.FieldByIndex(field.index), value.FieldByIndex(field.index)
		if field.comparable {
			if before.Equal(after) {
				continue
			}
		} else if reflect.DeepEqual(before.Interface(), after.Interface()) {
			continue
		}
		if patch == nil {
			patch = make(map[string]any)
		}
		patch[field.name] = after.Interface()
	}
	return patch
}

type jsonField struct {
	name       string
	index      []int
	comparable bool
}

var jsonFieldCache sync.Map // map[reflect.Type][]jsonField

// Cache exported JSON fields once per type. Comparing primitive fields directly
// avoids boxing whole entities and reparsing struct tags on every update.
func jsonFields(typ reflect.Type) []jsonField {
	if cached, ok := jsonFieldCache.Load(typ); ok {
		return cached.([]jsonField)
	}
	var fields []jsonField
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "-" {
			continue
		}
		if field.Anonymous && name == "" && field.Type.Kind() == reflect.Struct {
			for _, embedded := range jsonFields(field.Type) {
				embedded.index = append([]int{i}, embedded.index...)
				fields = append(fields, embedded)
			}
			continue
		}
		if name == "" {
			name = field.Name
		}
		// Pointers describe JSON objects; compare their contents, not addresses.
		fields = append(fields, jsonField{name: name, index: field.Index, comparable: field.Type.Comparable() && field.Type.Kind() != reflect.Pointer})
	}
	cached, _ := jsonFieldCache.LoadOrStore(typ, fields)
	return cached.([]jsonField)
}
