package scene

import (
	_ "embed"
	"encoding/json"
)

//go:embed defaults.json
var defaultsJSON []byte

// Bands lists the 7 octave-band center frequencies in Hz used throughout
// the acoustic engine for per-band absorption lookups.
var Bands = [7]int{125, 250, 500, 1000, 2000, 4000, 8000}

// Absorption holds per-octave-band absorption coefficients for a material,
// indexed to match Bands: [125, 250, 500, 1000, 2000, 4000, 8000 Hz].
//
// A value of 0 means the surface is a perfect reflector; 1 means it
// absorbs all incident energy. Real materials fall between these extremes
// and vary with frequency — harder materials absorb more at high frequencies,
// porous absorbers work better at mid-to-high frequencies.
//
// Coefficients in defaults.json are random-incidence values drawn from:
//   - Kuttruff, H. "Room Acoustics", 5th ed. Spon Press, 2009. Table 1.2.
//   - Beranek, L.L. "Acoustics". Acoustical Society of America, 1993. Appendix.
//   - Cox, T.J. & D'Antonio, P. "Acoustic Absorbers and Diffusers", 2nd ed.
//     Spon Press, 2009. Appendix A.
//   - Egan, M.D. "Architectural Acoustics". McGraw-Hill, 1988. Appendix tables.
//
// Values are approximate; real materials vary by manufacturer, mounting
// condition, and installation density. Use as a realistic starting point,
// not as authoritative acoustic data.
type Absorption [7]float64

// KnownMaterials is the built-in material library, populated at startup from
// the embedded defaults.json. Keys are lowercase material names as used in
// scene JSON files (e.g. "concrete", "acoustic_tile"). Values are Absorption
// arrays indexed by octave band.
var KnownMaterials map[string]Absorption

// init loads KnownMaterials from the embedded defaults.json at program startup.
// Panics if defaults.json is malformed — this would indicate a build-time
// error in the embedded data, not a runtime user error.
func init() {
	if err := json.Unmarshal(defaultsJSON, &KnownMaterials); err != nil {
		panic("invalid scene/defaults.json: " + err.Error())
	}
}
