# Airpath — Plugin Architecture Options

## Overview

The core engineering challenge for the Airpath bleed simulator is the multichannel input problem: the plugin needs simultaneous access to N separate source signals (one per instrument track) to compute the N×M bleed matrix and distribute the results to M microphone outputs. Standard audio plugins operate on a single track's audio. Getting multiple tracks' audio into a single processing engine, and getting differentiated results back out to individual tracks, is architecturally non-trivial.

This document evaluates five approaches to solving this problem, with the Send/Room plugin pair identified as the plan-of-record for the first commercial release.

Development will be primarily on macOS, with cross-platform compatibility (macOS and Windows) as a goal. Linux support is desirable but secondary.

---

## Option A: Wide Channel Count on a Single Bus

The plugin is a single instance with a high channel count (e.g., 16-in/16-out) inserted on a dedicated bus track. Users route source tracks into specific channel pairs on the bus using their DAW's send/receive routing: kick on channels 1–2, vocal on 3–4, guitar on 5–6, etc. The plugin sees all sources as a wide interleaved buffer and produces per-mic outputs on the corresponding output channel pairs.

### Pros

- Single plugin instance — no inter-instance coordination, no shared memory, no synchronization complexity.
- The acoustic engine has a complete, simultaneous view of all sources in every audio callback.
- Straightforward to implement: one process callback, one IR matrix, one set of convolution engines.
- Works well in Reaper (64 channels per track, flexible routing) and reasonably in Cubase/Nuendo.

### Cons

- DAW compatibility is poor. Logic Pro does not handle arbitrary high-channel-count effect plugins well. Ableton Live has minimal support. Pro Tools is restrictive.
- Requires the user to manually configure per-channel send routing from each source track to the bus, and per-channel return routing from the bus back to mic tracks. For an 8×8 configuration, this is 16 routing assignments — tedious and error-prone.
- The channel-to-source assignment is implicit (kick is on channels 1–2 because you routed it that way) rather than explicit in the plugin UI, making it fragile and hard to troubleshoot.
- Users unfamiliar with multichannel routing (the majority) will find setup daunting.

### Key Technology

- VST3 supports arbitrary channel counts per bus via `SpeakerArrangement`.
- JUCE's `BusesProperties` API configures the channel layout, but the DAW must accept it.
- CLAP (the newer plugin format) also supports flexible channel configurations, with reportedly better DAW adoption for non-standard layouts.

---

## Option B: Multiple Auxiliary Input Buses

The plugin declares one main stereo input bus plus N-1 auxiliary (sidechain) input buses in the VST3 bus layout. The DAW presents each aux bus as a routable sidechain input. Users assign source tracks to specific sidechain inputs via the DAW's sidechain routing UI.

### Pros

- "Correct" use of the VST3 bus architecture — the plugin's input topology is explicitly declared and the DAW can present it cleanly.
- Cubase/Nuendo handles multiple sidechain inputs well (Steinberg's own Frequency 2 EQ supports 8 sidechain inputs).
- Each input bus can be named, making the source-to-bus assignment more discoverable than Option A.

### Cons

- DAW support is even more uneven than Option A. Logic Pro supports only a single sidechain bus. Ableton Live does not support multiple sidechains. Pro Tools supports one key input.
- The user still has to manually route each source track to the correct sidechain input.
- Adding or removing sources requires the plugin to change its bus layout dynamically, which not all DAWs handle gracefully (some require re-instantiating the plugin).

### Key Technology

- VST3 `kAux` bus type for sidechain inputs. The `kMain` bus must be declared before any `kAux` buses.
- JUCE tutorial "Configuring the right bus layouts" covers the API for declaring and negotiating bus layouts.
- AU (Audio Unit) format has limited support for multiple sidechain inputs — typically one sidechain bus only.

---

## Option C: Multiple Linked Instances (Send/Room Plugin Pair)

**This is the plan-of-record.**

Two cooperating plugins communicate via shared memory. A lightweight "Send" plugin is inserted on each source track. A single "Room" plugin instance manages the room model and runs the acoustic engine. The Send instances capture their track's audio and transmit it to the Room engine through a shared-memory back channel. The Room engine computes the N×M bleed matrix and returns each source track's bleed contributions to its respective Send instance, which mixes the bleed into its track output.

No DAW-level routing is required beyond inserting plugins on tracks.

### Architecture

```
Track: Kick           Track: Vocal          Track: Guitar
  [Airpath Send]        [Airpath Send]        [Airpath Send]
  "I am kick,           "I am vocal,          "I am guitar,
   position (3,1)"       position (5,3)"       position (1.5,4)"
       |                     |                     |
       |   audio write       |   audio write       |   audio write
       v                     v                     v
  ┌─────────────────────────────────────────────────────┐
  │              Shared Memory Region                   │
  │  ┌─────────┐  ┌─────────┐  ┌─────────┐            │
  │  │ kick    │  │ vocal   │  │ guitar  │  audio in   │
  │  │ buffer  │  │ buffer  │  │ buffer  │            │
  │  └────┬────┘  └────┬────┘  └────┬────┘            │
  │       └────────────┼────────────┘                   │
  │                    v                                │
  │          ┌──────────────────┐                       │
  │          │  Acoustic Engine │                       │
  │          │  (N×M convolve)  │                       │
  │          └────────┬─────────┘                       │
  │       ┌───────────┼───────────┐                     │
  │  ┌────v────┐  ┌───v─────┐  ┌─v───────┐            │
  │  │ kick    │  │ vocal   │  │ guitar  │  bleed out  │
  │  │ bleed   │  │ bleed   │  │ bleed   │            │
  │  └─────────┘  └─────────┘  └─────────┘            │
  └─────────────────────────────────────────────────────┘
       |                     |                     |
       |   bleed read        |   bleed read        |   bleed read
       v                     v                     v
  [Airpath Send]        [Airpath Send]        [Airpath Send]
  mixes bleed into      mixes bleed into      mixes bleed into
  track output          track output          track output
```

A separate track (or the same instance serving dual purpose) hosts the **Airpath Room** plugin, which provides the room designer UI and coordinates the engine. The Room instance does not need to be on the audio path — it can sit on a muted track or a dedicated control track. Its role is to manage the room configuration and push IR updates to the engine.

### Send Plugin Behavior

Each Send instance performs three functions per audio block:

1. **Write** its track's audio into the shared input buffer for its assigned source position.
2. **Read** the bleed contributions destined for its assigned mic position from the shared output buffer (computed by the engine on the previous block).
3. **Mix** the bleed into the track's audio output, scaled by a per-instance wet/dry control.

The Send plugin's UI is minimal: a dropdown to select which source/mic position it represents (populated from the Room instance's configuration), a bleed level control, and a bypass toggle. Optionally, a small 2D position indicator showing where this source/mic sits in the room.

### Room Plugin Behavior

The Room plugin provides the room designer interface — initially a 2D plan view for the in-DAW plugin, with the standalone 3D designer communicating via IPC for more detailed room design work. The Room instance:

- Manages the room geometry, surface materials, source positions, mic positions, and gobo placements.
- Computes the N×M IR matrix whenever the room configuration changes.
- Loads the IRs into the convolution engines in the shared acoustic engine.
- Maintains the registry of active Send instances and their source/mic assignments.

### Synchronization

DAWs process tracks in parallel on multiple CPU cores. Send instances on different tracks may process their audio blocks in arbitrary order. The synchronization approach:

- Each Send writes its audio for the current block and reads its bleed from the previous block. This introduces exactly one block of latency (e.g., 256 samples at 48 kHz ≈ 5.3 ms).
- All Send instances report the same latency to the DAW via the standard plugin latency reporting mechanism. The DAW's Plugin Delay Compensation (PDC) system aligns the dry and processed signals automatically.
- The acoustic engine runs on a dedicated background thread, triggered when all Send instances have written their audio for the current block (tracked via an atomic counter). Alternatively, the engine runs continuously on a timer, processing whatever audio is available.
- All shared buffers use lock-free ring buffers or double-buffering to avoid blocking the audio thread. No mutexes or locks in the audio path.

### Instance Discovery and Group Management

- The Room plugin creates a named shared-memory region when it initializes. The name can include a user-configurable group identifier (e.g., "Airpath-RoomA") to support multiple independent rooms in the same session.
- Send instances search for the shared-memory region by name on initialization. If found, they register with the Room's instance registry.
- The registry tracks: instance ID, assigned source position, assigned mic position, active/bypassed state.
- When a Send instance is removed (track deleted, plugin bypassed, DAW session closed), it deregisters. The engine adapts dynamically to the changing set of active sources.
- Session save/load: each Send instance saves its source/mic assignment as plugin state. The Room instance saves the complete room configuration. On session reload, the Room initializes first (creating the shared memory), then the Sends reconnect and re-register.

### Edge Cases

- **Undo/redo:** If a user deletes a track and then undoes the deletion, the Send instance must re-register with its previous configuration intact. The plugin's saved state (source/mic assignment) ensures this works as long as the Room instance is still active.
- **Offline rendering/bouncing:** During offline bounce, the DAW may process tracks serially rather than in parallel, and at faster-than-real-time speed. The one-block-latency approach works correctly regardless of processing order or speed, since each Send always reads the previous block's results.
- **Track solo/mute:** When a source track is muted, its Send instance should still write silence (or stop writing) so the engine doesn't use stale audio. When a track is soloed, the engine should ideally still compute bleed from all sources (so the soloed track's bleed sounds correct), but this depends on whether the DAW mutes the Send plugin or the track output.
- **Different buffer sizes:** Some DAWs change buffer sizes between playback and rendering. The shared buffers must accommodate variable block sizes.

### Pros

- Works in every DAW without special routing configuration. Users insert plugins on tracks — a completely standard workflow.
- No DAW-specific routing knowledge required from the user.
- Scales naturally: adding a source means inserting another Send instance.
- Degrades gracefully: bypassing or removing a Send just removes that source from the bleed network.
- The Room plugin can have a rich UI (2D room view, parameter controls) without being constrained by multichannel bus layout.
- Compatible with all plugin formats (VST3, AU, AAX) since each instance is a standard stereo insert.

### Cons

- Shared-memory inter-instance communication adds engineering complexity: lock-free buffers, atomic synchronization, instance lifecycle management.
- One block of inherent latency (typically 3–10 ms depending on buffer size). Acceptable for a bleed simulator but visible in the DAW's latency reporting.
- CPU load is borne by whichever thread runs the acoustic engine, which is not transparently reported in the DAW's per-track CPU meters.
- Testing requires a DAW environment with multiple tracks — unit testing the coordination logic requires either a mock host or integration tests.
- The shared-memory approach is platform-specific in its implementation details (POSIX shared memory on macOS/Linux, named shared memory on Windows), though JUCE and Boost.Interprocess abstract this.
- Some DAWs sandbox plugins in separate processes (AU validation, some AAX configurations), which would break shared-memory communication. This needs to be tested per-DAW.

### Key Technology

- **JUCE SharedResourcePointer** or a static singleton for in-process instance discovery (works when all instances are in the same process, which is the common case).
- **POSIX `shm_open` / `mmap`** (macOS/Linux) or **Windows `CreateFileMapping`** for cross-process shared memory if needed.
- **Lock-free ring buffers** (e.g., JUCE `AbstractFifo`) for audio data exchange without blocking the audio thread.
- **`std::atomic`** for synchronization counters and state flags.
- **JUCE `InterprocessConnection`** or local TCP/Unix domain sockets for control-plane communication between the Room instance and the standalone 3D designer application.

---

## Option D: JACK/PipeWire Audio Server (External Application)

The room simulator runs as a standalone audio application that connects to the system's inter-application audio routing layer. On macOS, this means Core Audio inter-app routing (via a virtual audio device or aggregate device). On Linux, this means JACK or PipeWire. The DAW sends source tracks to the simulator's inputs via hardware sends, and receives the processed (bleed-added) signals back on return tracks.

### Pros

- The acoustic engine has a complete, simultaneous view of all sources in a single audio callback — no synchronization complexity.
- The standalone application has complete UI freedom: full OpenGL/Vulkan 3D room designer, no plugin UI constraints.
- CPU load is clearly attributable to a single, visible process.
- Testing is straightforward: feed WAV files in, examine WAV files out.
- On Linux, JACK/PipeWire makes this nearly zero-friction. The DAW (Ardour/Mixbus) is a native JACK client.

### Cons

- On macOS, inter-application audio routing requires either a third-party virtual audio cable (BlackHole, Loopback) or developing a custom Core Audio HAL plugin. Users must install additional software or a system-level audio driver.
- On Windows, the situation is worse — virtual audio routing requires kernel-mode drivers or third-party tools (VB-Audio Virtual Cable, ASIO Link Pro).
- Session recall is not automatic. The user must separately save/load the room configuration and re-establish audio routing when reopening a DAW session. JACK session management and PipeWire session restoration can help on Linux but are not available on macOS/Windows.
- The user must manually configure hardware sends/returns in the DAW for each source track — similar routing burden to Option A.
- Latency is additive: the DAW's output buffer, plus the simulator's processing, plus the DAW's input buffer. DAW hardware insert latency compensation can correct for this, but not all DAWs support it for virtual devices.
- The product requires the user to run two applications simultaneously (DAW + room simulator), which is a workflow complication.

### Key Technology

- **macOS:** Core Audio, aggregate audio devices, BlackHole (open-source virtual audio device), Rogue Amoeba Loopback (commercial). For a production product, a custom Core Audio HAL plugin (Audio Server Plugin API) would eliminate the third-party dependency.
- **Linux:** JACK2 or PipeWire (with JACK compatibility layer). Go bindings exist for libjack. Ardour/Mixbus are native JACK clients.
- **Windows:** ASIO, WASAPI, VB-Audio Virtual Cable, or a custom WDM audio driver.
- **Cross-platform audio framework:** RtAudio or PortAudio for the standalone application, with platform-specific virtual device integration.

---

## Option E: Virtual Audio Device (Emulated Hardware)

An evolution of Option D. Instead of relying on third-party inter-application routing, the product includes a custom virtual audio driver that presents itself to the operating system as a hardware audio device. The DAW sees "Airpath" in its audio device/hardware insert list and routes to it using standard hardware insert workflow. The room simulator application processes the audio behind the virtual device.

### How It Works

The virtual audio device registers with the operating system's audio subsystem as an audio interface with N input channels and M output channels. From the DAW's perspective, it behaves identically to a physical audio interface — audio sent to the device's outputs is processed by the room simulator and returned on the device's inputs.

```
DAW                          Virtual Device              Simulator App
                             "Airpath 8×8"
Track: Kick ──send──►  Output 1-2  ─────────►  Input 1-2 (kick)
Track: Vocal ──send──► Output 3-4  ─────────►  Input 3-4 (vocal)
Track: Guitar ─send──► Output 5-6  ─────────►  Input 5-6 (guitar)
                                                    │
                                              ┌─────┴──────┐
                                              │  Acoustic   │
                                              │  Engine     │
                                              │  (N×M)      │
                                              └─────┬──────┘
                                                    │
Track: Kick ◄──recv──   Input 1-2  ◄─────────  Output 1-2 (kick+bleed)
Track: Vocal ◄──recv──  Input 3-4  ◄─────────  Output 3-4 (vocal+bleed)
Track: Guitar ◄──recv── Input 5-6  ◄─────────  Output 5-6 (guitar+bleed)
```

The user's workflow mirrors outboard hardware: set up a hardware insert on each source track, select "Airpath" as the device, assign the output/input channel pair. This is a workflow every professional engineer already knows.

### Implementation by Platform

**macOS:** The Audio Server Plugin API (introduced in macOS 10.11) allows creating virtual audio devices as user-space code — no kernel extension required. The driver is packaged as a `.driver` bundle installed in `/Library/Audio/Plug-Ins/HAL/`. It must be code-signed and notarized. The driver creates an `AudioServerPlugInDriverInterface` that the Core Audio HAL loads. The driver communicates with the room simulator application via shared memory or Mach IPC. Example open-source implementations: BlackHole (by Existential Audio), which provides a minimal virtual audio device that can be studied as a reference.

**Windows:** A virtual audio driver is significantly more complex. The traditional approach is a WDM (Windows Driver Model) audio miniport driver, which requires kernel-mode code, a code-signing certificate (EV certificate for kernel drivers on Windows 10+), and WHQL certification for distribution without driver signature warnings. Newer alternatives include the Audio Processing Object (APO) framework and the Acoustic Echo Cancellation (AEC) API, but these are limited in scope. A pragmatic alternative is to bundle a lightweight ASIO loopback driver that connects the DAW to the simulator application.

**Linux:** Not needed — JACK/PipeWire provides the virtual routing layer natively. The simulator registers as a JACK client with N+M ports.

### Latency Characteristics

Because the virtual device operates entirely in the digital domain (no A/D or D/A conversion), the round-trip latency is determined solely by the buffer size. A buffer of 64 samples at 48 kHz is 1.3 ms — significantly lower than physical hardware round-trip. The DAW's hardware insert latency compensation measures the round-trip automatically (most DAWs send a test pulse and measure the delay).

### Session and Lifecycle Management

When the simulator application is not running but the virtual device driver is installed, the device should be visible to the DAW but should pass audio through unchanged (unity gain) or output silence. The driver must be robust against the application connecting and disconnecting — the audio stream must never glitch or produce garbage during state transitions.

The simulator application stores room configurations as project files. A session management protocol between the DAW and the simulator could be implemented via OSC (Open Sound Control), MIDI SysEx, or a local socket, allowing the DAW to send a "recall session" command to the simulator when a DAW session is loaded. However, this requires DAW-specific scripting or a companion plugin that sends the recall command on session load.

A companion plugin approach bridges the gap: a lightweight plugin in the DAW session stores the room configuration file path and sends a recall command to the simulator on session load. This is much simpler than the full Send/Room plugin pair — it's just a control messenger, not an audio processor.

### Pros

- Universal DAW compatibility via the most fundamental and well-tested audio pathway: hardware I/O.
- Familiar workflow for professional engineers accustomed to outboard hardware.
- No inter-plugin communication, no shared memory synchronization, no multi-bus negotiation.
- The simulator application has complete UI freedom and runs independently.
- Very low latency possible (digital-only round-trip).
- Clean separation of concerns: the driver handles audio transport, the application handles processing, the DAW handles routing.

### Cons

- Requires developing platform-specific audio drivers — a specialized skill set distinct from application or plugin development.
- macOS: manageable (user-space Audio Server Plugin API), but requires code-signing, notarization, and careful lifecycle management.
- Windows: significantly harder (kernel-mode WDM driver, EV code-signing certificate, WHQL certification considerations). This is the single biggest practical barrier.
- Driver installation requires elevated privileges and may require a system restart. Some users and organizations have policies against installing third-party audio drivers.
- A buggy audio driver can crash the system or cause kernel panics — the stakes for quality are higher than for application-level code.
- Session recall requires additional infrastructure (companion plugin, OSC protocol, or manual file management).
- The user needs enough I/O channels on the virtual device to cover their source/mic count. The driver must be pre-configured with a maximum channel count.
- Two applications running simultaneously (DAW + simulator), though this is common practice with hardware synths and controllers.

### Key Technology

- **macOS Audio Server Plugin API:** User-space virtual audio device framework. Reference implementation: BlackHole (open-source, MIT license). Apple documentation: Audio Server Plug-in Host (AudioServerPlugIn.h).
- **Windows WDM Audio Miniport Driver:** Kernel-mode audio driver framework. Requires Windows Driver Kit (WDK), kernel debugging tools, and EV code-signing certificate. Reference: Microsoft Virtual Audio Device Driver Sample (Sysvad).
- **Shared memory / IPC** between the driver and the simulator application: Mach IPC on macOS, named pipes or shared memory on Windows.
- **OSC (Open Sound Control)** for session recall commands between the DAW (via companion plugin) and the simulator application.

---

## Recommendation

The **Send/Room plugin pair (Option C)** is the recommended architecture for the first commercial release. It provides universal DAW compatibility without driver installation, requires no manual audio routing from the user, and keeps all development within the standard plugin SDK ecosystem (JUCE + VST3/AU/AAX).

The **virtual audio device (Option E)** is a compelling evolution for a future version, particularly for the professional market that values the outboard-gear workflow and can tolerate driver installation. On macOS, the Audio Server Plugin API makes this feasible as a user-space component. On Windows, the driver development cost is significant and may not be justified until the product has proven market demand.

The **JACK/PipeWire approach (Option D)** is valuable during development for real-time listening and validation of the acoustic engine, but is not viable as a commercial distribution strategy due to platform limitations on macOS and Windows.

For proof-of-concept development on macOS, the Go CLI tool (`airpath`) generates IR WAV files that can be loaded into any convolution reverb plugin in the DAW, validating the acoustic model with zero plugin development. This remains the immediate next step regardless of which runtime architecture is ultimately chosen.
