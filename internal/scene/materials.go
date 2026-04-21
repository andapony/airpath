package scene

import (
	_ "embed"
	"encoding/json"
)

//go:embed defaults.json
var defaultsJSON []byte

// Bands lists the 7 octave-band center frequencies in Hz.
var Bands = [7]int{125, 250, 500, 1000, 2000, 4000, 8000}

// Absorption holds per-octave-band absorption coefficients for a material,
// one value per octave band (125, 250, 500, 1000, 2000, 4000, 8000 Hz).
//
// Coefficients in defaults.json are random-incidence values drawn from:
//   - Kuttruff, H. "Room Acoustics", 5th ed. Spon Press, 2009. Table 1.2.
//   - Beranek, L.L. "Acoustics". Acoustical Society of America, 1993. Appendix.
//   - Cox, T.J. & D'Antonio, P. "Acoustic Absorbers and Diffusers", 2nd ed.
//     Spon Press, 2009. Appendix A.
//   - Egan, M.D. "Architectural Acoustics". McGraw-Hill, 1988. Appendix tables.
//
// Values represent the fraction of incident sound energy absorbed (0 = perfect
// reflector, 1 = perfect absorber). All coefficients are approximate; real
// materials vary by manufacturer, mounting condition, and installation density.
type Absorption [7]float64

// KnownMaterials is the built-in library, loaded once at startup.
var KnownMaterials map[string]Absorption

func init() {
	if err := json.Unmarshal(defaultsJSON, &KnownMaterials); err != nil {
		panic("invalid scene/defaults.json: " + err.Error())
	}
}
