package scene

// Aim describes the pointing direction of a microphone.
// Azimuth 0° = north (+Y), 90° = east (+X), increasing clockwise when viewed from above.
// Elevation 0° = horizontal, positive = upward (+Z).
// Both angles are in degrees and are converted to a unit vector at compute time.
type Aim struct {
	Azimuth   float64 `json:"azimuth"`
	Elevation float64 `json:"elevation"`
}

// Source is a sound-emitting point in the room.
// Sources are assumed to be omnidirectional — they radiate equally in all directions.
// Directivity of real sources (instrument cabinets, vocals) is not modelled.
// All coordinates are in metres.
type Source struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	Z  float64 `json:"z"`
}

// Mic is a microphone with a position, aim direction, and polar pattern.
// The polar pattern applies a directional gain factor to every path that
// arrives at the microphone. All coordinates are in metres.
type Mic struct {
	ID      string  `json:"id"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
	Aim     Aim     `json:"aim"`
	Pattern string  `json:"pattern"`
}

// Gobo is a vertical acoustic barrier panel.
// Gobos are modelled as axis-aligned or arbitrary-orientation rectangles
// standing perpendicular to the floor. The footprint is defined by two
// floor-plan coordinates (X1,Y1)→(X2,Y2); Height is measured from the floor.
// All coordinates and dimensions are in metres.
type Gobo struct {
	ID       string  `json:"id"`
	X1       float64 `json:"x1"`
	Y1       float64 `json:"y1"`
	X2       float64 `json:"x2"`
	Y2       float64 `json:"y2"`
	Height   float64 `json:"height"`
	Material string  `json:"material"`
}

// Surfaces names the material on each of the six rectangular room faces.
// Each material name must appear in KnownMaterials; the scene validator
// rejects unknown names before the acoustic engine runs.
type Surfaces struct {
	Floor   string `json:"floor"`
	Ceiling string `json:"ceiling"`
	North   string `json:"north"`
	South   string `json:"south"`
	East    string `json:"east"`
	West    string `json:"west"`
}

// Room describes the rectangular room geometry.
// The room is a right-rectangular box. The coordinate system places the
// south-west floor corner at the origin: X increases east, Y increases north,
// Z increases upward. Width = east–west extent, Depth = north–south extent,
// Height = floor–ceiling extent. All dimensions are in metres and must be positive.
type Room struct {
	Width    float64  `json:"width"`
	Depth    float64  `json:"depth"`
	Height   float64  `json:"height"`
	Surfaces Surfaces `json:"surfaces"`
}

// Scene is the top-level scene description parsed from a JSON file.
// It holds all the information the acoustic engine needs: the room geometry,
// surface materials, source positions, microphone positions/patterns, and
// any gobo panels. The Version field is checked to guard against future
// format changes.
type Scene struct {
	Version    int      `json:"version"`
	SampleRate int      `json:"sample_rate"`
	Room       Room     `json:"room"`
	Sources    []Source `json:"sources"`
	Mics       []Mic    `json:"mics"`
	Gobos      []Gobo   `json:"gobos"`
}
