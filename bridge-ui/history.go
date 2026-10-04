package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Activity is the record of one question, shown in the app's activity feed.
// It is kept on disk so questions handled by the background hub show up too.
type Activity struct {
	ID       string    `json:"id"`
	Question string    `json:"question"`
	Options  []string  `json:"options"`
	Client   string    `json:"client"`
	Project  string    `json:"project"`
	State    string    `json:"state"` // waiting | answered | expired | stopped
	Answer   string    `json:"answer,omitempty"`
	Created  time.Time `json:"created"`
	Closed   time.Time `json:"closed,omitempty"`
}

const maxActivity = 100

var activityMu sync.Mutex

func activityPath() string { return activityFile(dataDir()) }

func activityFile(dir string) string { return filepath.Join(dir, "activity.json") }

func ReadActivity() []Activity {
	activityMu.Lock()
	defer activityMu.Unlock()
	return readActivityLocked()
}

func readActivityLocked() []Activity {
	var items []Activity
	if b, err := os.ReadFile(activityPath()); err == nil {
		json.Unmarshal(b, &items)
	}
	if items == nil {
		items = []Activity{}
	}
	return items
}

// recordActivity inserts or updates one item (newest first) in dir and returns it.
func recordActivity(dir string, q *pendingQuestion) Activity {
	a := Activity{ID: q.ID, Question: q.Question, Options: q.Options, Client: q.Client, Project: q.Project,
		State: q.state, Answer: q.answer, Created: q.Created}
	if q.state != stateWaiting {
		a.Closed = time.Now()
	}
	activityMu.Lock()
	defer activityMu.Unlock()
	var items []Activity
	if b, err := os.ReadFile(activityFile(dir)); err == nil {
		json.Unmarshal(b, &items)
	}
	replaced := false
	for i := range items {
		if items[i].ID == a.ID {
			items[i] = a
			replaced = true
		}
	}
	if !replaced {
		items = append([]Activity{a}, items...)
	}
	// A hub that stopped (or crashed) can't answer its old questions any more.
	for i := range items {
		if items[i].State == stateWaiting && time.Since(items[i].Created) > 24*time.Hour {
			items[i].State = stateExpired
		}
	}
	if len(items) > maxActivity {
		items = items[:maxActivity]
	}
	if b, err := json.Marshal(items); err == nil {
		os.WriteFile(activityFile(dir), b, 0600)
	}
	return a
}

func ClearActivity() {
	activityMu.Lock()
	defer activityMu.Unlock()
	os.Remove(activityPath())
}
