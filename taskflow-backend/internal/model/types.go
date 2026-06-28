package model

type Tone string

const (
	ToneRose  Tone = "rose"
	TonePeach Tone = "peach"
	ToneAmber Tone = "amber"
	ToneMint  Tone = "mint"
	ToneSky   Tone = "sky"
	ToneLilac Tone = "lilac"
)

type Member struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Initial string `json:"initial"`
	Tone    Tone   `json:"tone"`
	Avatar  string `json:"avatar"`
}

type ChecklistItem struct {
	ID   string `json:"id"`
	Text string `json:"text"`
	Done bool   `json:"done"`
}

type Task struct {
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Cat       string          `json:"cat"`
	Assignee  *string         `json:"assignee"`
	Done      bool            `json:"done"`
	DoneBy    *string         `json:"doneBy,omitempty"`
	DoneAt    *string         `json:"doneAt,omitempty"`
	Due       string          `json:"due"`
	Late      bool            `json:"late"`
	Recurring bool            `json:"recurring"`
	Repeat    *string         `json:"repeat,omitempty"`
	WeekDays  []int           `json:"weekDays,omitempty"`
	MonthDay  *int            `json:"monthDay,omitempty"`
	Time      *string         `json:"time,omitempty"`
	FreqText  *string         `json:"freqText,omitempty"`
	Checklist []ChecklistItem `json:"checklist,omitempty"`
}

type HistoryEntry struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Cat    string `json:"cat"`
	By     string `json:"by"`
	At     string `json:"at"`
	TaskID string `json:"taskId,omitempty"`
}

type Vaccine struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Date  string `json:"date"`
	Next  string `json:"next"`
	Done  bool   `json:"done"`
}

type Treatment struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Every string `json:"every"`
	Last  string `json:"last"`
	Next  string `json:"next"`
}

type VetAppt struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Date   string `json:"date"`
	Time   string `json:"time"`
	Clinic string `json:"clinic"`
}

type WeightPoint struct {
	Date string  `json:"date"`
	W    float64 `json:"w"`
}

type Pet struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Species    string        `json:"species"`
	Emoji      string        `json:"emoji"`
	Tone       Tone          `json:"tone"`
	Breed      string        `json:"breed"`
	Age        string        `json:"age"`
	Weight     float64       `json:"weight"`
	Sex        string        `json:"sex"`
	Vaccines   []Vaccine     `json:"vaccines"`
	Vet        []VetAppt     `json:"vet"`
	Treatments []Treatment   `json:"treatments"`
	WeightLog  []WeightPoint `json:"weightLog"`
}

type Settings struct {
	Theme         string `json:"theme"`
	Accent        string `json:"accent"`
	NotifTasks    bool   `json:"notifTasks"`
	NotifPets     bool   `json:"notifPets"`
	NotifAgenda   bool   `json:"notifAgenda"`
	Sounds        bool   `json:"sounds"`
	CurrentMember string `json:"currentMember"`
	HouseholdName string `json:"householdName"`
	Onboarded     bool   `json:"onboarded"`
}

type GoogleEvent struct {
	ID    string `json:"id"`
	Date  string `json:"date"`
	Time  string `json:"time"`
	Title string `json:"title"`
}

type AppData struct {
	Version      int            `json:"version"`
	Members      []Member       `json:"members"`
	Pets         []Pet          `json:"pets"`
	Tasks        []Task         `json:"tasks"`
	History      []HistoryEntry `json:"history"`
	GoogleEvents []GoogleEvent  `json:"googleEvents"`
	Settings     Settings       `json:"settings"`
}

// MQTTSnapshot est le payload publié sur foyer/snapshot
type MQTTSnapshot struct {
	Seq     int            `json:"seq"`
	Tasks   []Task         `json:"tasks"`
	Members []Member       `json:"members"`
	History []HistoryEntry `json:"history"`
}
