package server

import (
	"bytes"
	"compress/flate"
	"encoding/json"
	"testing"

	"cookiekill/internal/game"
)

// This measures the actual delta encoder over ten simulated seconds, excluding
// the initial snapshot. The smaller comparison frame removes only the animals
// field, so its difference includes animals' incremental compression cost.
func TestAnimalSteadyStateBandwidth(t *testing.T) {
	w := game.New(nil)
	w.Join("animal_bandwidth", "Animal bandwidth test")
	encoders := []stateEncoder{{delta: true}, {delta: true, animalPaths: true}}
	first := w.Snapshot("animal_bandwidth")
	for i := range encoders {
		encoders[i].commit(first)
	}
	var compressed bytes.Buffer
	writer, err := flate.NewWriter(&compressed, flate.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	deflatedSize := func(data []byte) int {
		compressed.Reset()
		writer.Reset(&compressed)
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := writer.Flush(); err != nil {
			t.Fatal(err)
		}
		// permessage-deflate omits the four-byte sync-flush tail.
		return compressed.Len() - 4
	}
	type traffic struct {
		raw, compressed, animalRaw, animalCompressed, animalFrames int
	}
	var captures [2]traffic
	const frames = 100
	for n := 0; n < frames; n++ {
		w.Tick(.05)
		w.Tick(.05)
		snapshot := w.SnapshotUpdate("animal_bandwidth")
		// Real wall-clock seconds are irrelevant to this simulated capture.
		snapshot.ServerTime = 2_000_000_000 + int64(n/10)
		for i := range encoders {
			data := encodeDeltaTest(t, &encoders[i], snapshot)
			encoders[i].commit(snapshot)
			var fields map[string]json.RawMessage
			decodeDeltaTest(t, data, &fields)
			capture := &captures[i]
			if _, exists := fields["animals"]; exists {
				capture.animalFrames++
			}
			delete(fields, "animals")
			withoutAnimals := marshalDeltaTest(t, fields)
			compressedSize := deflatedSize(data)
			capture.raw += len(data)
			capture.compressed += compressedSize
			capture.animalRaw += len(data) - len(withoutAnimals)
			capture.animalCompressed += compressedSize - deflatedSize(withoutAnimals)
		}
	}
	for i, capture := range captures {
		t.Logf("animal paths=%t; idle mainland, %d animals, %d updates / 10s: raw %.0f B/s (%.1f kbit/s), deflated %.0f B/s (%.1f kbit/s); animals %.0f B/s raw (%.1f%%), %.0f B/s deflated incremental (%.1f%%), %d/%d frames", encoders[i].animalPaths, len(first.Animals), frames,
			float64(capture.raw)/10, float64(capture.raw)*8/10_000, float64(capture.compressed)/10, float64(capture.compressed)*8/10_000,
			float64(capture.animalRaw)/10, 100*float64(capture.animalRaw)/float64(capture.raw), float64(capture.animalCompressed)/10, 100*float64(capture.animalCompressed)/float64(capture.compressed), capture.animalFrames, frames)
	}
	legacy, paths := captures[0], captures[1]
	// Allow initial route announcements and occasional lifecycle changes while
	// catching accidental per-tick coordinates, age, or hatch countdown traffic.
	if paths.animalRaw > legacy.animalRaw/10 || paths.animalFrames > frames/10 {
		t.Fatalf("projected routes did not remove at least 90%% of routine animal traffic: legacy=%+v paths=%+v", legacy, paths)
	}
	if paths.compressed > legacy.compressed/2 {
		t.Fatalf("projected routes did not materially reduce overall compressed traffic: legacy=%+v paths=%+v", legacy, paths)
	}
}
