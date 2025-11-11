package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand"
)

var leftNames = []string{
	"brave", "calm", "eager", "gentle", "kind", "proud", "quiet", "sharp", "wise", "zealous",
	"bold", "clever", "curious", "daring", "focused", "graceful", "humble", "jolly", "lively", "merry",
	"patient", "quick", "resourceful", "steady", "thoughtful", "trusty", "vivid", "witty", "zesty", "cheerful",
}

var rightNames = []string{
	"builder", "creator", "dreamer", "explorer", "friend", "helper", "leader", "maker", "seeker", "thinker",
	"artisan", "pathfinder", "innovator", "navigator", "observer", "planner", "storyteller", "strategist", "tinkerer", "visionary",
	"adventurer", "collaborator", "discoverer", "engineer", "fixer", "pioneer", "scholar", "traveler", "watcher", "worker",
}

func RandName() string {
	left := leftNames[mrand.Intn(len(leftNames))]
	right := rightNames[mrand.Intn(len(rightNames))]

	var b [8]byte // 64 bits of randomness
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	suffix := hex.EncodeToString(b[:]) // 16 hex chars
	return fmt.Sprintf("%s_%s_%s", left, right, suffix)
}

func RandString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[mrand.Intn(len(charset))]
	}
	return string(b)
}
