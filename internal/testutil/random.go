package testutil

import (
	"math/rand"
	"strconv"
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
	left := leftNames[rand.Intn(len(leftNames))]
	right := rightNames[rand.Intn(len(rightNames))]
	return left + "_" + right + "_" + strconv.Itoa(rand.Intn(50))
}

func RandString(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
