# airpath

A Go command-line tool that computes impulse response (IR) sets for simulated acoustic spaces. Given a room description — geometry, surface materials, source positions, microphone positions/orientations, and gobos — it generates an N×M matrix of WAV files representing the acoustic path from each source to each microphone.

The primary use case is loading these IRs into a DAW for offline convolution against dry multitrack recordings, simulating realistic live-room bleed on overdubbed tracks. It is also a proof-of-concept for the acoustic modeling that will power a VST3 plugin.

## Usage

```bash
# Generate IRs from a scene description
airpath generate --scene scene.json --output ./output/ [--order 4] [--samplerate 48000] [--tail] [--tail-onset 0.08]

# Print room analysis (RT60, modes, path counts)
airpath info --scene scene.json

# List available materials and absorption coefficients
airpath materials

# Validate a scene file without generating output
airpath validate --scene scene.json
```

Output files are named `{source_id}_to_{mic_id}.wav`, one per source-mic pair, plus a `report.txt` summary.

## Scene File Format

Scenes are described in JSON. Spatial units are meters, angles are degrees.

```json
{
  "version": 1,
  "sample_rate": 48000,
  "room": {
    "width": 8.0,
    "depth": 6.0,
    "height": 3.5,
    "surfaces": {
      "floor":   "concrete",
      "ceiling": "acoustic_tile",
      "north":   "drywall",
      "south":   "drywall",
      "east":    "brick",
      "west":    "glass_window"
    }
  },
  "sources": [
    { "id": "kick",   "x": 3.0, "y": 1.0, "z": 0.3 },
    { "id": "vocal",  "x": 5.0, "y": 3.0, "z": 1.5 }
  ],
  "mics": [
    {
      "id": "kick_close",
      "x": 3.0, "y": 1.3, "z": 0.3,
      "aim": { "azimuth": 180, "elevation": 0 },
      "pattern": "cardioid"
    },
    {
      "id": "room_L",
      "x": 4.0, "y": 5.5, "z": 2.0,
      "aim": { "azimuth": 270, "elevation": -10 },
      "pattern": "omni"
    }
  ],
  "gobos": [
    {
      "id": "drum_iso",
      "x1": 2.0, "y1": 2.0,
      "x2": 4.0, "y2": 2.0,
      "height": 2.0,
      "material": "plywood"
    }
  ]
}
```

## Materials

Built-in absorption coefficients (per octave band: 125, 250, 500, 1k, 2k, 4k, 8k Hz):

`concrete`, `brick`, `drywall`, `plaster`, `glass_window`, `wood_panel`, `plywood`, `acoustic_tile`, `acoustic_foam`, `heavy_curtain`, `carpet`, `hardwood_floor`, `linoleum`

Run `airpath materials` for coefficient values.

## How It Works

### Impulse Responses

An *impulse response* (IR) captures everything a room does to sound. Imagine firing a perfectly instantaneous click from a speaker — the IR is what a microphone would record: the direct sound arriving first, then discrete early echoes from nearby walls, then a increasingly dense cloud of reflections as sound bounces around the room, and finally a diffuse reverb tail that decays gradually to silence. Load that recording into a convolution reverb plugin and you can apply the room's acoustic character to any dry signal.

airpath builds an IR by identifying every significant acoustic path from a sound source to a microphone and summing their contributions. Each path is described by two numbers:

- **Delay** — how long the sound takes to travel that path, converted to a sample offset. Distance ÷ 343 m/s × sample rate gives the exact arrival time in samples.
- **Amplitude** — how loud the sound is when it arrives, after accounting for distance, air absorption, surface bounces, and the microphone's directional sensitivity.

The final IR buffer is assembled by stamping each path's amplitude at its arrival sample. Multiple paths can arrive at the same sample and their contributions accumulate, just as real sound waves superimpose.

### The Direct Path

The simplest path is the straight line from source to microphone with no bounces. Its amplitude follows the *inverse distance law*: pressure halves every time distance doubles, so amplitude = 1/distance (normalized to 1 at 1 metre). High-frequency energy is also absorbed by the air itself — a subtle rolloff that grows with distance and is more pronounced at 4 kHz and above.

### Reflections: The Image-Source Method

The key insight behind the image-source method is that a reflection off a flat wall sounds exactly like a second source placed behind that wall — a mirror image of the original. From the microphone's perspective, there is no difference between "sound that bounced off the east wall" and "sound that came directly from a virtual copy of the source positioned symmetrically on the other side of the east wall."

For a rectangular room this extends beautifully into three dimensions. Tiling all of space with mirror-image copies of the room, each containing a reflected copy of the source, produces an infinite 3D lattice of *image sources*. Each image source represents a unique sequence of wall bounces. Computing the straight-line distance from any image source to the microphone gives the travel distance for that reflection path — no ray tracing required.

#### The Lattice

Each image source is identified by an integer triple (p, q, r), where each coordinate counts reflections along one axis:

- `p` = reflections across the west/east walls (x-axis)
- `q` = reflections across the south/north walls (y-axis)
- `r` = reflections across the floor/ceiling (z-axis)

The triple (0, 0, 0) is the real source. The *reflection order* of an image source is |p| + |q| + |r| — the total number of wall bounces the path takes. Order 1 produces 6 image sources (one per wall face). Order 4 produces 258 image sources in total.

The position of image source (p, q, r) is computed analytically from the room dimensions and the source's position. For a room of width W with source at x-coordinate s:

```
x_image(p) = p × W + s          if p is even
x_image(p) = p × W + (W − s)    if p is odd
```

The same formula applies independently to each axis. There is no iteration, no ray marching — just arithmetic.

The number of bounces off each wall is also read directly from the indices. For index p along the x-axis, the east wall (positive side) is hit `⌈|p|/2⌉` times when p > 0 and `⌊|p|/2⌋` times when p < 0, with west and east swapped for negative p. This hit count is what drives the surface absorption calculation.

### Surface Properties

Every wall in the scene has a *material* that determines how much energy it absorbs at each bounce. The material library stores absorption coefficients at seven octave bands (125 Hz to 8 kHz). A coefficient of 0 means the surface is a perfect reflector; 1 means it absorbs everything.

For each image source, airpath multiplies the path's amplitude by a *surface absorption scalar*: the product of `(1 − α)^n` for each wall, where α is the absorption coefficient and n is the number of times that wall was hit. A single bounce off concrete (α ≈ 0.02) retains 98% of amplitude. A single bounce off acoustic foam (α ≈ 0.90) retains only 10%. A 4-bounce path through an absorbent room can lose the majority of its energy.

This is why material choice dramatically changes the character of the resulting IR. Compare two extremes:

- **Bare concrete room** — low absorption on every surface, reflections arrive with high amplitude and take many bounces to decay. The IR shows a dense, bright cluster of early reflections.
- **Heavily treated room** — high absorption, reflections lose energy fast. The IR shows only a few faint early reflections before falling to silence.

The current implementation uses a mid-band (1 kHz) absorption value as a single scalar per path. Milestone 5 will replace this with per-octave-band FIR filtering for accurate spectral shaping at each reflection.

### Microphone Polar Patterns

Real microphones are not equally sensitive in all directions. A close-mic pointed at a source rejects sounds arriving from behind; a room mic may be omnidirectional. airpath models this with the standard *cardioid family* formula:

```
gain(θ) = a + (1 − a) × cos(θ)
```

where θ is the angle between the microphone's aim direction and the direction the sound arrives from. The pattern coefficient `a` selects the shape:

| Pattern       | a    | On-axis (θ=0°) | Side (θ=90°) | Rear (θ=180°) |
|---------------|------|----------------|--------------|---------------|
| Omni          | 1.0  | 1.0            | 1.0          | 1.0           |
| Cardioid      | 0.5  | 1.0            | 0.5          | 0.0           |
| Supercardioid | 0.37 | 1.0            | 0.37         | −0.26 → 0.0  |
| Figure-8      | 0.0  | 1.0            | 0.0          | −1.0 → 0.0   |

(Negative values are clamped to zero — no phase inversion in this model.)

Omni is the simplest: `cos(θ)` cancels out and sensitivity is the same in every direction. Cardioid is the most common studio pattern — it picks up well at the front, attenuates the sides by 6 dB, and rejects the rear completely. Supercardioid has a tighter front lobe but a small rear lobe. Figure-8 captures front and rear equally while rejecting the sides — typical of ribbon microphones.

The aim direction is set in the scene file as azimuth (0° = north/+Y, 90° = east/+X, clockwise) and elevation (0° = horizontal, positive = upward). airpath converts this to a unit vector and computes cos(θ) via the dot product with the direction of each incoming path.

### Gobo Occlusion and Diffraction

A *gobo* (short for "go-between") is a portable acoustic panel placed in a room to block sound paths between a source and a microphone. A drum screen between the kit and a guitar amp is a typical example. airpath models gobos as vertical flat rectangles. Each gobo is defined by two floor-plan coordinates — `(x1, y1)` and `(x2, y2)` — that describe the panel's footprint, plus a `height` measured from the floor.

#### Intersection Test

For each acoustic path — direct or reflected — airpath checks whether the line segment from source to mic (or from image source to mic) intersects the gobo panel.

The test works by decomposing the problem into three independent checks:

1. **Plane crossing.** Compute the outward normal of the gobo's footprint line, then find the parameter `t ∈ [0, 1]` at which the path segment crosses that vertical plane. If the segment is parallel to the plane, or `t` falls outside `[0, 1]`, there is no intersection.

2. **Horizontal extent.** At the crossing point, project onto the footprint direction and compute a parameter `s ∈ [0, 1]`. If `s` is outside that range the path misses the panel left or right.

3. **Vertical extent.** Check that the crossing point's height is between 0 and the gobo's height. Paths that clear the top are not blocked.

All three conditions must be true for the path to be occluded.

#### Maekawa Diffraction

When a path is blocked, the sound is not silenced — it bends around the top edge of the gobo and arrives at the microphone with reduced amplitude. This is *diffraction*, and airpath models it with the Maekawa barrier formula, a standard empirical approximation used in architectural acoustics:

```
δ = |source → D| + |D → mic| − |source → mic|
N = 2δ / λ      (λ = 343 / 1000 = 0.343 m at 1 kHz)
attenuation_dB ≈ 10 × log10(3 + 20N)
scalar = 10^(−attenuation_dB / 20)
```

Here `D` is the *diffraction point* — the spot on the top edge of the gobo directly above where the blocked path would have crossed the panel's footprint. The quantity `δ` is the extra distance the diffracted wave must travel compared to the straight-line path that the gobo blocked. A longer detour means more attenuation.

`N` is the *Fresnel number*, a dimensionless measure of how deeply the listener sits in the acoustic shadow. At the geometric shadow boundary (path grazing the edge, `δ → 0`) the formula gives about 5 dB of attenuation — even at the edge, diffraction softens but does not eliminate the sound. At `N = 1` (a modest detour) attenuation is around 13 dB; at `N = 10` around 23 dB.

The formula is evaluated at 1 kHz as a mid-band approximation, and the resulting amplitude scalar is applied to the entire path contribution. Per-octave-band diffraction (which would produce the characteristic low-frequency bleed and high-frequency shadow of a real gobo) is planned for Milestone 5 alongside full FIR filtering.

When multiple gobos block the same path, their scalars multiply.

#### Reflected Paths and the Mirrored-Gobo Technique

Applying the intersection test to reflected paths requires care. The image-source method represents a reflection as a straight segment from an image source (outside the room) to the microphone. Testing this segment against in-room gobos only covers the *mic-side leg* of the path — from the reflection point to the microphone. It misses the *source-side leg* — from the source to the reflection point.

airpath handles 1st-order reflections with a mirrored-gobo technique. For each 1st-order image source, every gobo in the room is mirrored across the same wall that created the image source. The mirrored gobo lands in image space at the same offset outside the room. The image-source-to-mic segment then intercepts this mirrored gobo at precisely the point where the real reflected path would have intercepted the real gobo — so a single intersection test covers both legs without any changes to the reflection geometry.

For 2nd-order and higher reflections, only the mic-side leg is tested. Full source-side coverage at higher orders requires tracking the ordered sequence of walls for each image source, which is not yet implemented.

### Late Reverb Tail

The image-source method produces discrete early reflections but not the dense, diffuse reverb tail that characterises a real room. At some point after the early reflections — typically 50–100 ms in a small room — the reflection density becomes so high that individual echoes are no longer distinguishable, and the room transitions to a diffuse field: energy arriving from all directions with no coherent structure. airpath appends a synthetic tail to model this region.

#### Sabine RT60

The decay rate of the tail is derived from the room's *RT60* — the time for sound energy to fall by 60 dB after a source stops. airpath computes RT60 from the Sabine equation:

```
RT60 = 0.161 × V / A
```

where V is the room volume (m³) and A is the total *absorption area*:

```
A = Σ (surface area × α)
```

summed over all six walls. The absorption coefficient α is taken at the 1 kHz mid-band, consistent with the scalar absorption used throughout the early-reflection model. A concrete room with α ≈ 0.02 on every surface might have RT60 > 3 s; a heavily treated room (α > 0.70 on most surfaces) might have RT60 < 0.3 s.

#### Generating the Tail

The synthetic tail is built in four steps:

1. **Gaussian noise.** White noise is generated for the tail region using the Box-Muller transform. A fixed random seed ensures deterministic output across runs.

2. **Exponential decay.** Each sample is multiplied by an envelope that reaches exactly −60 dB at t = RT60:

   ```
   envelope(n) = exp(−ln(1000) × n / (RT60 × sampleRate))
   ```

3. **One-pole HF rolloff.** High frequencies decay faster than low frequencies in real rooms — air absorption and surface losses accumulate more quickly at high frequencies. A leaky integrator (~3 kHz cutoff) is applied in-place to simulate this:

   ```
   α = 1 − exp(−2π × 3000 / sampleRate)
   y[n] = α × x[n] + (1 − α) × y[n−1]
   ```

4. **Raised-cosine fade-in and normalisation.** A 20 ms fade-in prevents a click at the tail onset. The tail is then normalised so its RMS over the first 20 ms equals 1.0 — the engine applies a per-pair gain at mix time.

#### Onset and Per-Pair Amplitude Scaling

The tail begins at a configurable onset time (default 80 ms, set with `--tail-onset`). Before mixing the tail into the assembled IR, the engine measures the RMS of the IR in a ±20 ms window around the onset time. This RMS becomes the tail's mix gain, so the tail amplitude matches the late early-reflection energy of that specific source-mic pair: close sources with strong late reflections get a louder tail; lightly absorbed rooms with sparse late reflections get a quieter one. If the IR is silent at the onset (for example, with `--order 0`), the tail mixes in at zero gain — acoustically correct, since there is no energy to seed the diffuse field.

### Model Limitations

**Phase changes at surfaces.** When sound reflects off a surface, the surface's acoustic impedance can shift the phase of the reflected wave. airpath's amplitude values are real scalars — there is no phase rotation per bounce. For typical hard surfaces (concrete, brick, drywall) this is a reasonable approximation, since those materials are close to the rigid limit and reflect with little phase shift. It becomes inaccurate for soft or resonant surfaces, grazing-angle incidence, or any situation where impedance mismatch is significant. A more complete model would use complex-valued per-band amplitudes, which is a prerequisite for the per-band FIR filtering planned for Milestone 5.

**Room resonance modes.** The image-source method is mathematically equivalent to the exact wave equation solution for a rigid rectangular room when summed to infinite order — room modes emerge naturally from the constructive interference of image sources whose delays are integer multiples of a modal period. In practice, airpath uses a finite reflection order (default 4, producing at most 258 image sources) and a truncated IR duration. Room modes are a steady-state phenomenon that builds up over hundreds or thousands of bounces; a 4th-order model captures the early reflections but not the resonant buildup. The resulting IR shows frequency-domain coloration related to the room geometry, but not sharp standing-wave peaks.

There is also a hard frequency limit for geometric acoustics. Below the *Schroeder frequency* — roughly 100–200 Hz for a typical small room — modes are sparse and widely separated, and the wave nature of sound dominates. Accurate modeling in that region requires a wave-based solver (finite element method, FDTD). airpath makes no attempt to model low-frequency modal behavior and is best suited to frequencies above ~150 Hz, which covers the bulk of musical content relevant to the bleed-simulation use case.

**Pressure-zone (boundary) microphones.** A PZM mounted flush against a surface exploits the fact that the direct sound and the surface reflection arrive simultaneously, summing coherently for a +6 dB sensitivity boost across all frequencies. airpath reproduces this correctly as a natural consequence of the geometry: place a mic at `z: 0` on the floor (or `x: 0` on a wall) and the corresponding image source is at the mirror position, giving identical travel distances for the direct path and the surface reflection. Both contributions land on the same sample and their amplitudes add — no special-casing required.

What the model gets wrong is the polar pattern. A real PZM is sensitive only to the half-space facing away from its mounting surface; the wall or floor behind it acts as a baffle. airpath's `omni` pattern is a full sphere, so a floor-mounted mic will receive contributions from image sources at negative z — paths that notionally arrive through the floor, which is not physical. Those sources represent high-order, heavily attenuated reflections, so the error is usually minor in practice, but it is an error. A proper hemispherical pattern option is the missing piece for accurate PZM simulation.

## Algorithms

**Mic polar pattern:** `gain(θ) = a + (1 - a) * cos(θ)`
| Pattern       | a    |
|---------------|------|
| Omni          | 1.0  |
| Cardioid      | 0.5  |
| Supercardioid | 0.37 |
| Figure-8      | 0.0  |

**Reflections:** Image-source method up to configurable order (default: 4).

**Gobo attenuation:** Maekawa barrier approximation at 1 kHz mid-band scalar.

**Late reverb:** Sabine RT60 — `RT60 = 0.161 * V / A` — shapes a noise-based exponential decay tail.

**Air absorption:** `exp(-α(f) * d)` with α ≈ 0.001 dB/m at 1kHz, 0.01 dB/m at 4kHz, 0.03 dB/m at 8kHz.

## Building

```bash
go build ./cmd/airpath
```

Requires Go 1.21+. No CGo, no external C libraries.

## Project Structure

```
airpath/
├── cmd/airpath/         # CLI entry point
├── internal/
│   ├── scene/           # Scene structs, JSON parsing, material library
│   ├── geometry/        # 3D vector math, ray, plane
│   ├── acoustics/       # Image-source, polar patterns, absorption, diffraction, reverb tail, IR assembly
│   ├── output/          # WAV writer, text/JSON report, SVG floor plan
│   └── engine/          # Top-level orchestration
├── materials/           # Default absorption coefficient data
└── examples/            # Example scene files
```
