# Airpath Hardware Coprocessor — Design Document

## Concept

Airpath Hardware is a USB audio coprocessor that applies an N×M convolution bleed matrix to multichannel audio. The device presents itself to the host computer as a standard USB Audio Class 2.0 (UAC2) interface with N output channels (DAW sends audio to the device) and N input channels (DAW receives processed audio back). Between output and input, the onboard DSP applies the room simulation convolution engine. All room configuration and IR management is handled by a companion desktop application communicating over a separate USB control channel.

The device contains no analog audio circuitry — no A/D converters, no D/A converters, no preamps, no output amplifiers. Audio enters and leaves the device entirely over USB in the digital domain. This eliminates an entire class of hardware design complexity (precision analog layout, voltage references, audio clocking, EMI shielding of analog sections) and keeps the BOM minimal.

No custom audio drivers are required on any platform. UAC2 is natively supported on macOS, Windows 10+, and Linux.

## Signal Path

```
Host Computer                  USB Cable               Airpath Device
─────────────                  ─────────               ──────────────

DAW                         (isochronous audio)
  Track: Kick    ──────►  UAC2 Out 1-2  ──────►  DSP input buffer 1-2
  Track: Vocal   ──────►  UAC2 Out 3-4  ──────►  DSP input buffer 3-4
  Track: Guitar  ──────►  UAC2 Out 5-6  ──────►  DSP input buffer 5-6
  Track: Bass    ──────►  UAC2 Out 7-8  ──────►  DSP input buffer 7-8
                                                       │
                                                  N×M convolution
                                                  (bleed matrix)
                                                       │
  Track: Kick    ◄──────  UAC2 In 1-2   ◄──────  DSP output buffer 1-2
  Track: Vocal   ◄──────  UAC2 In 3-4   ◄──────  DSP output buffer 3-4
  Track: Guitar  ◄──────  UAC2 In 5-6   ◄──────  DSP output buffer 5-6
  Track: Bass    ◄──────  UAC2 In 7-8   ◄──────  DSP output buffer 7-8

Companion App               (bulk/CDC control)
  Room Designer  ◄────────────────────────────►  Configuration engine
  IR upload      ──────────────────────────────►  IR storage (RAM/flash)
  Status/meters  ◄──────────────────────────────  DSP telemetry
```

All audio is sample-aligned within a single USB audio frame. The DSP receives all N input channels simultaneously in each processing cycle — there is no synchronization problem, no inter-instance coordination, no shared memory management. The multichannel input problem that dominates the plugin architecture discussion does not exist here.

## USB Architecture

The device exposes two USB interfaces on a single composite USB device:

**Interface 1 — UAC2 Audio:** Isochronous endpoints for audio streaming. The host OS recognizes this as a standard audio interface. The DAW sees "Airpath" in its device list and can route to it using standard hardware insert or I/O assignment. Audio format: 48 kHz, 24-bit or 32-bit float, N stereo pairs (configurable up to 8 stereo pairs = 16 channels on USB 2.0 High-Speed).

**Interface 2 — CDC/ACM Control:** Bulk endpoints presenting as a virtual serial port on the host. The companion application connects to this serial port for device configuration, IR data transfer, and status monitoring. This is a standard USB CDC class — no custom drivers needed. The serial port appears as `/dev/tty.usbmodem*` on macOS, `COM*` on Windows, `/dev/ttyACM*` on Linux.

USB 2.0 High-Speed (480 Mbit/s) provides ample bandwidth for both interfaces simultaneously. At 48 kHz / 24-bit / 16 channels, the audio stream requires approximately 18.4 Mbit/s in each direction — well within USB 2.0's capacity.

### Clocking

In UAC2 adaptive mode, the device synchronizes its audio sample clock to the USB host's clock reference. This eliminates the need for a precision crystal oscillator on the device for audio clocking — the host's clock (derived from the DAW's audio engine) is the master. The device's internal processing runs at whatever rate the host demands.

If higher clock accuracy is required (for standalone operation or for reducing jitter), a low-cost MEMS oscillator or a small crystal can provide a local 48 kHz reference, with the UAC2 asynchronous mode allowing the device to be clock master. Asynchronous mode is preferred by audiophile-grade USB interfaces but adds complexity to the firmware.

## DSP Processing

### Core Algorithm

The DSP applies partitioned FFT convolution to each source-mic pair. For N source channels and M mic positions (where M may equal N in the common case), this is N×M simultaneous convolutions.

Each convolution takes a source channel's audio, convolves it with the impulse response representing the acoustic path from that source position to a given mic position, and contributes the result to that mic's output buffer. Each mic output is the sum of contributions from all sources:

    out_mic_j = Σ over i of (input_source_i * IR_ij)

### Compute Requirements

For an 8×8 configuration (64 convolutions) with 512-sample IRs (early reflections only, ~10.7 ms at 48 kHz):

Each convolution requires a forward FFT of the input block, a complex multiply with the pre-transformed IR, and an inverse FFT of the result. Using a 1024-point FFT (512 samples with overlap-save), each convolution is approximately 3 × 1024 × log2(1024) × 2 = ~61,000 multiply-accumulate operations per block. For 64 convolutions per 512-sample block at 48 kHz (10.7 ms per block), the total compute budget is roughly 3.9 million MAC operations per 10.7 ms, or approximately 365 MMAC/s.

For longer IRs (1 second = 48,000 samples), partitioned convolution breaks the IR into segments processed at different FFT sizes. The compute load scales roughly logarithmically with IR length due to the efficiency of larger FFT partitions. A 1-second IR at 64 convolutions would require on the order of 1–2 GMAC/s sustained.

### Memory Requirements

IR storage: 64 IRs × 48,000 samples × 4 bytes (32-bit float) = ~12 MB for 1-second IRs. For 512-sample early-reflection-only IRs, this drops to ~128 KB — easily fits in on-chip RAM.

Audio buffers: double-buffered input and output, 16 channels × 512 samples × 4 bytes × 2 buffers = ~64 KB.

FFT workspace: depends on partition strategy, but typically 2–4× the IR storage for pre-computed frequency-domain IR data and intermediate results.

Total RAM requirement: ~32 MB for the full-featured version (long IRs, 8×8), under 1 MB for the early-reflections-only version.

## Hardware Design

### Bill of Materials (Core)

| Component | Purpose | Notes |
|-----------|---------|-------|
| DSP/MCU | Audio processing + USB device | See platform analysis below |
| RAM | IR storage, audio buffers, FFT workspace | External SDRAM if on-chip RAM insufficient |
| Flash | Firmware, persistent room configuration, factory presets | 4–16 MB SPI NOR flash |
| USB-C connector | Power and data | Single connector for everything |
| Voltage regulator | 5V USB → core voltage (1.2V–3.3V) | Low-noise LDO or DC-DC |
| LED(s) | Power, status, audio activity | 1–3 LEDs |
| Enclosure | Desktop housing | Small extruded aluminum or injection-molded plastic |
| PCB | 4-layer, standard FR4 | No controlled-impedance audio traces needed (no analog) |

No audio codec, no analog components beyond power regulation. The PCB layout is predominantly digital, which simplifies design and manufacturing significantly compared to a traditional audio interface.

### Power

USB bus power provides 5V at 500 mA (USB 2.0) or negotiated higher power via USB-C PD. At 2.5W (USB 2.0 standard), most candidate DSPs operate comfortably. A low-power DSP like the STM32H7 draws under 500 mW; a more powerful processor like the XMOS XU316 draws approximately 1W. No external power supply required.

### Physical Form Factor

Target size: approximately 80mm × 50mm × 25mm — smaller than a guitar pedal, roughly the size of a USB audio dongle or a Raspberry Pi Zero in a case. The absence of audio connectors (no jacks, no XLR, no ADAT) means the only external feature is the USB-C port and the LEDs. The enclosure can be very compact.

## Companion Software

The companion desktop application provides all user-facing functionality:

- Room designer (2D plan view initially, 3D view in future versions).
- Source and mic placement, orientation, and polar pattern selection.
- Gobo and surface material configuration.
- IR computation (the acoustic engine runs on the host computer, not on the DSP).
- IR transfer to the device over the USB control channel.
- Real-time level metering and status monitoring.
- Room preset save/load.
- Firmware update capability.

### IR Computation: Host vs. Device

The acoustic engine (image-source method, surface absorption, gobo diffraction, late reverb tail synthesis) runs on the host computer in the companion application — not on the DSP. The host computes the IR matrix and uploads the resulting IR data to the device. The device's only job is to apply the pre-computed IRs via convolution. This keeps the device firmware simple and focused on real-time audio processing, while allowing the acoustic model to be as sophisticated as needed without being constrained by the DSP's compute capacity.

When the user adjusts the room layout, the companion app recomputes the affected IRs and transfers them incrementally. Moving one source changes one row of the matrix (M IRs); moving one mic changes one column (N IRs). Transfer time for an incremental update: a single 1-second IR at 48 kHz / 32-bit float is 192 KB, which transfers over USB 2.0 bulk in under 10 ms. A full 8×8 matrix of 1-second IRs (~12 MB) transfers in approximately 200 ms — fast enough for interactive room design.

### Session Recall

The device stores the current room configuration and IR data in non-volatile flash. On power-up, it loads the last-used configuration and is immediately operational. This supports use cases where the device is used without the companion app running (e.g., live sound, or simply day-to-day mixing after initial setup).

The companion app can save and load room presets as files on the host. A DAW companion plugin (extremely lightweight — just a data messenger, no audio processing) could be used to trigger session recall: when the DAW loads a session, the companion plugin sends a preset-load command to the device via the USB control channel. This provides automatic session recall without requiring the user to manually reload the room configuration.

## Latency

USB audio round-trip latency at 48 kHz with a 256-sample buffer:

- Host to device: ~5.3 ms (one buffer period)
- DSP processing: < 0.1 ms (sub-sample if pipelined)
- Device to host: ~5.3 ms (one buffer period)
- Total round-trip: ~10.7 ms

This is comparable to the round-trip latency of any USB audio interface used as a hardware insert. The DAW's hardware insert latency compensation measures the actual round-trip (typically by sending a test impulse) and corrects for it automatically. The bleed audio is time-aligned with the dry signal by the DAW's PDC system.

For comparison, real acoustic bleed in a room involves propagation delays of approximately 3 ms per meter. The USB round-trip latency is equivalent to a source-mic distance of roughly 3.5 meters — well within the range of typical studio bleed.

## Platform Analysis

### Development Platforms

These platforms are suitable for prototyping, validating the DSP workload, and iterating on the firmware architecture. Ease of development, toolchain maturity, and availability of USB audio reference implementations are the primary selection criteria.

**Teensy 4.1 (NXP i.MX RT1062)**

- Processor: ARM Cortex-M7 at 600 MHz, single-precision FPU, 1 MB RAM, 8 MB flash.
- USB: High-Speed device (480 Mbit/s) with built-in PHY.
- Audio ecosystem: Paul Stoffregen's Teensy Audio Library provides a mature DSP framework with FFT, filtering, and mixing primitives. Active community with extensive examples.
- UAC2: USB audio class implementation available via the Teensy Audio Library and community contributions. May require customization for multichannel (>2 channel) UAC2 operation.
- Development environment: Arduino IDE or PlatformIO. C/C++ with Teensy-specific extensions. Fast compile-flash-test cycle.
- Compute capacity: sufficient for a 4×4 configuration with short IRs. Likely insufficient for 8×8 with long IRs.
- Cost: ~$30 for the development board.
- Best for: initial proof-of-concept, validating the USB audio round-trip, testing companion app communication. Fastest path to a working prototype.

**STM32H7 series (e.g., STM32H743, STM32H750, STM32H723)**

- Processor: ARM Cortex-M7 at 480–550 MHz, single and double-precision FPU, up to 1 MB RAM (plus optional external SDRAM via FMC).
- USB: High-Speed device with built-in PHY (STM32H723/H733/H743).
- Audio ecosystem: ST's HAL and middleware includes a UAC2 device class implementation. CMSIS-DSP library provides optimized FFT and vector math routines for Cortex-M7.
- Development environment: STM32CubeIDE (Eclipse-based), STM32CubeMX for peripheral configuration. Mature debugging via ST-Link. C/C++.
- Compute capacity: similar to Teensy. Suitable for 4×4 with short IRs, potentially 8×8 with early-reflections-only IRs if FFT routines are well-optimized.
- Cost: ~$25–50 for a Nucleo or Discovery development board.
- Best for: more rigorous firmware development than Teensy, with better debugging tools and a clearer path to production. Good choice if the STM32H7 is also the target production processor for a smaller (4×4) product variant.

**Raspberry Pi Pico 2 / RP2350**

- Processor: Dual ARM Cortex-M33 at 150 MHz, or dual RISC-V cores.
- USB: Full-Speed only (12 Mbit/s) — insufficient bandwidth for multichannel audio. Not suitable.

**Raspberry Pi CM4 / CM5 (Linux SBC)**

- Processor: ARM Cortex-A72 (CM4) or Cortex-A76 (CM5), 1.5–2.4 GHz, 1–8 GB RAM.
- USB: USB 2.0 host and device (with appropriate carrier board). USB gadget mode allows the Pi to act as a UAC2 device using Linux kernel's USB gadget framework (g_audio module).
- Audio ecosystem: full Linux environment — FFTW, libsamplerate, ALSA, JACK. Can run the same C/C++ convolution code as the desktop application.
- Development environment: standard Linux development. Cross-compile from macOS or develop directly on the Pi. GDB remote debugging.
- Compute capacity: substantially more powerful than the MCU options. An 8×8 configuration with full-length IRs is feasible with optimized NEON SIMD code.
- UAC2 via Linux USB gadget: the Linux kernel includes a UAC2 gadget driver that presents the device as a class-compliant USB audio interface. This is well-tested and configurable for arbitrary channel counts. This eliminates the need to write any USB audio firmware — the kernel handles it.
- Cost: ~$35–75 for CM4/CM5 + carrier board.
- Best for: validating the full 8×8 configuration with long IRs. Fastest path to a feature-complete prototype. The Linux USB gadget approach provides a fully compliant UAC2 device with minimal custom code. Not ideal for production (power consumption, boot time, Linux overhead) but excellent for development.

**XMOS xcore.ai Development Kit (XK-EVK-XU316)**

- Processor: XMOS XU316, 16 hardware threads, vector processing unit, 512 KB SRAM (plus external RAM support).
- USB: High-Speed device with hardware USB PHY. XMOS provides a complete UAC2 reference design that is used in commercial products.
- Audio ecosystem: XMOS's `lib_xua` (USB Audio library) is the production-grade UAC2 implementation used by Focusrite, RODE, and other manufacturers. `lib_dsp` provides FFT, FIR, biquad, and other audio DSP primitives optimized for the xcore architecture.
- Development environment: XMOS xTIMEcomposer or the newer XTC Tools. The programming language is XC (C-like with extensions for concurrency and timing). Learning curve for the xcore programming model (hardware threads, channels, ports) is significant but well-documented.
- Compute capacity: the vector processing unit and hardware threading model are well-suited to parallel convolution workloads. An 8×8 configuration with medium-length IRs is likely feasible, though this needs benchmarking.
- Cost: ~$50–100 for the evaluation kit.
- Best for: evaluating the XMOS platform specifically if it is the target for production. Not the fastest path to a prototype (XC learning curve), but the evaluation kit includes a working UAC2 audio device out of the box.

### Production Platforms

These platforms are suitable for a manufactured, commercial product. Selection criteria shift to: unit cost, power consumption, reliability, supply chain availability, production toolchain maturity, and regulatory compliance.

**XMOS XU316 (recommended for production)**

- The industry-standard platform for USB audio devices. Used in commercial products by Focusrite, RODE, Behringer, PreSonus, and others.
- Turnkey UAC2 implementation (`lib_xua`) that has been validated across thousands of product SKUs and millions of shipped units. Compliance with USB Audio Class is well-tested and field-proven.
- Hardware-threaded architecture provides deterministic real-time guarantees that are difficult to achieve on general-purpose MCUs or Linux SBCs.
- Low power consumption (under 1W), suitable for USB bus power.
- Unit cost: approximately $5–10 in production quantities (1000+).
- Supply chain: XMOS is a fabless semiconductor company (UK-based); chips are manufactured by TSMC. Availability has been generally good, though subject to the same semiconductor supply constraints as any chip.
- Well-established production toolchain: XMOS provides reference PCB designs, BOM templates, and compliance test procedures.
- The learning curve for XC programming is a one-time investment that pays off across product revisions.

**STM32H7 (alternative for lower-cost or simpler product variant)**

- Suitable for a smaller product (4×4 with short IRs) at a lower price point.
- Unit cost: approximately $5–8 in production quantities.
- Very mature supply chain (ST Microelectronics), broad distribution.
- ST provides USB audio class examples, though they are less production-hardened than XMOS's `lib_xua`.
- Appropriate if the product line includes a budget "Airpath Mini" alongside a full-featured XMOS-based "Airpath Pro."

**Analog Devices SHARC (ADSP-SC589 or similar)**

- The premium choice for maximum DSP performance. Quad-core SHARC+ with hardware-accelerated FFT.
- Used by Universal Audio, Eventide, TC Electronic, and other high-end audio companies.
- Substantially more expensive than XMOS or STM32 (~$20–40 per chip in quantity).
- Does not include a USB device peripheral — requires a separate USB interface chip (e.g., Microchip USB3300 ULPI PHY + a USB controller, or a separate MCU handling USB with audio data passed to the SHARC over SPI/I2S).
- Higher power consumption (2–5W), may require USB-C PD negotiation for bus power or an external power supply.
- Best suited if the compute requirements exceed what XMOS can deliver — for example, a 16×16 configuration with full-length IRs and sympathetic resonance processing on-device.
- The production ecosystem is mature but the development toolchain (CrossCore Embedded Studio) is less accessible than XMOS or STM32 tools.

**FPGA (Lattice ECP5, Xilinx Artix-7)**

- Could implement the convolution matrix very efficiently in dedicated hardware (parallel FFT engines).
- Extremely high performance per watt for this specific workload.
- Development is in Verilog or VHDL — a fundamentally different skill set from C/C++ firmware development. Significantly longer development cycle.
- USB device implementation on FPGA is possible but non-trivial (requires a soft USB PHY or an external USB device controller).
- Unit cost can be competitive ($5–15) but the development NRE is high.
- Best reserved for a future high-channel-count product (16×16 or 32×32) where the compute requirements exceed what a single XMOS or SHARC can deliver, or for a cost-optimized high-volume production version.

### Platform Comparison Summary

| Platform | Dev ease | Compute (8×8 long IR) | UAC2 support | Unit cost | Power | Production ready |
|----------|----------|----------------------|--------------|-----------|-------|-----------------|
| Teensy 4.1 | Excellent | Insufficient | Partial | $30 (dev) | Very low | No |
| STM32H7 | Good | Marginal | Good | $5–8 | Very low | Yes (simpler configs) |
| RPi CM4/CM5 | Excellent | Sufficient | Excellent (gadget) | $35–75 | High (2–5W) | No (boot time, OS) |
| XMOS XU316 | Moderate | Good | Excellent | $5–10 | Low | Yes (recommended) |
| SHARC SC589 | Moderate | Excellent | External USB needed | $20–40 | Moderate | Yes (premium) |
| FPGA | Low | Excellent | External USB needed | $5–15 | Low | Yes (high NRE) |

### Recommended Development Path

**Phase 1 — Quick Prototype (Raspberry Pi CM4/CM5):**
Use a Raspberry Pi with the Linux USB gadget framework to create a UAC2 device running the convolution engine. This validates the complete signal path (DAW → USB → DSP → USB → DAW) with minimal firmware development. The convolution code runs as a normal Linux application. Development is entirely in C/C++ on a familiar Linux environment. This prototype answers the key question: does the hardware coprocessor concept work in practice, across DAWs, on macOS and Windows?

**Phase 2 — XMOS Evaluation (XK-EVK-XU316):**
Port the convolution engine to the XMOS platform using the evaluation kit. Benchmark the 8×8 workload. Validate that `lib_xua` handles the required channel count. Develop the companion app communication protocol over the CDC interface. This phase answers: does the production platform meet the performance requirements?

**Phase 3 — Custom PCB and Production:**
Design a custom PCB around the XMOS XU316 (or STM32H7 for a smaller variant), targeting the compact form factor. Prototype in small quantities (10–50 units). Conduct EMC pre-compliance testing. Submit for FCC/CE certification. Engage a contract manufacturer for production runs.

## Regulatory and Compliance

**FCC Part 15 (United States):** Required for any electronic device sold in the US. The device is an unintentional radiator (digital device). Class B limits apply for consumer/desktop use. Pre-compliance testing with a near-field probe set can catch issues early; formal testing at an accredited lab costs approximately $3,000–$5,000.

**CE Marking (European Union):** Requires compliance with the EMC Directive (EN 55032/55035) and the Low Voltage Directive (if applicable — USB bus-powered devices under 50V may be exempt). Testing costs are similar to FCC.

**USB-IF Certification:** Optional but recommended. The USB Implementers Forum offers a certification program for USB devices. Certification ensures interoperability and allows use of the USB logo. Cost is approximately $5,000–$6,000 per product.

**RoHS / REACH:** Required for sale in the EU. Ensure all components and PCB materials are RoHS-compliant. This is standard practice with major component suppliers.

## Manufacturing and Cost Estimates

### BOM Estimate (XMOS XU316 version, production quantity 1000)

| Component | Est. Unit Cost |
|-----------|---------------|
| XMOS XU316 | $8 |
| 32 MB SDRAM | $2 |
| 16 MB SPI flash | $1 |
| USB-C connector + ESD protection | $1 |
| Voltage regulators (2) | $1 |
| Passive components (caps, resistors) | $1 |
| PCB (4-layer, standard) | $3 |
| Enclosure (extruded aluminum, anodized) | $5 |
| Assembly (pick and place + reflow) | $5 |
| **Total BOM + assembly** | **~$27** |

At a retail price of $249–$299, this provides a healthy margin even accounting for packaging, shipping, distribution, returns, and warranty costs. The standard multiplier for hardware products is 3–5× BOM cost to retail price; this BOM supports a retail price as low as $80–$135 while maintaining viable margins, though positioning as a premium product at $249+ is more appropriate for the target market.

### Minimum Viable Production Run

A first production run of 100–250 units balances per-unit cost (lower quantities mean higher per-unit PCB and assembly costs) against inventory risk. At 250 units with a $35/unit all-in cost and a $249 retail price, the total investment is ~$8,750 in inventory with a potential gross revenue of ~$62,250 — a manageable risk for a bootstrapped product launch, particularly if pre-orders are collected before committing to manufacturing.

---

## General-Purpose Convolution Platform — Application Notes

The hardware coprocessor is fundamentally a general-purpose device: it takes N channels of audio in, applies an arbitrary matrix of impulse responses, and sends N channels back out. Convolution is one of the most fundamental operations in signal processing, and the N×M IR matrix is a very flexible abstraction. The device hardware is identical regardless of application — what changes is the IR set and the companion software's UI and workflow. This section explores applications beyond the core bleed simulator use case.

### Guitar Cabinet and Amp Simulation

Every guitar modeler and amp sim ultimately runs the signal through a cabinet impulse response. The current approach is either a plugin in the DAW or firmware on a dedicated modeler pedal. A USB convolution box could apply cab IRs with near-zero-latency feel (sub-millisecond DSP processing, total USB round-trip under 6 ms at low buffer sizes).

With the matrix architecture, the device could apply multiple cab IRs simultaneously to a single guitar input — a close mic, a room mic, and a ribbon mic simulation, each with different IRs, blended and output on separate channels. This is what products like the Torpedo Captor X do, but over USB with no analog stage and with an arbitrary IR library rather than a fixed set.

### Headphone Spatial Audio and Binaural Rendering

Convolution with Head-Related Transfer Functions (HRTFs) is the standard technique for creating the illusion of 3D sound positioning over headphones. An HRTF is a pair of short impulse responses (one per ear) for a given source direction. The N×M matrix becomes a tool for spatializing N mono sources into binaural stereo: each source gets convolved with a pair of HRTFs corresponding to its virtual position, and the results are summed to the left and right ear outputs.

This is exactly what Apple's Spatial Audio, Dolby Atmos for Headphones, and Dear Reality's (now defunct) plugins do — but as a hardware device, it works outside any specific software ecosystem. A mixing engineer could monitor their multichannel mix binaurally through the device without any plugin or software configuration.

The connection to the room simulator is direct: binaural rendering is the bleed matrix with M=2 (two ears) and the IRs are BRIRs (Binaural Room Impulse Responses) instead of room IRs. The same engine, different IR data.

### Live Sound Processing

In a live sound context, a FOH engineer could insert the device on a digital console's insert points to add room character to close-miked sources. Live recordings and mixes often sound dry and clinical because every source is isolated — exactly the problem the bleed simulator solves. In a live context, plugins aren't practical (no DAW at FOH), so a hardware device is the natural form factor.

Beyond bleed, the device could serve as a live reverb processor — load a high-quality hall IR and use it as a send effect from the console. Many live engineers carry dedicated hardware reverb units (Bricasti M7 at ~$3,500, TC Electronic System 6000 at ~$8,000, Lexicon PCM96 at ~$3,000) for exactly this purpose. A USB convolution box with a library of quality IRs at $249 would be dramatically cheaper.

Note: live sound use may require analog I/O (ADAT or direct analog) rather than USB if the console doesn't support USB audio. This could motivate a variant with ADAT optical I/O or a future hardware revision with analog I/O. Alternatively, the USB device connects to a laptop running the companion app, which bridges audio between the console and the device — though this adds complexity and a laptop to the signal chain.

### Speaker Correction and Room EQ

The inverse of the room simulation problem: instead of adding room character, remove it. Measure the impulse response of a listening room (speakers + room acoustics), compute the inverse filter, and apply it via convolution. This is what products like Sonarworks SoundID, IK Multimedia ARC, and Dirac Live do in software.

A hardware device doing this sits between the DAW output and the monitoring path, correcting the monitoring environment transparently without requiring any software to be running. It works with any source — DAW output, streaming music, video playback, anything routed through it.

For multichannel monitoring setups (5.1, 7.1, Atmos), each speaker channel gets its own correction IR. The N×N architecture handles this directly.

### Acoustic Measurement and Auralization

Architecture and acoustic consulting firms use convolution to audition spaces that don't exist yet. You model a proposed concert hall, compute its IR, and convolve dry recordings of orchestral instruments through it to hear what the hall would sound like before it's built. This is called auralization.

The Airpath device plus the companion room designer is already an auralization system — it just needs to be marketed to architects and acoustic consultants as well as musicians. The academic and consulting market is small but high-value. Existing software (ODEON, CATT-Acoustic, EASE) is priced in the thousands and is generally dated in UX terms.

With a firmware extension, the device could also generate test signals (swept sine, MLS noise) on its outputs and capture the response on its inputs, computing the IR of whatever acoustic or electronic system is connected. This turns it into a measurement tool — an acoustic analyzer for room measurement, speaker testing, or microphone characterization. The companion app would display the measured IR, compute RT60, frequency response, and other acoustic metrics.

### Musical Instrument Body Resonance

Convolution with an instrument body IR can transform a dry pickup signal into a resonant acoustic sound. This is how acoustic guitar modelers work (e.g., Fishman Aura) — they convolve a piezo pickup signal with an IR of the guitar body recorded with a microphone.

The device could serve as a universal instrument body modeler: plug in a piezo-equipped acoustic guitar, violin, cello, or mandolin (via an audio interface), and convolve with IRs captured from high-end instruments or specific microphone setups. Each channel could carry a different instrument with a different body IR.

### Cross-Talk Simulation for Electronic Music

Electronic music producers sometimes want the opposite of digital separation — they want their pristine digital synths to sound like they were played through amps in a room together, with all the bleed and interaction that implies. This is the core bleed simulator use case applied to a different genre and audience. The marketing angle shifts from "make your overdubs sound live" to "make your electronic tracks sound analog and physical."

### Convolution as a Creative Effect

Experimental convolution beyond realistic spaces: convolving a vocal with a piano's resonance, convolving a drum with the body of a guitar, convolving anything with anything to create hybrid timbres. A hardware device with drag-and-drop IR loading through the companion app makes this kind of experimentation tactile and immediate.

### Broadcast and Podcast Production

Broadcast studios frequently need to place a dry voice into a convincing environment — a narrator in a cathedral, a character in a small room, an announcer in a stadium. Convolution with space-specific IRs is the standard technique. A hardware device that a broadcast engineer can insert in the signal chain without touching their software setup has real utility. The podcast production market in particular is large and growing, and many producers are non-technical — a hardware box they plug in is more accessible than a plugin they need to configure.

---

## Platform Product Strategy

The hardware is identical regardless of application. What changes is the IR set and the companion software's mode. This suggests a single hardware product with multiple software-defined personalities:

| Mode | Application | IR Source | Target User |
|------|-------------|-----------|-------------|
| Airpath Studio | Bleed simulation | Computed from room model | Recording engineers, producers |
| Airpath Monitor | Speaker/room correction | Measured inverse filter | Mixing engineers, mastering |
| Airpath Space | Binaural/spatial rendering | HRTF sets | Headphone mixers, immersive audio |
| Airpath Cab | Guitar cabinet simulation | Captured cabinet IRs | Guitarists, amp modeler users |
| Airpath Measure | Acoustic measurement + IR capture | Self-generated test signals | Acoustic consultants, studio designers |
| Airpath Live | Live reverb / bleed processing | Hall IRs, room models | Live sound engineers |

Each mode is a software feature in the companion application, not a hardware variant. This dramatically reduces hardware product risk — the device isn't a bet on one application, it's a platform that can pivot toward wherever demand is strongest. Modes can be sold as the base product with optional expansion packs, or as a single all-inclusive package.

The same modes could exist as software-only products (plugins and standalone apps) sharing the same acoustic engine code. The hardware and software products reinforce each other: the hardware is the premium offering for users who want dedicated processing and the cleanest integration, while the plugins reach the broader market at a lower price point.
