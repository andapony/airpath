package scene

// Aim describes the pointing direction of a microphone.
type Aim struct {
	Azimuth   float64 `json:"azimuth"`
	Elevation float64 `json:"elevation"`
}

// Source is a sound-emitting point in the room.
type Source struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	Z  float64 `json:"z"`
}

// Mic is a microphone with a position, aim direction, and polar pattern.
type Mic struct {
	ID      string  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
	Aim     Aim     `json:"aim"`
	Pattern string  `json:"pattern"`
}

// Gobo is a vertical acoustic barrier panel.
type Gobo struct {
	ID       string  `json:"id"`
	X1       float64 `json:"x1"`
	Y1       float64 `json:"y1"`
	X2       float64 `json:"x2"`
	Y2       float64 `json:"y2"`
	Height   float64 `json:"height"`
	Material string  `json:"material"`
}

// Surfaces names the material on each of the six room faces.
type Surfaces struct {
	Floor   string `json:"floor"`
	Ceiling string `json:"ceiling"`
	North   string `json:"north"`
	South   string `json:"south"`
	East    string `json:"east"`
	West    string `json:"west"`
}

// Room describes the rectangular room geometry.
type Room struct {
	Width    float64  `json:"width"`
	Depth    float64  `json:"depth"`
	Height   float64  `json:"height"`
	Surfaces Surfaces `json:"surfaces"`
}

// Scene is the top-level scene description parsed from JSON.
type Scene struct {
	Version    int      `json:"version"`
	SampleRate int      `json:"sample_rate"`
	Room       Room     `json:"room"`
	Sources    []Source `json:"sources"`
	Mics       []Mic    `json:"mics"`
	Gobos      []Gobo   `json:"gobos"`
}
